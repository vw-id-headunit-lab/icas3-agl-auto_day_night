package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func findHMI(proc string) (int, error) {
	entries, err := os.ReadDir(proc)
	if err != nil {
		return 0, err
	}
	var matches []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 1 {
			continue
		}
		command, err := os.ReadFile(filepath.Join(proc, entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(string(command), "\x00")
		if len(args) == 0 || !strings.HasSuffix(args[0], "/java") {
			continue
		}
		for _, arg := range args[1:] {
			if arg == "de.vw.mib.MIBMain" || arg == "bench.platform.BenchPlatform" {
				matches = append(matches, pid)
				break
			}
		}
	}
	if len(matches) == 0 {
		return 0, errors.New("no running HMI JVM found")
	}
	if len(matches) != 1 {
		return 0, fmt.Errorf("multiple HMI JVMs found %v; select one with --pid", matches)
	}
	return matches[0], nil
}

func startTime(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	return parseStartTime(string(data))
}

func parseStartTime(data string) (string, error) {
	end := strings.LastIndexByte(data, ')')
	if end < 0 {
		return "", errors.New("malformed process stat")
	}
	fields := strings.Fields(data[end+1:])
	if len(fields) < 20 {
		return "", errors.New("truncated process stat")
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return "", errors.New("invalid process start time")
	}
	return fields[19], nil
}

func attachSocket(pid int) (string, func(), error) {
	proc := fmt.Sprintf("/proc/%d", pid)
	info, err := os.Stat(proc)
	if err != nil {
		return "", nil, err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || (os.Geteuid() != 0 && uint32(os.Geteuid()) != owner.Uid) {
		return "", nil, errors.New("run as the target JVM owner or root")
	}
	socket := filepath.Join(proc, "root/tmp", fmt.Sprintf(".java_pid%d", pid))
	validate := func() bool {
		file, e := os.Lstat(socket)
		if e != nil || file.Mode()&os.ModeSocket == 0 {
			return false
		}
		stat, ok := file.Sys().(*syscall.Stat_t)
		return ok && stat.Uid == owner.Uid
	}
	if validate() {
		return socket, func() {}, nil
	}
	marker := filepath.Join(proc, "cwd", fmt.Sprintf(".attach_pid%d", pid))
	f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		marker = filepath.Join(proc, "root/tmp", fmt.Sprintf(".attach_pid%d", pid))
		f, err = os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	}
	if err != nil {
		return "", nil, fmt.Errorf("could not create attach marker: %w", err)
	}
	if os.Geteuid() == 0 {
		err = f.Chown(int(owner.Uid), int(owner.Gid))
	}
	f.Close()
	cleanup := func() { os.Remove(marker) }
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if err = syscall.Kill(pid, syscall.SIGQUIT); err != nil {
		cleanup()
		return "", nil, err
	}
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		if validate() {
			return socket, cleanup, nil
		}
	}
	cleanup()
	return "", nil, errors.New("Attach listener did not appear; no patch loaded")
}
