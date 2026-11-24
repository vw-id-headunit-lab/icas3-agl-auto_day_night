package main

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentResultMustMatchTargetAndCompletedState(t *testing.T) {
	valid := agentResult{OK: true, Action: "install", Installed: true, State: "INSTALLED", Runtime: "1080@localhost", StartTime: 12345}
	data, _ := json.Marshal(valid)
	if err := validateResult(data, 1080, "install"); err != nil {
		t.Fatal(err)
	}
	cases := []agentResult{valid, valid, valid, valid, valid, valid}
	cases[0].Runtime = "1081@localhost"
	cases[1].Action = "remove"
	cases[2].State = "RECOVERY_REQUIRED"
	cases[3].Installed = false
	cases[4].StartTime = 0
	cases[5].OK = false
	for _, result := range cases {
		data, _ := json.Marshal(result)
		if validateResult(data, 1080, "install") == nil {
			t.Errorf("accepted inconsistent result: %+v", result)
		}
	}
	if validateResult([]byte(`{"ok":true}`), 1080, "install") == nil {
		t.Fatal("accepted incomplete report")
	}
}

func TestStartTimeHandlesParenthesesAndSpaces(t *testing.T) {
	fields := make([]string, 24)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0] = "S"
	fields[19] = "1234567"
	got, err := parseStartTime("1080 (JAVA (worker) name) " + strings.Join(fields, " "))
	if err != nil || got != "1234567" {
		t.Fatalf("identity: %q %v", got, err)
	}
	for _, bad := range []string{"garbage", "1 (java) S 1", strings.Replace("1 (java) "+strings.Join(fields, " "), "1234567", "invalid", 1)} {
		if _, err := parseStartTime(bad); err == nil {
			t.Fatalf("accepted malformed stat: %s", bad)
		}
	}
}

func TestAttachWireProtocolAndFailure(t *testing.T) {
	for _, response := range []string{"0\n0\n", "0\nreturn code: 0\n", "101\ncommand rejected\n"} {
		dir, err := os.MkdirTemp("/tmp", "daynight-wire-")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "socket")
		listener, err := net.Listen("unix", path)
		if err != nil {
			os.RemoveAll(dir)
			t.Fatal(err)
		}
		expected := "1\x00load\x00instrument\x00false\x00/tmp/agent.jar=install|/tmp/result.json\x00"
		received := make(chan string, 1)
		go func() {
			client, err := listener.Accept()
			if err != nil {
				received <- err.Error()
				return
			}
			defer client.Close()
			data := make([]byte, len(expected))
			_, err = io.ReadFull(client, data)
			if err != nil {
				received <- err.Error()
				return
			}
			received <- string(data)
			io.WriteString(client, response)
		}()
		got, err := exchange(path, "load", "instrument", "false", "/tmp/agent.jar=install|/tmp/result.json")
		if actual := <-received; actual != expected {
			t.Errorf("unexpected command bytes: %q", actual)
		}
		if strings.HasPrefix(response, "0\n") && (err != nil || got != strings.TrimPrefix(response, "0\n")) {
			t.Errorf("response: %q %v", got, err)
		}
		if !strings.HasPrefix(response, "0\n") && err == nil {
			t.Error("ignored VM rejection")
		}
		listener.Close()
		os.RemoveAll(dir)
	}
}

func TestAttachArgumentsRejectNULBeforeConnecting(t *testing.T) {
	if _, err := exchange("/missing", "load", "instrument", "false", "bad\x00argument"); err == nil {
		t.Fatal("accepted NUL")
	}
	if _, err := exchange("/missing", "load"); err == nil {
		t.Fatal("accepted incomplete command")
	}
}
