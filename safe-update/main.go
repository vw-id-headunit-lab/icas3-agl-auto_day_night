package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const target = "/vw/hmi/Resources/skin1/viewhandler.jar"
const baseline = "9113535ba604511c41229bf00201ab4840126966f2fa8f09668f25a564d42c85"

func hash(p string) (string, error) {
	f, e := os.Open(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}
func copyData(src, dst string) error {
	f, e := os.Open(src)
	if e != nil {
		return e
	}
	defer f.Close()
	o, e := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(o, f)
	if e == nil {
		e = o.Sync()
	}
	c := o.Close()
	if e != nil {
		return e
	}
	return c
}
func label(p string) (string, error) {
	b := make([]byte, 256)
	n, e := syscall.Getxattr(p, "security.SMACK64", b)
	if e != nil {
		return "", e
	}
	return string(b[:n]), nil
}
func users(p, proc string) ([]string, error) {
	want, e := os.Stat(p)
	if e != nil {
		return nil, e
	}
	dirs, e := os.ReadDir(proc)
	if e != nil {
		return nil, e
	}
	found := []string{}
	for _, d := range dirs {
		if _, e := strconv.Atoi(d.Name()); e != nil {
			continue
		}
		pid := filepath.Join(proc, d.Name())
		cmd, e := os.ReadFile(filepath.Join(pid, "cmdline"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		busy := strings.Contains(string(cmd), "/lge/hmi/jvm/bin/java")
		fds, e := os.ReadDir(filepath.Join(pid, "fd"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		for _, fd := range fds {
			s, e := os.Stat(filepath.Join(pid, "fd", fd.Name()))
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return nil, e
			}
			if os.SameFile(want, s) {
				busy = true
			}
		}
		maps, e := os.ReadFile(filepath.Join(pid, "maps"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if strings.Contains(string(maps), p) {
			busy = true
		}
		if busy {
			found = append(found, d.Name())
		}
	}
	return found, nil
}
func gate() error {
	out, e := exec.Command("/bin/systemctl", "show", "vw-hmi.service", "-p", "ActiveState").Output()
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(out)) != "ActiveState=inactive" {
		return errors.New("HMI not inactive; online write refused")
	}
	link, e := os.Readlink("/run/systemd/system/vw-hmi.service")
	if e != nil || link != "/dev/null" {
		return errors.New("runtime mask absent; write refused")
	}
	pids, e := users(target, "/proc")
	if e != nil {
		return fmt.Errorf("cannot inspect readers; refused: %w", e)
	}
	if len(pids) > 0 {
		return fmt.Errorf("JVM or JAR readers remain: %v", pids)
	}
	return nil
}

type Manifest struct {
	Before    string `json:"before"`
	Candidate string `json:"candidate"`
}

func check(p, expected string) error {
	h, e := hash(p)
	if e != nil {
		return e
	}
	if h != expected {
		return fmt.Errorf("checksum mismatch: %s", p)
	}
	return nil
}
func prepare(src, work string) error {
	if e := check(target, baseline); e != nil {
		return e
	}
	if e := os.Mkdir(work, 0700); e != nil {
		return e
	}
	if e := copyData(src, filepath.Join(work, "candidate.jar")); e != nil {
		return e
	}
	if e := copyData(target, filepath.Join(work, "before.jar")); e != nil {
		return e
	}
	if e := check(filepath.Join(work, "before.jar"), baseline); e != nil {
		return e
	}
	h, e := hash(src)
	if e != nil {
		return e
	}
	if e = check(filepath.Join(work, "candidate.jar"), h); e != nil {
		return e
	}
	b, _ := json.MarshalIndent(Manifest{baseline, h}, "", "  ")
	return os.WriteFile(filepath.Join(work, "manifest.json"), b, 0600)
}
func apply(work string, restore bool) error {
	lock, e := os.OpenFile(filepath.Join(work, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return e
	}
	b, e := os.ReadFile(filepath.Join(work, "manifest.json"))
	if e != nil {
		return e
	}
	var m Manifest
	if e = json.Unmarshal(b, &m); e != nil {
		return e
	}
	src, undo, expected, current := "candidate.jar", "before.jar", m.Candidate, m.Before
	if restore {
		src, undo, expected, current = undo, src, current, expected
	}
	src = filepath.Join(work, src)
	undo = filepath.Join(work, undo)
	if e = check(src, expected); e != nil {
		return e
	}
	if e = check(target, current); e != nil {
		return e
	}
	if e = gate(); e != nil {
		return e
	}
	l, e := label(target)
	if e != nil || l != "hmi::sub" {
		return errors.New("unexpected access label")
	}
	old, e := os.Stat(target)
	if e != nil {
		return e
	}
	if e = gate(); e != nil {
		return e
	}
	verify := func() error {
		if e := check(target, expected); e != nil {
			return e
		}
		now, e := os.Stat(target)
		if e != nil {
			return e
		}
		a, c := old.Sys().(*syscall.Stat_t), now.Sys().(*syscall.Stat_t)
		if !os.SameFile(old, now) || old.Mode() != now.Mode() || a.Uid != c.Uid || a.Gid != c.Gid {
			return errors.New("metadata changed")
		}
		l, e := label(target)
		if e != nil || l != "hmi::sub" {
			return errors.New("label changed")
		}
		return nil
	}
	e = copyData(src, target)
	if e == nil {
		e = verify()
	}
	if e != nil {
		if check(undo, current) == nil {
			if r := copyData(undo, target); r != nil {
				return fmt.Errorf("write failed %v; recovery failed %v", e, r)
			}
		}
		return e
	}
	return nil
}
func main() {
	var e error
	if len(os.Args) < 3 {
		e = errors.New("usage: update prepare WORK CANDIDATE | apply WORK | restore WORK | inspect TARGET")
	} else {
		switch os.Args[1] {
		case "prepare":
			if len(os.Args) != 4 {
				e = errors.New("candidate required")
			} else {
				e = prepare(os.Args[3], os.Args[2])
			}
		case "apply":
			e = apply(os.Args[2], false)
		case "restore":
			e = apply(os.Args[2], true)
		case "inspect":
			var p []string
			p, e = users(os.Args[2], "/proc")
			fmt.Println("Readers:", p)
		default:
			e = errors.New("unknown action")
		}
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "REFUSED:", e)
		os.Exit(1)
	}
	fmt.Println("Verified. No service start/stop or reboot performed.")
}
