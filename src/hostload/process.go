package hostload

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// processSample returns the engine process's cumulative CPU time (user +
// system, as /usr/bin/time reports it) and its resident memory now
// (research R2). Linux only.
func processSample() (cpuNS uint64, rssBytes int64, err error) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, 0, fmt.Errorf("getrusage: %w", err)
	}
	cpuNS = uint64(ru.Utime.Nano() + ru.Stime.Nano())
	statm, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(string(statm))
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("/proc/self/statm: %q", statm)
	}
	pages, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("/proc/self/statm: %w", err)
	}
	return cpuNS, pages * int64(os.Getpagesize()), nil
}

// hardwareThreads returns the machine's online hardware threads.
func hardwareThreads() int {
	b, err := os.ReadFile("/sys/devices/system/cpu/online")
	if err != nil {
		return runtime.NumCPU()
	}
	if n := parseCPUList(string(b)); n > 0 {
		return n
	}
	return runtime.NumCPU()
}

// usableThreads returns the threads the engine may run on (its CPU affinity).
func usableThreads() int { return runtime.NumCPU() }

// parseCPUList counts the CPUs of a kernel CPU list ("0-3,8-11"); 0 if it
// cannot be parsed.
func parseCPUList(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n := 0
	for _, part := range strings.Split(s, ",") {
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(lo)
		if err != nil {
			return 0
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(hi); err != nil || b < a {
				return 0
			}
		}
		n += b - a + 1
	}
	return n
}
