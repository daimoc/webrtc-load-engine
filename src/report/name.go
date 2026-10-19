package report

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// maxSlug is the longest name part of a report's file name.
const maxSlug = 40

// baseName is the file name of a run's reports without extension
// (research R3): end time (UTC), platform, name, run ID. An empty name
// slug falls back to the scenario ID's.
func baseName(endedAt time.Time, platform, name, scenarioID, runID string) string {
	s := slug(name)
	if s == "" {
		s = slug(scenarioID)
	}
	return endedAt.UTC().Format("20060102T150405Z") + "-" + slug(platform) + "-" + s + "-" + runID
}

// slug lower-cases s, keeps [a-z0-9], maps every other character to a
// dash, collapses and trims dashes, and cuts at maxSlug characters.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			continue
		}
		dash = true
	}
	out := b.String()
	if len(out) > maxSlug {
		out = strings.TrimRight(out[:maxSlug], "-")
	}
	return out
}

// newRunID returns 8 lowercase hex characters, new for each run.
func newRunID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
}
