package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"local/vw-daynight/internal/daynight"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func emit(v any) { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }
func run() error {
	f := flag.NewFlagSet("daynight", flag.ContinueOnError)
	configPath := f.String("config", "config.json", "configuration path")
	write := f.Bool("write", false, "allow real theme changes; bridge must also allow_write=true")
	force := f.Bool("force", false, "explicitly rearm a manual override/uncertain result")
	at := f.String("at", "", "RFC3339 time, preview only")
	f.Usage = func() {
		fmt.Fprintln(f.Output(), "daynight [flags] init|configure [options]|select MODE|preview|status|apply|run|restore\nModes: off, native-auto, fixed, solar. Default: read-only; use --write after vehicle verification.")
		f.PrintDefaults()
	}
	if err := f.Parse(os.Args[1:]); err != nil {
		return err
	}
	args := f.Args()
	if len(args) == 0 {
		f.Usage()
		return nil
	}
	abs, err := filepath.Abs(*configPath)
	if err != nil {
		return err
	}
	*configPath = abs
	if args[0] == "init" {
		if len(args) != 1 {
			return errors.New("init takes no arguments")
		}
		if _, e := os.Lstat(abs); !os.IsNotExist(e) {
			return errors.New("configuration already exists or is inaccessible")
		}
		return daynight.AtomicJSON(abs, daynight.DefaultConfig())
	}
	c, err := daynight.LoadConfig(abs)
	if err != nil {
		return err
	}
	if args[0] == "configure" {
		edit := flag.NewFlagSet("configure", flag.ContinueOnError)
		edit.StringVar(&c.DayStart, "day-start", c.DayStart, "local HH:MM")
		edit.StringVar(&c.NightStart, "night-start", c.NightStart, "local HH:MM")
		edit.StringVar(&c.Timezone, "timezone", c.Timezone, "IANA time zone")
		edit.StringVar(&c.PositionFile, "position-file", c.PositionFile, "optional WGS84 position JSON")
		edit.IntVar(&c.SunriseOffsetMinutes, "sunrise-offset", c.SunriseOffsetMinutes, "minutes after sunrise; negative means earlier")
		edit.IntVar(&c.SunsetOffsetMinutes, "sunset-offset", c.SunsetOffsetMinutes, "minutes after sunset; negative means earlier")
		edit.BoolVar(&c.RespectManual, "respect-manual", c.RespectManual, "pause after manual changes")
		lat := edit.String("latitude", "", "configured city latitude, decimal degrees")
		lon := edit.String("longitude", "", "configured city longitude, decimal degrees")
		clear := edit.Bool("clear-position", false, "remove configured city and position file")
		if err = edit.Parse(args[1:]); err != nil {
			return err
		}
		if len(edit.Args()) != 0 {
			return errors.New("unexpected configure arguments")
		}
		if *clear {
			c.Latitude = nil
			c.Longitude = nil
			c.PositionFile = ""
		}
		if (*lat == "") != (*lon == "") {
			return errors.New("provide both --latitude and --longitude")
		}
		if *lat != "" {
			a, e := strconv.ParseFloat(*lat, 64)
			if e != nil {
				return e
			}
			b, e := strconv.ParseFloat(*lon, 64)
			if e != nil {
				return e
			}
			c.Latitude = &a
			c.Longitude = &b
		}
		if err = c.Validate(); err != nil {
			return err
		}
		c.Revision++
		if err = daynight.AtomicJSON(abs, c); err != nil {
			return err
		}
		emit(map[string]any{"configured": true, "mode": c.Mode})
		return nil
	}
	if args[0] == "select" {
		if len(args) != 2 {
			return errors.New("select requires one mode")
		}
		c.Mode = args[1]
		c.Revision++
		if err = c.Validate(); err != nil {
			return err
		}
		if err = daynight.AtomicJSON(abs, c); err != nil {
			return err
		}
		emit(map[string]any{"selected": c.Mode, "note": "configuration saved; running controller reloads it, otherwise apply explicitly"})
		return nil
	}
	if len(args) != 1 {
		return errors.New("unexpected arguments; put flags before command")
	}
	if *at != "" && args[0] != "preview" {
		return errors.New("--at is supported only for preview")
	}
	if *force && args[0] != "apply" && args[0] != "restore" {
		return errors.New("--force is supported only for explicit apply/restore")
	}
	resolve := func(c *daynight.Config) {
		for _, p := range []*string{&c.TokenFile, &c.StateFile, &c.PositionFile} {
			if *p != "" && !filepath.IsAbs(*p) {
				*p = filepath.Join(filepath.Dir(abs), *p)
			}
		}
	}
	resolve(&c)
	if c.StateFile == abs || c.TokenFile == abs || c.StateFile == c.TokenFile {
		return errors.New("config, state and token paths must be distinct")
	}
	now := time.Now()
	if *at != "" {
		now, err = time.Parse(time.RFC3339, *at)
		if err != nil {
			return err
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	bridge := func(c daynight.Config) daynight.Backend {
		return daynight.SocketBridge{Address: c.BridgeAddress, TokenFile: c.TokenFile}
	}
	switch args[0] {
	case "preview":
		d, e := daynight.Decide(c, now)
		if e != nil {
			return e
		}
		emit(d)
		return nil
	case "status":
		r, e := bridge(c).Status(ctx)
		if e != nil {
			return e
		}
		emit(r)
		return nil
	case "apply", "restore", "run":
		unlock, e := daynight.Lock(c.StateFile + ".lock")
		if e != nil {
			return e
		}
		defer unlock()
		if args[0] == "restore" {
			r, e := daynight.Restore(ctx, c, bridge(c), *write, *force)
			if e != nil {
				return e
			}
			emit(r)
			return nil
		}
		if args[0] == "apply" {
			r, e := daynight.Step(ctx, c, now, bridge(c), *write, *force)
			if e != nil {
				return e
			}
			emit(r)
			return nil
		}
		lockedState := c.StateFile
		last := ""
		for {
			fresh, e := daynight.LoadConfig(abs)
			if e == nil {
				resolve(&fresh)
				if fresh.StateFile != lockedState || fresh.StateFile == abs || fresh.TokenFile == abs || fresh.StateFile == fresh.TokenFile {
					e = errors.New("restart required for state path change, or conflicting paths")
				}
			}
			var out any
			if e == nil {
				c = fresh
				r, stepErr := daynight.Step(ctx, c, time.Now(), bridge(c), *write, false)
				if stepErr != nil {
					out = map[string]any{"error": stepErr.Error()}
				} else {
					out = r
				}
			} else {
				out = map[string]any{"error": e.Error()}
			}
			b, _ := json.Marshal(out)
			if string(b) != last {
				emit(out)
				last = string(b)
			}
			t := time.NewTimer(time.Duration(c.IntervalSeconds) * time.Second)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil
			case <-t.C:
			}
		}
	default:
		return errors.New("unknown command")
	}
}
