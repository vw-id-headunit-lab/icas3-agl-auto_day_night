package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type agentResult struct {
	OK        bool   `json:"ok"`
	Action    string `json:"action"`
	Installed bool   `json:"installed"`
	State     string `json:"state"`
	Runtime   string `json:"runtime"`
	StartTime int64  `json:"start_time_ms"`
}

func validateResult(data []byte, pid int, action string) error {
	var result agentResult
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if !result.OK {
		return errors.New("agent reported failure; see the JSON output")
	}
	if result.Action != action || strings.SplitN(result.Runtime, "@", 2)[0] != strconv.Itoa(pid) || result.StartTime <= 0 {
		return errors.New("agent result does not match the requested JVM/action")
	}
	if result.State == "RECOVERY_REQUIRED" || result.State == "INSTALLING" || result.State == "RESTORING" {
		return errors.New("agent has not completed the requested transition")
	}
	if (result.State != "ORIGINAL" && result.State != "INSTALLED") || result.Installed != (result.State == "INSTALLED") {
		return errors.New("agent result contains inconsistent state")
	}
	if (action == "install" && !result.Installed) || (action == "remove" && result.Installed) {
		return errors.New("agent did not reach the requested state")
	}
	return nil
}

func run() error {
	options := flag.NewFlagSet("daynight-patch", flag.ContinueOnError)
	pid := options.Int("pid", 0, "HMI JVM PID (normally detected automatically)")
	agent := options.String("agent", "", "external agent JAR (developer override)")
	action := options.String("action", "install", "install, remove, or status")
	report := options.String("report", "", "optional new report under /tmp; automatic reports are removed after reading")
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		*action = args[0]
		args = args[1:]
	}
	if err := options.Parse(args); err != nil {
		return err
	}
	if options.NArg() != 0 {
		return errors.New("unexpected positional argument")
	}
	if *action != "install" && *action != "remove" && *action != "status" {
		return errors.New("use daynight-patch [install|remove|status]")
	}
	if *report != "" && (!strings.HasPrefix(*report, "/tmp/") || filepath.Clean(*report) != *report) {
		return errors.New("report must be a new file under /tmp")
	}
	if (*agent != "" && !filepath.IsAbs(*agent)) || strings.ContainsAny(*agent+*report, "\x00\n\r|=") {
		return errors.New("invalid agent or report path")
	}
	if *pid == 0 {
		detected, err := findHMI("/proc")
		if err != nil {
			return err
		}
		*pid = detected
	}
	if *pid <= 1 {
		return errors.New("invalid JVM PID")
	}

	identity, err := startTime(*pid)
	if err != nil {
		return err
	}
	cmd, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", *pid))
	if err != nil || !strings.HasSuffix(strings.Split(string(cmd), "\x00")[0], "/java") {
		return errors.New("selected process is not a JVM")
	}
	root := fmt.Sprintf("/proc/%d/root", *pid)
	reportDirectory := "/tmp"
	var payload *stagedPayload
	if *agent == "" {
		agentBytes, readErr := embeddedPayload.ReadFile("payload/daynight-agent.jar")
		if readErr != nil {
			return errors.New("patch payload is missing; build with attach/build.py")
		}
		helpers, readErr := embeddedPayload.ReadFile("payload/daynight-helpers.jar")
		if readErr != nil {
			return readErr
		}
		process, statErr := os.Stat(fmt.Sprintf("/proc/%d", *pid))
		if statErr != nil {
			return statErr
		}
		owner, ok := process.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("cannot determine JVM owner")
		}
		payload, err = stagePayload(root, *pid, identity, owner, agentBytes, helpers)
		if err != nil {
			return err
		}
		*agent = payload.Agent
		reportDirectory = payload.Directory
		// Retain payload files if a timed-out request may still be running.
		defer func() {
			if payload != nil {
				syscall.Flock(int(payload.lock.Fd()), syscall.LOCK_UN)
				payload.lock.Close()
			}
		}()
	}
	if _, err = os.Stat(root + *agent); err != nil {
		return err
	}
	automaticReport := *report == ""
	if automaticReport {
		*report, err = newReportPath(reportDirectory)
		if err != nil {
			return err
		}
	}
	if _, err = os.Lstat(root + *report); !os.IsNotExist(err) {
		return errors.New("report path already exists or cannot be checked")
	}

	socket, cleanup, err := attachSocket(*pid)
	if err != nil {
		return err
	}
	defer cleanup()
	if after, err := startTime(*pid); err != nil || after != identity {
		return errors.New("target PID identity changed")
	}
	response, err := exchange(socket, "load", "instrument", "false", *agent+"="+*action+"|"+*report)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(root + *report)
	if err != nil {
		return fmt.Errorf("agent did not produce its result (load response %q): %w", response, err)
	}
	if automaticReport {
		defer os.Remove(root + *report)
	}
	if payload != nil {
		payload.close()
		payload = nil
	}
	fmt.Print(string(data))
	if err = validateResult(data, *pid, *action); err != nil {
		return err
	}
	if strings.TrimSpace(response) != "0" && strings.TrimSpace(response) != "return code: 0" {
		return fmt.Errorf("agent load failed: %s", response)
	}
	if after, err := startTime(*pid); err != nil || after != identity {
		return errors.New("JVM identity changed during operation")
	}
	fmt.Printf("Verified same JVM pid=%s start_ticks=%s; no HMI restart.\n", strconv.Itoa(*pid), identity)
	return nil
}
