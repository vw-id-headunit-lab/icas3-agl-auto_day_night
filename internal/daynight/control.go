package daynight

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Reply struct {
	OK             bool   `json:"ok"`
	Code           string `json:"code"`
	ConfiguredMode *int   `json:"configured_mode"`
	Day            *bool  `json:"day"`
	Changed        bool   `json:"changed"`
}
type Backend interface {
	Status(context.Context) (Reply, error)
	Set(context.Context, int, int) (Reply, error)
}
type SocketBridge struct{ Address, TokenFile string }

func (b SocketBridge) call(ctx context.Context, command string) (Reply, error) {
	var r Reply
	info, err := os.Lstat(b.TokenFile)
	if err != nil {
		return r, errors.New("bridge token unavailable")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return r, errors.New("bridge token must be a regular owner-only file")
	}
	token, err := os.ReadFile(b.TokenFile)
	if err != nil {
		return r, errors.New("cannot read bridge token")
	}
	t := strings.TrimSpace(string(token))
	if len(t) != 64 {
		return r, errors.New("bridge token must contain 64 hex characters")
	}
	if _, err = hex.DecodeString(t); err != nil {
		return r, errors.New("invalid bridge token")
	}
	conn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", b.Address)
	if err != nil {
		return r, errors.New("bridge connection unavailable")
	}
	defer conn.Close()
	deadline := time.Now().Add(9 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err = conn.SetDeadline(deadline); err != nil {
		return r, err
	}
	// Token never appears in output or errors. No HTTP, proxy or redirects involved.
	if _, err = fmt.Fprintf(conn, "%s %s\n", t, command); err != nil {
		return r, errors.New("bridge write failed; outcome may be unknown")
	}
	line, err := bufio.NewReaderSize(conn, 4096).ReadSlice('\n')
	if err != nil {
		return r, errors.New("bridge response unavailable; outcome may be unknown")
	}
	if err = DecodeStrict(line, &r); err != nil {
		return r, errors.New("invalid bridge response; outcome may be unknown")
	}
	if r.OK && (r.ConfiguredMode == nil || *r.ConfiguredMode < 0 || *r.ConfiguredMode > 2) {
		return r, errors.New("bridge returned invalid configured mode")
	}
	if !r.OK {
		return r, fmt.Errorf("bridge: %s", r.Code)
	}
	return r, nil
}
func (b SocketBridge) Status(ctx context.Context) (Reply, error) { return b.call(ctx, "STATUS") }
func (b SocketBridge) Set(ctx context.Context, expected, desired int) (Reply, error) {
	return b.call(ctx, fmt.Sprintf("SET %d %d", expected, desired))
}

type State struct {
	Version         int       `json:"version"`
	OriginalMode    *int      `json:"original_mode,omitempty"`
	LastManagedMode *int      `json:"last_managed_mode,omitempty"`
	PolicyKey       string    `json:"policy_key,omitempty"`
	HoldUntil       time.Time `json:"hold_until,omitempty"`
	ManualLatched   bool      `json:"manual_latched,omitempty"`
	PendingMode     *int      `json:"pending_mode,omitempty"`
}

func LoadState(path string) (State, error) {
	s := State{Version: 1}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err = DecodeStrict(b, &s); err != nil {
		return s, err
	}
	if s.Version != 1 {
		return s, errors.New("unknown state version")
	}
	for _, m := range []*int{s.OriginalMode, s.LastManagedMode, s.PendingMode} {
		if m != nil && (*m < 0 || *m > 2) {
			return s, errors.New("invalid saved mode")
		}
	}
	return s, nil
}
func AtomicJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".daynight-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func SaveState(path string, s State) error {
	b, _ := json.MarshalIndent(s, "", "  ")
	old, err := os.ReadFile(path)
	if err == nil && strings.TrimSpace(string(old)) == string(b) {
		return nil
	}
	return AtomicJSON(path, s)
}
func Lock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("another controller is using this state file")
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
func policyKey(c Config) string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type Result struct {
	Decision Decision `json:"decision"`
	Status   *Reply   `json:"status,omitempty"`
	Action   string   `json:"action"`
	Detail   string   `json:"detail,omitempty"`
}

// Step is shared by native-auto and custom policies; there is exactly one writer.
// The persisted getter is used for configured_mode. HMI's published Mode attribute
// can report effective 0/1 even when the persisted native configuration is AUTO=2.
func Step(ctx context.Context, c Config, now time.Time, b Backend, write, force bool) (Result, error) {
	d, err := Decide(c, now)
	r := Result{Decision: d}
	if err != nil {
		return r, err
	}
	if d.DesiredMode == nil {
		r.Action = "off"
		return r, nil
	}
	s, err := LoadState(c.StateFile)
	if err != nil {
		return r, err
	}
	current, err := b.Status(ctx)
	if err != nil {
		return r, err
	}
	r.Status = &current
	if current.ConfiguredMode == nil {
		return r, errors.New("status missing configured mode")
	}
	actual := *current.ConfiguredMode
	key := policyKey(c)
	if force {
		s.ManualLatched = false
		s.HoldUntil = time.Time{}
		s.LastManagedMode = nil
	}
	// An uncertain request is reconciled before a new policy can issue any write.
	if s.PendingMode != nil {
		if actual == *s.PendingMode {
			s.LastManagedMode = modePtr(actual)
			s.PendingMode = nil
			if write {
				if err = SaveState(c.StateFile, s); err != nil {
					return r, err
				}
			}
		} else if !force {
			r.Action = "paused"
			r.Detail = "unconfirmed previous write; inspect status, then explicitly apply --force"
			return r, nil
		} else {
			s.PendingMode = nil
		}
	}
	if s.PolicyKey != key {
		s.PolicyKey = key
		s.ManualLatched = false
		s.HoldUntil = time.Time{}
		s.LastManagedMode = nil
	}
	if c.RespectManual && !force && s.LastManagedMode != nil && actual != *s.LastManagedMode {
		s.LastManagedMode = nil
		s.ManualLatched = d.NextBoundary.IsZero()
		s.HoldUntil = d.NextBoundary
		if write {
			if err = SaveState(c.StateFile, s); err != nil {
				return r, err
			}
		}
	}
	if c.RespectManual && (s.ManualLatched || now.Before(s.HoldUntil)) {
		r.Action = "manual_override"
		r.Detail = "paused until next boundary; native-auto stays paused until reselected with apply --force"
		return r, nil
	}
	s.HoldUntil = time.Time{}
	s.ManualLatched = false
	if actual == *d.DesiredMode {
		r.Action = "unchanged"
		s.LastManagedMode = modePtr(actual)
		if write {
			err = SaveState(c.StateFile, s)
		}
		return r, err
	}
	if !write {
		r.Action = "would_set"
		return r, nil
	}
	if s.OriginalMode == nil {
		s.OriginalMode = modePtr(actual)
	}
	// Persist intent before sending. A timeout/crash cannot trigger blind retries.
	s.PendingMode = modePtr(*d.DesiredMode)
	if err = SaveState(c.StateFile, s); err != nil {
		return r, err
	}
	after, setErr := b.Set(ctx, actual, *d.DesiredMode)
	if setErr != nil {
		if noMutation(after.Code) {
			s.PendingMode = nil
			if after.Code == "conflict" && c.RespectManual {
				s.LastManagedMode = nil
				s.ManualLatched = d.NextBoundary.IsZero()
				s.HoldUntil = d.NextBoundary
			}
			if err = SaveState(c.StateFile, s); err != nil {
				return r, err
			}
		}
		return r, setErr
	}
	if after.ConfiguredMode == nil || *after.ConfiguredMode != *d.DesiredMode {
		return r, errors.New("set did not verify persisted mode; paused pending reconciliation")
	}
	s.PendingMode = nil
	s.LastManagedMode = modePtr(*d.DesiredMode)
	if err = SaveState(c.StateFile, s); err != nil {
		return r, err
	}
	r.Status = &after
	r.Action = "set"
	return r, nil
}
func noMutation(code string) bool {
	switch code {
	case "read_only", "unauthorized", "invalid_command", "not_ready", "busy", "conflict", "expired":
		return true
	}
	return false
}

func Restore(ctx context.Context, c Config, b Backend, write, force bool) (Result, error) {
	r := Result{}
	s, err := LoadState(c.StateFile)
	if err != nil {
		return r, err
	}
	if s.OriginalMode == nil {
		return r, errors.New("no saved original mode")
	}
	cur, err := b.Status(ctx)
	if err != nil {
		return r, err
	}
	r.Status = &cur
	if cur.ConfiguredMode == nil {
		return r, errors.New("missing configured mode")
	}
	if s.PendingMode != nil {
		if *cur.ConfiguredMode != *s.PendingMode && !force {
			return r, errors.New("pending write is unconfirmed; inspect before restore --force")
		}
		s.LastManagedMode = modePtr(*cur.ConfiguredMode)
		s.PendingMode = nil
	}
	r.Decision = Decision{Policy: "restore", DesiredMode: s.OriginalMode, Reason: "restore saved original native setting"}
	if *cur.ConfiguredMode == *s.OriginalMode {
		r.Action = "unchanged"
		if write {
			return r, os.Remove(c.StateFile)
		}
		return r, nil
	}
	if !force && (s.LastManagedMode == nil || *cur.ConfiguredMode != *s.LastManagedMode) {
		return r, errors.New("setting changed outside controller; inspect before restore --force")
	}
	if !write {
		r.Action = "would_restore"
		return r, nil
	}
	s.PendingMode = modePtr(*s.OriginalMode)
	if err = SaveState(c.StateFile, s); err != nil {
		return r, err
	}
	after, err := b.Set(ctx, *cur.ConfiguredMode, *s.OriginalMode)
	if err != nil {
		if noMutation(after.Code) {
			s.PendingMode = nil
			_ = SaveState(c.StateFile, s)
		}
		return r, err
	}
	if after.ConfiguredMode == nil || *after.ConfiguredMode != *s.OriginalMode {
		return r, errors.New("restore not confirmed")
	}
	r.Status = &after
	r.Action = "restored"
	return r, os.Remove(c.StateFile)
}
