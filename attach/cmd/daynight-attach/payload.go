package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

//go:embed payload/*
var embeddedPayload embed.FS

type stagedPayload struct {
	Agent     string
	Directory string
	lock      *os.File
	files     []string
}

func (p *stagedPayload) close() {
	// The JVM holds any JAR it still needs open. A later invocation stages identical
	// bytes at the same paths before invoking the cached agent again.
	for _, path := range p.files {
		os.Remove(path)
	}
	syscall.Flock(int(p.lock.Fd()), syscall.LOCK_UN)
	p.lock.Close()
}

func stagePayload(root string, pid int, identity string, owner *syscall.Stat_t, agent, helpers []byte) (*stagedPayload, error) {
	if len(agent) == 0 || len(helpers) == 0 {
		return nil, errors.New("build this patch with attach/build.py before running it")
	}
	digest := sha256.Sum256(append(append([]byte(nil), agent...), helpers...))
	directory := fmt.Sprintf("/tmp/icas3-daynight-%d-%s-%x", pid, identity, digest[:6])
	local := root + directory
	if err := os.Mkdir(local, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	info, err := os.Lstat(local)
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !ok || (stat.Uid != owner.Uid && stat.Uid != uint32(os.Geteuid())) {
		return nil, errors.New("temporary payload directory has unexpected type or owner")
	}
	if os.Geteuid() == 0 {
		if err = os.Chown(local, int(owner.Uid), int(owner.Gid)); err != nil {
			return nil, err
		}
	}
	if err = os.Chmod(local, 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(local, "lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if os.Geteuid() == 0 {
		if err = lock.Chown(int(owner.Uid), int(owner.Gid)); err != nil {
			lock.Close()
			return nil, err
		}
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, errors.New("another patch command is running")
	}
	payload := &stagedPayload{Agent: directory + "/daynight-agent.jar", Directory: directory, lock: lock}
	for name, content := range map[string][]byte{"daynight-agent.jar": agent, "daynight-helpers.jar": helpers} {
		path := filepath.Join(local, name)
		file, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(openErr) {
			info, statErr := os.Lstat(path)
			if statErr != nil || !info.Mode().IsRegular() {
				payload.close()
				return nil, errors.New("unexpected staged payload file")
			}
			before, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(before, content) {
				payload.close()
				return nil, errors.New("staged payload differs from this executable")
			}
		} else if openErr != nil {
			payload.close()
			return nil, openErr
		} else {
			payload.files = append(payload.files, path)
			_, err = file.Write(content)
			if err == nil && os.Geteuid() == 0 {
				err = file.Chown(int(owner.Uid), int(owner.Gid))
			}
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				payload.close()
				return nil, err
			}
			continue
		}
		payload.files = append(payload.files, path)
	}
	return payload, nil
}

func newReportPath(directory string) (string, error) {
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return directory + "/result-" + hex.EncodeToString(token[:]) + ".json", nil
}
