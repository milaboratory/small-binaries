// Package cgroup reads the resource consumption of the cgroup the wrapper runs in - which, inside
// a job container, is the container's own cgroup and therefore covers the command and everything
// it spawns.
//
// Both cgroup v2 (unified) and v1 layouts are supported. Every reader takes explicit directories so
// it can be exercised against fixture files on any OS; Discover locates the real ones on Linux.
package cgroup

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrUnavailable is returned by Discover when no cgroup can be found for this process.
var ErrUnavailable = errors.New("cgroup accounting is not available")

// Above this a v1 limit means "unlimited" (the file holds a page-rounded max int64).
const noLimitThreshold = 1 << 60

// MemoryStats is one reading of the memory controller.
type MemoryStats struct {
	// Current is memory.current (v2) / memory.usage_in_bytes (v1): anon + file cache + kernel.
	Current uint64
	// Peak is the kernel-maintained high-water mark of Current. HasPeak is false where the kernel
	// does not expose it (cgroup v2 before Linux 5.19).
	Peak    uint64
	HasPeak bool
	// InactiveFile is the reclaimable part of the file cache; Current minus it is the working set.
	InactiveFile uint64
	// OOMKills counts processes the kernel OOM killer took from this cgroup (memory.events
	// oom_kill / memory.oom_control oom_kill). OOMGroupKills is memory.events oom_group_kill.
	OOMKills      uint64
	OOMGroupKills uint64
}

// CPUStats is one reading of the cpu controller.
type CPUStats struct {
	UsageUsec     uint64
	NrThrottled   uint64
	ThrottledUsec uint64
}

// IOStats is the syscall-level IO (rchar / wchar from /proc/<pid>/io) summed over every process
// of the cgroup. The counters are cumulative and monotonic across process exits.
type IOStats struct {
	ReadBytes  uint64
	WriteBytes uint64
}

// Stats is one sample. A nil section means that controller could not be read.
type Stats struct {
	At     time.Time
	Memory *MemoryStats
	CPU    *CPUStats
	IO     *IOStats
}

// Limits is what the cgroup grants. Zero means "no limit".
type Limits struct {
	MemoryBytes   uint64 `json:"ram,omitempty"`
	CPUMillicores uint64 `json:"cpu,omitempty"`
}

// Info describes where readings come from.
type Info struct {
	Available bool   `json:"available"`
	Version   int    `json:"version,omitempty"`
	Path      string `json:"path,omitempty"`
	CPUPath   string `json:"cpuPath,omitempty"` // v1 only, when it differs from Path
	Error     string `json:"error,omitempty"`
}

// Source reads one cgroup.
type Source interface {
	Read() (Stats, error)
	Limits() Limits
	Info() Info
}

// ---- file helpers ----

func readTrim(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func readUint(p string) (uint64, error) {
	s, err := readTrim(p)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 10, 64)
}

// readKV parses "key value" lines (memory.stat, cpu.stat, memory.events, memory.oom_control).
func readKV(p string) (map[string]uint64, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	m := make(map[string]uint64)
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		m[fields[0]] = v
	}
	return m, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// ---- cgroup v2 ----

type v2 struct {
	dir string
	io  *ioTracker
}

// NewV2 reads the unified hierarchy at dir. procRoot is where /proc is mounted; an empty value
// disables IO accounting.
func NewV2(dir, procRoot string) Source {
	return &v2{dir: dir, io: newIOTracker(procRoot)}
}

func (c *v2) Info() Info { return Info{Available: true, Version: 2, Path: c.dir} }

func (c *v2) Read() (Stats, error) {
	st := Stats{At: time.Now()}
	if cur, err := readUint(filepath.Join(c.dir, "memory.current")); err == nil {
		m := &MemoryStats{Current: cur}
		if peak, err := readUint(filepath.Join(c.dir, "memory.peak")); err == nil {
			m.Peak, m.HasPeak = peak, true
		}
		if kv, err := readKV(filepath.Join(c.dir, "memory.stat")); err == nil {
			m.InactiveFile = kv["inactive_file"]
		}
		if kv, err := readKV(filepath.Join(c.dir, "memory.events")); err == nil {
			m.OOMKills = kv["oom_kill"]
			m.OOMGroupKills = kv["oom_group_kill"]
		}
		st.Memory = m
	}
	if kv, err := readKV(filepath.Join(c.dir, "cpu.stat")); err == nil {
		if _, ok := kv["usage_usec"]; ok {
			st.CPU = &CPUStats{
				UsageUsec:     kv["usage_usec"],
				NrThrottled:   kv["nr_throttled"],
				ThrottledUsec: kv["throttled_usec"],
			}
		}
	}
	if io, ok := c.io.read(filepath.Join(c.dir, "cgroup.procs")); ok {
		st.IO = &io
	}
	if st.Memory == nil && st.CPU == nil {
		return st, errors.New("neither memory.current nor cpu.stat readable in " + c.dir)
	}
	return st, nil
}

func (c *v2) Limits() Limits {
	var l Limits
	if s, err := readTrim(filepath.Join(c.dir, "memory.max")); err == nil && s != "max" {
		if v, err := strconv.ParseUint(s, 10, 64); err == nil {
			l.MemoryBytes = v
		}
	}
	// cpu.max: "<quota> <period>" or "max <period>".
	if s, err := readTrim(filepath.Join(c.dir, "cpu.max")); err == nil {
		l.CPUMillicores = parseCPUQuota(s)
	}
	return l
}

// parseCPUQuota converts a "<quota> <period>" pair (microseconds) to millicores; "max" is 0.
func parseCPUQuota(s string) uint64 {
	fields := strings.Fields(s)
	if len(fields) != 2 || fields[0] == "max" {
		return 0
	}
	quota, err1 := strconv.ParseUint(fields[0], 10, 64)
	period, err2 := strconv.ParseUint(fields[1], 10, 64)
	if err1 != nil || err2 != nil || period == 0 {
		return 0
	}
	return quota * 1000 / period
}

// ---- cgroup v1 ----

type v1 struct {
	memDir string
	cpuDir string
	io     *ioTracker
}

// NewV1 reads the legacy hierarchy: memDir is the memory controller directory, cpuDir the
// cpu,cpuacct (or cpuacct) one. Either may be empty.
func NewV1(memDir, cpuDir, procRoot string) Source {
	return &v1{memDir: memDir, cpuDir: cpuDir, io: newIOTracker(procRoot)}
}

func (c *v1) Info() Info {
	info := Info{Available: true, Version: 1, Path: c.memDir}
	if c.cpuDir != c.memDir {
		info.CPUPath = c.cpuDir
	}
	return info
}

func (c *v1) Read() (Stats, error) {
	st := Stats{At: time.Now()}
	if c.memDir != "" {
		if cur, err := readUint(filepath.Join(c.memDir, "memory.usage_in_bytes")); err == nil {
			m := &MemoryStats{Current: cur}
			if peak, err := readUint(filepath.Join(c.memDir, "memory.max_usage_in_bytes")); err == nil {
				m.Peak, m.HasPeak = peak, true
			}
			if kv, err := readKV(filepath.Join(c.memDir, "memory.stat")); err == nil {
				if v, ok := kv["total_inactive_file"]; ok {
					m.InactiveFile = v
				} else {
					m.InactiveFile = kv["inactive_file"]
				}
			}
			if kv, err := readKV(filepath.Join(c.memDir, "memory.oom_control")); err == nil {
				m.OOMKills = kv["oom_kill"]
			}
			st.Memory = m
		}
	}
	if c.cpuDir != "" {
		if ns, err := readUint(filepath.Join(c.cpuDir, "cpuacct.usage")); err == nil {
			cpu := &CPUStats{UsageUsec: ns / 1000}
			if kv, err := readKV(filepath.Join(c.cpuDir, "cpu.stat")); err == nil {
				cpu.NrThrottled = kv["nr_throttled"]
				cpu.ThrottledUsec = kv["throttled_time"] / 1000
			}
			st.CPU = cpu
		}
	}
	procs := filepath.Join(c.memDir, "cgroup.procs")
	if c.memDir == "" {
		procs = filepath.Join(c.cpuDir, "cgroup.procs")
	}
	if io, ok := c.io.read(procs); ok {
		st.IO = &io
	}
	if st.Memory == nil && st.CPU == nil {
		return st, errors.New("neither memory.usage_in_bytes nor cpuacct.usage readable")
	}
	return st, nil
}

func (c *v1) Limits() Limits {
	var l Limits
	if c.memDir != "" {
		if v, err := readUint(filepath.Join(c.memDir, "memory.limit_in_bytes")); err == nil && v < noLimitThreshold {
			l.MemoryBytes = v
		}
	}
	if c.cpuDir != "" {
		quota, err1 := readTrim(filepath.Join(c.cpuDir, "cpu.cfs_quota_us"))
		period, err2 := readTrim(filepath.Join(c.cpuDir, "cpu.cfs_period_us"))
		if err1 == nil && err2 == nil && !strings.HasPrefix(quota, "-") {
			l.CPUMillicores = parseCPUQuota(quota + " " + period)
		}
	}
	return l
}

// ---- syscall-level IO over the cgroup's processes ----

type ioCounters struct{ r, w uint64 }

// ioTracker sums rchar/wchar over the PIDs listed in cgroup.procs. A process that exited between
// two samples keeps its last known counters in `finalized`, so the total never decreases.
type ioTracker struct {
	procRoot  string
	live      map[int]ioCounters
	finalized ioCounters
}

func newIOTracker(procRoot string) *ioTracker {
	if procRoot == "" {
		return nil
	}
	return &ioTracker{procRoot: procRoot, live: map[int]ioCounters{}}
}

func (t *ioTracker) read(procsFile string) (IOStats, bool) {
	if t == nil {
		return IOStats{}, false
	}
	b, err := os.ReadFile(procsFile)
	if err != nil {
		return IOStats{}, false
	}
	self := os.Getpid()
	next := make(map[int]ioCounters, len(t.live))
	for _, f := range strings.Fields(string(b)) {
		pid, err := strconv.Atoi(f)
		if err != nil || pid == self {
			continue
		}
		c, ok := readProcIO(filepath.Join(t.procRoot, f, "io"))
		if !ok {
			// Vanished or unreadable: keep what we knew, it is finalized below if it is gone.
			if prev, had := t.live[pid]; had {
				next[pid] = prev
			}
			continue
		}
		next[pid] = c
	}
	for pid, prev := range t.live {
		if _, still := next[pid]; !still {
			t.finalized.r += prev.r
			t.finalized.w += prev.w
		}
	}
	t.live = next
	total := t.finalized
	for _, c := range next {
		total.r += c.r
		total.w += c.w
	}
	return IOStats{ReadBytes: total.r, WriteBytes: total.w}, true
}

func readProcIO(p string) (ioCounters, bool) {
	kv, err := readKV(p)
	if err != nil {
		return ioCounters{}, false
	}
	r, okR := kv["rchar:"]
	w, okW := kv["wchar:"]
	if !okR || !okW {
		return ioCounters{}, false
	}
	return ioCounters{r: r, w: w}, true
}
