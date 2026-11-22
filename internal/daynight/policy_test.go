package daynight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func instant(s string) time.Time {
	v, e := time.Parse(time.RFC3339, s)
	if e != nil {
		panic(e)
	}
	return v
}
func coordinate(v float64) *float64 { return &v }
func TestFixedBoundariesAndOvernight(t *testing.T) {
	c := DefaultConfig()
	c.Mode = "fixed"
	for _, x := range []struct {
		at   string
		mode int
		next string
	}{
		{"2026-11-21T06:59:59+08:00", 1, "2026-11-21T07:00:00+08:00"},
		{"2026-11-21T07:00:00+08:00", 0, "2026-11-21T18:30:00+08:00"},
		{"2026-11-21T18:30:00+08:00", 1, "2026-11-22T07:00:00+08:00"},
	} {
		d, e := Decide(c, instant(x.at))
		if e != nil || *d.DesiredMode != x.mode || !d.NextBoundary.Equal(instant(x.next)) {
			t.Fatalf("%s: %+v %v", x.at, d, e)
		}
	}
	c.DayStart = "20:00"
	c.NightStart = "08:00"
	for _, x := range []struct {
		at   string
		mode int
	}{{"2026-11-21T23:00:00+08:00", 0}, {"2026-11-21T07:00:00+08:00", 0}, {"2026-11-21T12:00:00+08:00", 1}} {
		d, e := Decide(c, instant(x.at))
		if e != nil || *d.DesiredMode != x.mode {
			t.Fatal(d, e)
		}
	}
}
func TestSolarIndependentReference(t *testing.T) {
	// Hong Kong Observatory 2026 almanac, June 21: sunrise 05:40, sunset 19:10 HKT.
	// https://www.hko.gov.hk/en/gts/astron2026/files/2026cal06.pdf
	// Approximation is deliberately allowed four minutes, not advertised as minute-accurate.
	loc, _ := time.LoadLocation("Asia/Hong_Kong")
	date := time.Date(2026, 6, 21, 0, 0, 0, 0, loc)
	rise, set, polar := SolarEvents(date, 22.302, 114.174)
	for _, x := range []struct{ got, want time.Time }{{rise, instant("2026-06-21T05:40:00+08:00")}, {set, instant("2026-06-21T19:10:00+08:00")}} {
		delta := x.got.Sub(x.want)
		if delta < -4*time.Minute || delta > 4*time.Minute {
			t.Fatalf("solar reference %v, want near %v", x.got, x.want)
		}
	}
	if polar != 0 {
		t.Fatal(polar)
	}
}
func TestSolarDatelineLeapAndPolar(t *testing.T) {
	for _, x := range []struct {
		zone, date string
		lat, lon   float64
	}{
		{"Pacific/Kiritimati", "2026-06-21", 1.872, -157.427},
		{"Pacific/Pago_Pago", "2026-06-21", -14.27, -170.70},
		{"Asia/Shanghai", "2028-02-29", 31.23, 121.47},
	} {
		loc, _ := time.LoadLocation(x.zone)
		d, _ := time.ParseInLocation("2006-01-02", x.date, loc)
		r, s, p := SolarEvents(d, x.lat, x.lon)
		if p != 0 || r.Format("2006-01-02") != x.date || s.Format("2006-01-02") != x.date || !r.Before(s) {
			t.Fatalf("%s: %v %v polar %d", x.zone, r, s, p)
		}
	}
	c := DefaultConfig()
	c.Mode = "solar"
	c.Timezone = "Europe/Oslo"
	c.Latitude = coordinate(69.65)
	c.Longitude = coordinate(18.96)
	for _, x := range []struct {
		date string
		mode int
	}{{"2026-06-21T12:00:00+02:00", 0}, {"2026-12-21T12:00:00+01:00", 1}} {
		d, e := Decide(c, instant(x.date))
		if e != nil || *d.DesiredMode != x.mode {
			t.Fatalf("polar: %+v %v", d, e)
		}
	}
}
func TestSolarOffsetsAndPositionFallback(t *testing.T) {
	c := DefaultConfig()
	c.Mode = "solar"
	c.Latitude = coordinate(22.302)
	c.Longitude = coordinate(114.174)
	c.SunsetOffsetMinutes = -20
	noon := instant("2026-06-21T12:00:00+08:00")
	d, e := Decide(c, noon)
	if e != nil {
		t.Fatal(e)
	}
	boundary := d.Sunset.Add(-20 * time.Minute)
	if !d.NextBoundary.Equal(boundary) {
		t.Fatal(d)
	}
	before, _ := Decide(c, boundary.Add(-time.Second))
	after, _ := Decide(c, boundary)
	if *before.DesiredMode != 0 || *after.DesiredMode != 1 {
		t.Fatal(before, after)
	}
	c.Latitude = nil
	c.Longitude = nil
	c.PositionFile = filepath.Join(t.TempDir(), "position.json")
	// Missing coordinates must never silently become 0,0.
	os.WriteFile(c.PositionFile, []byte(`{"valid":true,"coordinate_system":"WGS84","timestamp":"2026-06-21T12:00:00+08:00"}`), 0600)
	d, e = Decide(c, noon)
	if e != nil || !strings.Contains(d.Reason, "fallback") {
		t.Fatal(d, e)
	}
	p := Position{Latitude: coordinate(22.302), Longitude: coordinate(114.174), Valid: true, CoordinateSystem: "WGS84", Timestamp: noon}
	if e = AtomicJSON(c.PositionFile, p); e != nil {
		t.Fatal(e)
	}
	d, e = Decide(c, noon)
	if e != nil || !strings.Contains(d.Reason, "position_file") {
		t.Fatal(d, e)
	}
	d, e = Decide(c, noon.Add(2*time.Hour))
	if e != nil || !strings.Contains(d.Reason, "fallback") {
		t.Fatal(d, e)
	}
}
func TestConfigAndClockFailures(t *testing.T) {
	c := DefaultConfig()
	c.Mode = "fixed"
	if _, e := Decide(c, time.Unix(0, 0)); e == nil {
		t.Fatal("bad clock accepted")
	}
	c.Mode = "off"
	if _, e := Decide(c, time.Unix(0, 0)); e != nil {
		t.Fatal(e)
	}
	c.Mode = "native-auto"
	d, e := Decide(c, time.Unix(0, 0))
	if e != nil || *d.DesiredMode != 2 {
		t.Fatal(d, e)
	}
	for _, edit := range []func(*Config){func(c *Config) { c.DayStart = c.NightStart }, func(c *Config) { c.BridgeAddress = "0.0.0.0:47851" }, func(c *Config) { c.Latitude = coordinate(10) }, func(c *Config) { c.IntervalSeconds = 0 }} {
		c = DefaultConfig()
		edit(&c)
		if c.Validate() == nil {
			t.Fatal("invalid config accepted")
		}
	}
	if DecodeStrict([]byte(`{"unexpected":true}`), &c) == nil || DecodeStrict([]byte(`{} {}`), &c) == nil {
		t.Fatal("non-strict JSON")
	}
}
