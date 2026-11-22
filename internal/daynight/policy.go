package daynight

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type Config struct {
	Revision              uint64   `json:"revision"`
	Mode                  string   `json:"mode"`
	Timezone              string   `json:"timezone"`
	DayStart              string   `json:"day_start"`
	NightStart            string   `json:"night_start"`
	Latitude              *float64 `json:"latitude"`
	Longitude             *float64 `json:"longitude"`
	PositionFile          string   `json:"position_file"`
	PositionMaxAgeSeconds int      `json:"position_max_age_seconds"`
	SunriseOffsetMinutes  int      `json:"sunrise_offset_minutes"`
	SunsetOffsetMinutes   int      `json:"sunset_offset_minutes"`
	RespectManual         bool     `json:"respect_manual"`
	IntervalSeconds       int      `json:"interval_seconds"`
	BridgeAddress         string   `json:"bridge_address"`
	TokenFile             string   `json:"token_file"`
	StateFile             string   `json:"state_file"`
}

func DefaultConfig() Config {
	return Config{Mode: "off", Timezone: "Asia/Shanghai", DayStart: "07:00", NightStart: "18:30", RespectManual: true,
		PositionMaxAgeSeconds: 3600, IntervalSeconds: 30, BridgeAddress: "127.0.0.1:47851", TokenFile: "bridge.token", StateFile: "state.json"}
}

func DecodeStrict(b []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return errors.New("expected exactly one JSON value")
	}
	return nil
}

func LoadConfig(path string) (Config, error) {
	c := DefaultConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err = DecodeStrict(b, &c); err != nil {
		return c, err
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	switch c.Mode {
	case "off", "native-auto", "fixed", "solar":
	default:
		return errors.New("mode must be off, native-auto, fixed or solar")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("timezone: %w", err)
	}
	d, err := clockMinute(c.DayStart)
	if err != nil {
		return err
	}
	n, err := clockMinute(c.NightStart)
	if err != nil {
		return err
	}
	if d == n {
		return errors.New("day_start and night_start must differ")
	}
	if (c.Latitude == nil) != (c.Longitude == nil) {
		return errors.New("latitude and longitude must be supplied together")
	}
	if c.Latitude != nil && !validCoordinates(*c.Latitude, *c.Longitude) {
		return errors.New("coordinates outside supported latitude -89..89 / longitude -180..180")
	}
	if c.SunriseOffsetMinutes < -180 || c.SunriseOffsetMinutes > 180 || c.SunsetOffsetMinutes < -180 || c.SunsetOffsetMinutes > 180 {
		return errors.New("solar offsets must be within -180..180 minutes")
	}
	if c.IntervalSeconds < 5 || c.IntervalSeconds > 300 {
		return errors.New("interval_seconds must be 5..300")
	}
	if c.PositionMaxAgeSeconds < 60 || c.PositionMaxAgeSeconds > 86400 {
		return errors.New("position_max_age_seconds must be 60..86400")
	}
	if !strings.HasPrefix(c.BridgeAddress, "127.0.0.1:") {
		return errors.New("bridge_address must use 127.0.0.1")
	}
	port, e := strconv.Atoi(strings.TrimPrefix(c.BridgeAddress, "127.0.0.1:"))
	if e != nil || port < 1024 || port > 65535 {
		return errors.New("invalid bridge port")
	}
	if c.TokenFile == "" || c.StateFile == "" {
		return errors.New("token_file and state_file are required")
	}
	return nil
}

func clockMinute(s string) (int, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("invalid HH:MM: %q", s)
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}
func validCoordinates(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && !math.IsInf(lat, 0) && !math.IsInf(lon, 0) && lat >= -89 && lat <= 89 && lon >= -180 && lon <= 180
}

type Position struct {
	Latitude         *float64  `json:"latitude"`
	Longitude        *float64  `json:"longitude"`
	Timestamp        time.Time `json:"timestamp"`
	Valid            bool      `json:"valid"`
	CoordinateSystem string    `json:"coordinate_system"`
}

func (c Config) coordinates(now time.Time) (float64, float64, string, error) {
	if c.PositionFile != "" {
		b, err := os.ReadFile(c.PositionFile)
		if err == nil {
			var p Position
			if DecodeStrict(b, &p) == nil && p.Valid && p.CoordinateSystem == "WGS84" && p.Latitude != nil && p.Longitude != nil && validCoordinates(*p.Latitude, *p.Longitude) && !p.Timestamp.IsZero() && now.Sub(p.Timestamp) >= -time.Minute && now.Sub(p.Timestamp) <= time.Duration(c.PositionMaxAgeSeconds)*time.Second {
				return *p.Latitude, *p.Longitude, "position_file", nil
			}
		}
	}
	if c.Latitude != nil {
		return *c.Latitude, *c.Longitude, "configured_coordinates", nil
	}
	return 0, 0, "", errors.New("no valid position; using fixed-time fallback")
}

type Decision struct {
	Policy       string     `json:"policy"`
	DesiredMode  *int       `json:"desired_mode"`
	Reason       string     `json:"reason"`
	NextBoundary time.Time  `json:"next_boundary,omitempty"`
	Sunrise      *time.Time `json:"sunrise,omitempty"`
	Sunset       *time.Time `json:"sunset,omitempty"`
}

func modePtr(n int) *int { return &n }

func Decide(c Config, now time.Time) (Decision, error) {
	if err := c.Validate(); err != nil {
		return Decision{}, err
	}
	d := Decision{Policy: c.Mode}
	if c.Mode == "off" {
		d.Reason = "automatic control disabled"
		return d, nil
	}
	if c.Mode == "native-auto" {
		d.DesiredMode = modePtr(2)
		d.Reason = "delegate to native vehicle day/night signal"
		return d, nil
	}
	if now.Year() < 2024 || now.Year() > 2100 {
		return d, errors.New("system date is outside 2024..2100; keep current theme")
	}
	loc, _ := time.LoadLocation(c.Timezone)
	now = now.In(loc)
	if c.Mode == "fixed" {
		return fixedDecision(c, now), nil
	}
	lat, lon, source, err := c.coordinates(now)
	if err != nil {
		d = fixedDecision(c, now)
		d.Policy = "solar"
		d.Reason = err.Error()
		return d, nil
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	events := make([]boundary, 0, 10)
	var todayPolar int
	for i := -2; i <= 2; i++ {
		date := today.AddDate(0, 0, i)
		rise, set, polar := SolarEvents(date, lat, lon)
		if i == 0 {
			todayPolar = polar
			if polar == 0 {
				if rise.IsZero() || set.IsZero() {
					return d, errors.New("incomplete solar events near polar transition; keep current theme")
				}
				r, s := rise, set
				d.Sunrise = &r
				d.Sunset = &s
			}
		}
		if polar == 0 && !rise.IsZero() && !set.IsZero() {
			r, s := rise.Add(time.Duration(c.SunriseOffsetMinutes)*time.Minute), set.Add(time.Duration(c.SunsetOffsetMinutes)*time.Minute)
			if !r.Before(s) {
				return d, errors.New("solar offsets overlap day interval; keep current theme")
			}
			events = append(events, boundary{r, 0}, boundary{s, 1})
		}
	}
	if todayPolar != 0 {
		d.DesiredMode = modePtr(1)
		if todayPolar == 1 {
			d.DesiredMode = modePtr(0)
		}
		d.NextBoundary = today.AddDate(0, 0, 1)
		d.Reason = source + ": polar day/night; daily reevaluation"
		return d, nil
	}
	sort.Slice(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })
	for _, e := range events {
		if !e.at.After(now) {
			d.DesiredMode = modePtr(e.mode)
		} else if d.NextBoundary.IsZero() {
			d.NextBoundary = e.at
		}
	}
	if d.DesiredMode == nil || d.NextBoundary.IsZero() {
		return d, errors.New("solar boundary selection failed; keep current theme")
	}
	d.Reason = source + ": offline NOAA approximation; offsets applied"
	return d, nil
}

type boundary struct {
	at   time.Time
	mode int
}

func fixedDecision(c Config, now time.Time) Decision {
	dm, _ := clockMinute(c.DayStart)
	nm, _ := clockMinute(c.NightStart)
	minute := now.Hour()*60 + now.Minute()
	day := minute >= dm && minute < nm
	if dm > nm {
		day = minute >= dm || minute < nm
	}
	mode := 1
	if day {
		mode = 0
	}
	d := Decision{Policy: c.Mode, DesiredMode: modePtr(mode), Reason: "configured local time interval"}
	for i := 0; i <= 2; i++ {
		date := now.AddDate(0, 0, i)
		for _, m := range []int{dm, nm} {
			at := time.Date(date.Year(), date.Month(), date.Day(), m/60, m%60, 0, 0, now.Location())
			if at.After(now) && (d.NextBoundary.IsZero() || at.Before(d.NextBoundary)) {
				d.NextBoundary = at
			}
		}
	}
	return d
}

// NOAA's published fractional-year approximation, zenith 90.833 degrees.
// Dates are civil dates in date.Location(). UTC events can fall on an adjacent day.
// Returns polar=1 for all-day daylight, -1 for all-day darkness, 0 for crossings.
// Approximate theme scheduling only; not a navigation/astronomy instrument.
func SolarEvents(date time.Time, lat, lon float64) (time.Time, time.Time, int) {
	loc := date.Location()
	y, m, d := date.Date()
	anchor := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	var rise, set time.Time
	polar := 0
	for offset := -1; offset <= 1; offset++ {
		utcDate := anchor.AddDate(0, 0, offset)
		days := 365.
		year := utcDate.Year()
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			days = 366
		}
		gamma := 2 * math.Pi / days * float64(utcDate.YearDay()-1)
		eq := 229.18 * (0.000075 + 0.001868*math.Cos(gamma) - 0.032077*math.Sin(gamma) - 0.014615*math.Cos(2*gamma) - 0.040849*math.Sin(2*gamma))
		decl := 0.006918 - 0.399912*math.Cos(gamma) + 0.070257*math.Sin(gamma) - 0.006758*math.Cos(2*gamma) + 0.000907*math.Sin(2*gamma) - 0.002697*math.Cos(3*gamma) + 0.00148*math.Sin(3*gamma)
		latRad := lat * math.Pi / 180
		x := (math.Cos(90.833*math.Pi/180) - math.Sin(latRad)*math.Sin(decl)) / (math.Cos(latRad) * math.Cos(decl))
		if x < -1 {
			if offset == 0 {
				polar = 1
			}
			continue
		}
		if x > 1 {
			if offset == 0 {
				polar = -1
			}
			continue
		}
		ha := math.Acos(x) * 180 / math.Pi
		for _, isRise := range []bool{true, false} {
			angle := ha
			if !isRise {
				angle = -ha
			}
			mins := 720 - 4*(lon+angle) - eq
			at := utcDate.Add(time.Duration(math.Round(mins*60)) * time.Second).In(loc)
			ay, am, ad := at.Date()
			if ay == y && am == m && ad == d {
				if isRise {
					rise = at
				} else {
					set = at
				}
			}
		}
	}
	return rise, set, polar
}
