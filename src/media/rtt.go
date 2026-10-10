package media

import "time"

// ntpEpochOffset is the number of seconds from 1900-01-01 (NTP) to
// 1970-01-01 (Unix).
const ntpEpochOffset = 2208988800

// compactNTP returns the middle 32 bits of the NTP timestamp of t (16.16
// fixed point seconds), the format of LSR and DLSR in RTCP reports.
func compactNTP(t time.Time) uint32 {
	secs := uint64(t.Unix()+ntpEpochOffset) << 16
	frac := uint64(t.Nanosecond()) << 16 / uint64(time.Second)
	return uint32(secs | frac)
}

// ReportRoundTrip computes the round-trip time from a reception report about one
// of our streams received at now (RFC 3550 6.4.1: A - LSR - DLSR). It reports
// false when the remote has not received a sender report yet (LSR 0) or the
// result is not positive.
func ReportRoundTrip(now time.Time, lsr, dlsr uint32) (time.Duration, bool) {
	if lsr == 0 {
		return 0, false
	}
	rtt := int32(compactNTP(now) - lsr - dlsr)
	if rtt <= 0 {
		return 0, false
	}
	return time.Duration(int64(rtt) * int64(time.Second) >> 16), true
}
