package report

import (
	"regexp"
	"testing"
	"time"
)

func TestBaseName(t *testing.T) {
	end := time.Date(2026, 10, 18, 17, 30, 12, 0, time.FixedZone("CEST", 2*3600))
	got := baseName(end, "jitsi", "Ten bots", "scenario-1", "3f9a1c2e")
	if want := "20261018T153012Z-jitsi-ten-bots-3f9a1c2e"; got != want {
		t.Errorf("baseName = %q, want %q", got, want)
	}
	if got := baseName(end, "livekit", "///", "scenario-AB12", "00000000"); got != "20261018T153012Z-livekit-scenario-ab12-00000000" {
		t.Errorf("empty slug: baseName = %q", got)
	}
	if got := baseName(end, "jitsi", "", "scenario-x", "00000000"); got != "20261018T153012Z-jitsi-scenario-x-00000000" {
		t.Errorf("empty name: baseName = %q", got)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Ten bots":      "ten-bots",
		"lk-10-bots":    "lk-10-bots",
		"a/b c.d":       "a-b-c-d",
		"Réunion d'été": "r-union-d-t",
		"--x--y--":      "x-y",
		"":              "",
		"///":           "",
		"0123456789abcdefghijklmnopqrstuvwxyz0123456789": "0123456789abcdefghijklmnopqrstuvwxyz0123",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
	// A cut must not leave a trailing dash.
	if got := slug("abcdefghijklmnopqrstuvwxyz0123456789abc d"); got != "abcdefghijklmnopqrstuvwxyz0123456789abc" {
		t.Errorf("cut slug = %q", got)
	}
}

func TestNewRunID(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}$`)
	a, b := newRunID(), newRunID()
	if !re.MatchString(a) || !re.MatchString(b) {
		t.Errorf("run IDs %q, %q", a, b)
	}
	if a == b {
		t.Errorf("two run IDs equal: %q", a)
	}
}
