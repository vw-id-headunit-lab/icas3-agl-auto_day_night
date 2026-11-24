package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestReportCannotBeWrittenToPersistentData(t *testing.T) {
	before := os.Args
	defer func() { os.Args = before }()
	for _, path := range []string{"/data/report.json", "/tmp/../data/report.json"} {
		os.Args = []string{"daynight-patch", "status", "--report", path}
		if err := run(); err == nil || !strings.Contains(err.Error(), "under /tmp") {
			t.Fatalf("%s: %v", path, err)
		}
	}
}

func TestFindHMIRequiresUniqueExactMainClass(t *testing.T) {
	proc := t.TempDir()
	add := func(pid int, command string) {
		dir := filepath.Join(proc, strconv.Itoa(pid))
		os.Mkdir(dir, 0700)
		if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(command), 0600); err != nil {
			t.Fatal(err)
		}
	}
	add(10, "/bin/java\x00SomeOtherApp\x00-Ddescription=de.vw.mib.MIBMain\x00")
	if _, err := findHMI(proc); err == nil {
		t.Fatal("matched property text instead of main class")
	}
	add(11, "/lge/hmi/jvm/bin/java\x00de.vw.mib.MIBMain\x00")
	if pid, err := findHMI(proc); err != nil || pid != 11 {
		t.Fatalf("%d %v", pid, err)
	}
	add(12, "/bin/java\x00bench.platform.BenchPlatform\x00")
	if _, err := findHMI(proc); err == nil {
		t.Fatal("silently selected among multiple JVMs")
	}
}

func TestPayloadRestagesSamePathsAndLeavesNoJarFiles(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "tmp"), 0700)
	owner := &syscall.Stat_t{Uid: uint32(os.Geteuid()), Gid: uint32(os.Getegid())}
	first, err := stagePayload(root, 1080, "777", owner, []byte("agent"), []byte("helpers"))
	if err != nil {
		t.Fatal(err)
	}
	path := first.Agent
	if _, err := stagePayload(root, 1080, "777", owner, []byte("agent"), []byte("helpers")); err == nil {
		t.Fatal("allowed concurrent invocation")
	}
	first.close()
	if _, err := os.Stat(root + path); !os.IsNotExist(err) {
		t.Fatal("agent JAR left behind")
	}
	second, err := stagePayload(root, 1080, "777", owner, []byte("agent"), []byte("helpers"))
	if err != nil {
		t.Fatal(err)
	}
	defer second.close()
	if second.Agent != path {
		t.Fatal("same JVM/build must restage the identical path")
	}
	a, _ := newReportPath(second.Directory)
	b, _ := newReportPath(second.Directory)
	if a == b || filepath.Dir(a) != second.Directory {
		t.Fatal("reports need unique tmp paths")
	}
}
