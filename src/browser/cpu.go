package browser

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// clockTicks is the kernel's USER_HZ, the unit of /proc/<pid>/stat times
// (100 on Linux on every architecture the project runs on).
const clockTicks = 100

// groupCPU sums the CPU time of the processes of one process group
// (research R4): Chrome's renderer and GPU processes are its children, and
// killing the group leaves them unwaited, so the CPU time the browser's own
// process reports at its end misses them. Each scan keeps, per process, the
// largest time seen, so a process that ended between scans keeps its last
// reading.
type groupCPU struct {
	mu    sync.Mutex
	ticks map[int]uint64 // pid → user + system ticks; guarded by mu
}

// scan reads the processes of group pgid from /proc.
func (g *groupCPU) scan(pgid int) {
	if pgid <= 0 {
		return
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ticks == nil {
		g.ticks = map[int]uint64{}
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		group, ticks, ok := parseStat(raw)
		if !ok || group != pgid {
			continue
		}
		if ticks > g.ticks[pid] {
			g.ticks[pid] = ticks
		}
	}
}

// parseStat reads the process group and the user + system ticks of a
// /proc/<pid>/stat line. The command name, field 2, is in parentheses and
// may hold spaces: fields are counted after its closing one.
func parseStat(raw []byte) (pgrp int, ticks uint64, ok bool) {
	i := bytes.LastIndexByte(raw, ')')
	if i < 0 {
		return 0, 0, false
	}
	fields := bytes.Fields(raw[i+1:])
	// After the name: state(3) ppid(4) pgrp(5) ... utime(14) stime(15).
	if len(fields) < 13 {
		return 0, 0, false
	}
	pgrp, err1 := strconv.Atoi(string(fields[2]))
	utime, err2 := strconv.ParseUint(string(fields[11]), 10, 64)
	stime, err3 := strconv.ParseUint(string(fields[12]), 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, false
	}
	return pgrp, utime + stime, true
}

// seconds is the CPU time seen over the group.
func (g *groupCPU) seconds() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	var total uint64
	for _, t := range g.ticks {
		total += t
	}
	return float64(total) / clockTicks
}
