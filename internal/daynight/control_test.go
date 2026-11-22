package daynight

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type fakeBackend struct {
	mode, writes int
	day          bool
	failCode     string
	unknown      bool
}

func (f *fakeBackend) Status(context.Context) (Reply, error) {
	return Reply{OK: true, ConfiguredMode: modePtr(f.mode), Day: &f.day}, nil
}
func (f *fakeBackend) Set(ctx context.Context, expected, desired int) (Reply, error) {
	if f.mode != expected {
		return Reply{Code: "conflict"}, errors.New("conflict")
	}
	if f.failCode != "" {
		return Reply{Code: f.failCode}, errors.New(f.failCode)
	}
	f.writes++
	if f.unknown {
		return Reply{}, errors.New("socket timeout")
	}
	f.mode = desired
	r, _ := f.Status(ctx)
	r.Changed = true
	return r, nil
}
func testConfig(t *testing.T) Config {
	c := DefaultConfig()
	c.Mode = "fixed"
	c.StateFile = filepath.Join(t.TempDir(), "state.json")
	return c
}
func TestNativeConfigurationIsNotEffectiveTheme(t *testing.T) {
	c := testConfig(t)
	c.Mode = "native-auto"
	b := &fakeBackend{mode: 2, day: false}
	for i := 0; i < 3; i++ {
		r, e := Step(context.Background(), c, instant("2026-11-21T10:00:00+08:00"), b, true, false)
		if e != nil || r.Action != "unchanged" {
			t.Fatal(r, e)
		}
	}
	if b.writes != 0 {
		t.Fatal("AUTO rewritten based on effective night theme")
	}
}
func TestModesShareOneBackupAndNoRepeatedWrite(t *testing.T) {
	c := testConfig(t)
	b := &fakeBackend{mode: 1}
	ctx := context.Background()
	now := instant("2026-11-21T10:00:00+08:00")
	for i := 0; i < 3; i++ {
		if _, e := Step(ctx, c, now, b, true, false); e != nil {
			t.Fatal(e)
		}
	}
	if b.mode != 0 || b.writes != 1 {
		t.Fatal(b)
	}
	c.Mode = "native-auto"
	if _, e := Step(ctx, c, now, b, true, false); e != nil {
		t.Fatal(e)
	}
	s, e := LoadState(c.StateFile)
	if e != nil || *s.OriginalMode != 1 || b.mode != 2 {
		t.Fatal(s, b, e)
	}
	r, e := Restore(ctx, c, b, true, false)
	if e != nil || r.Action != "restored" || b.mode != 1 {
		t.Fatal(r, e)
	}
}
func TestManualOverrideAndBoundary(t *testing.T) {
	c := testConfig(t)
	b := &fakeBackend{mode: 1}
	ctx := context.Background()
	at := instant("2026-11-21T10:00:00+08:00")
	if _, e := Step(ctx, c, at, b, true, false); e != nil {
		t.Fatal(e)
	}
	b.mode = 1
	r, e := Step(ctx, c, at.Add(60e9), b, true, false)
	if e != nil || r.Action != "manual_override" {
		t.Fatal(r, e)
	}
	r, e = Step(ctx, c, instant("2026-11-21T18:30:00+08:00"), b, true, false)
	if e != nil || r.Action != "unchanged" {
		t.Fatal(r, e)
	}
	if b.writes != 1 {
		t.Fatal("unnecessary rewrite")
	}
	c.Mode = "native-auto"
	if _, e = Step(ctx, c, at, b, true, false); e != nil {
		t.Fatal(e)
	}
	b.mode = 0
	r, e = Step(ctx, c, at, b, true, false)
	if e != nil || r.Action != "manual_override" {
		t.Fatal(r, e)
	}
	r, e = Step(ctx, c, at.AddDate(0, 0, 2), b, true, false)
	if e != nil || r.Action != "manual_override" {
		t.Fatal(r, e)
	}
	r, e = Step(ctx, c, at, b, true, true)
	if e != nil || b.mode != 2 {
		t.Fatal(r, e)
	}
}
func TestUncertainWriteDoesNotBlindlyRetry(t *testing.T) {
	c := testConfig(t)
	b := &fakeBackend{mode: 1, unknown: true}
	ctx := context.Background()
	at := instant("2026-11-21T10:00:00+08:00")
	if _, e := Step(ctx, c, at, b, true, false); e == nil {
		t.Fatal("expected timeout")
	}
	r, e := Step(ctx, c, at, b, true, false)
	if e != nil || r.Action != "paused" || b.writes != 1 {
		t.Fatal(r, e, b)
	}
	// Simulate late completion. The next observation reconciles it without a write.
	b.mode = 0
	b.unknown = false
	r, e = Step(ctx, c, at, b, true, false)
	if e != nil || r.Action != "unchanged" || b.writes != 1 {
		t.Fatal(r, e, b)
	}
}
func TestReadOnlyAndRestoreConflict(t *testing.T) {
	c := testConfig(t)
	b := &fakeBackend{mode: 1}
	ctx := context.Background()
	at := instant("2026-11-21T10:00:00+08:00")
	r, e := Step(ctx, c, at, b, false, false)
	if e != nil || r.Action != "would_set" || b.writes != 0 {
		t.Fatal(r, e)
	}
	s, _ := LoadState(c.StateFile)
	if s.OriginalMode != nil {
		t.Fatal("dry run saved backup")
	}
	b.failCode = "read_only"
	if _, e = Step(ctx, c, at, b, true, false); e == nil {
		t.Fatal("write gate ignored")
	}
	s, e = LoadState(c.StateFile)
	if e != nil || s.PendingMode != nil {
		t.Fatal(s, e)
	}
	b.failCode = ""
	if _, e = Step(ctx, c, at, b, true, false); e != nil {
		t.Fatal(e)
	}
	b.mode = 2
	if _, e = Restore(ctx, c, b, true, false); e == nil {
		t.Fatal("clobbered external setting")
	}
	if _, e = Restore(ctx, c, b, true, true); e != nil || b.mode != 1 {
		t.Fatal(e, b)
	}
}
func TestLockAndCorruptState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.lock")
	unlock, e := Lock(path)
	if e != nil {
		t.Fatal(e)
	}
	defer unlock()
	if u, e := Lock(path); e == nil {
		u()
		t.Fatal("concurrent owner allowed")
	}
	c := testConfig(t)
	if e = AtomicJSON(c.StateFile, map[string]any{"version": 8}); e != nil {
		t.Fatal(e)
	}
	if _, e = Step(context.Background(), c, instant("2026-11-21T10:00:00+08:00"), &fakeBackend{mode: 1}, true, false); e == nil {
		t.Fatal("corrupt state ignored")
	}
}

func TestCASConflictPausesInsteadOfOverridingUserOnRetry(t *testing.T) {
	c := testConfig(t)
	c.Mode = "native-auto"
	b := &fakeBackend{mode: 1, failCode: "conflict"}
	at := instant("2026-11-21T10:00:00+08:00")
	if _, err := Step(context.Background(), c, at, b, true, false); err == nil {
		t.Fatal("expected CAS conflict")
	}
	b.failCode = ""
	b.mode = 0
	r, err := Step(context.Background(), c, at, b, true, false)
	if err != nil || r.Action != "manual_override" || b.writes != 0 {
		t.Fatal(r, err, b)
	}
}

func TestRestoreReconcilesLateCompletionEvenWhenPolicyIsOff(t *testing.T) {
	c := testConfig(t)
	b := &fakeBackend{mode: 1, unknown: true}
	ctx := context.Background()
	if _, err := Step(ctx, c, instant("2026-11-21T10:00:00+08:00"), b, true, false); err == nil {
		t.Fatal("expected uncertain write")
	}
	b.mode = 0
	b.unknown = false
	c.Mode = "off"
	r, err := Restore(ctx, c, b, true, false)
	if err != nil || r.Action != "restored" || b.mode != 1 {
		t.Fatal(r, err, b)
	}
}
