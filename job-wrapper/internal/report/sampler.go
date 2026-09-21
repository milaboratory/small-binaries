package report

import (
	"sync"
	"time"

	"github.com/milaboratory/small-binaries/job-wrapper/internal/cgroup"
)

// DefaultBaseInterval is the sampling cadence; DefaultMaxSamples the series budget before the
// resolution is halved in place.
const (
	DefaultBaseInterval = time.Second
	DefaultMaxSamples   = 240
)

// Peak sources, see Usage.PeakSource.
const (
	peakSourceKernel  = "kernel"
	peakSourceSampled = "sampled"
)

type point struct {
	at      time.Time
	cpuUsec uint64
	hasCPU  bool
	ram     uint64 // memory.current
	ws      uint64 // working set: current minus inactive file cache
	hasRAM  bool
	ioR     uint64
	ioW     uint64
	hasIO   bool
}

// peaks are tracked from every base-interval reading, independent of the retained series.
type peaks struct {
	ramSampled  uint64
	kernel      uint64
	hasKernel   bool
	cpuMilli    uint64
	ioReadRate  uint64
	ioWriteRate uint64
}

// Sampler turns a stream of cgroup readings into peaks and bounded series.
//
// Every reading updates the peaks, so peaks keep base-interval resolution for the whole run. Only
// every `stride`-th reading is retained for the series; when the retained points exceed
// maxSamples, every second one is dropped, the interval doubles and so does the stride. The series
// therefore always carry one honest interval, however long the run.
type Sampler struct {
	src        cgroup.Source
	base       time.Duration
	maxSamples int

	mu       sync.Mutex
	points   []point
	interval time.Duration
	stride   int
	ticks    int

	first    *point
	prev     *point
	observed int
	errors   int
	lastErr  string
	peaks    peaks

	startEvents *cgroup.MemoryStats
	lastMem     *cgroup.MemoryStats
	lastCPU     *cgroup.CPUStats

	start *Point
	end   *Point
}

// NewSampler creates a sampler over src. src may be nil, in which case every reading fails softly
// and the report carries no series.
func NewSampler(src cgroup.Source, base time.Duration, maxSamples int) *Sampler {
	if base <= 0 {
		base = DefaultBaseInterval
	}
	if maxSamples < 2 {
		maxSamples = DefaultMaxSamples
	}
	return &Sampler{src: src, base: base, maxSamples: maxSamples, interval: base, stride: 1}
}

// Start takes the reading that precedes the command and seeds the series.
func (s *Sampler) Start(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, st, ok := s.read(now)
	if !ok {
		return
	}
	s.startEvents = st.Memory
	s.start = toPoint(p)
	s.points = append(s.points, p)
}

// Tick takes one base-interval reading. It returns the memory reading (nil when unavailable) so
// the caller can react to high water without a second read.
func (s *Sampler) Tick(now time.Time) *cgroup.MemoryStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, st, ok := s.read(now)
	if !ok {
		return nil
	}
	s.ticks++
	if s.ticks%s.stride == 0 {
		s.points = append(s.points, p)
		s.downsample()
	}
	return st.Memory
}

// Finish takes the reading that follows the command's exit.
func (s *Sampler) Finish(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, _, ok := s.read(now)
	if !ok {
		return
	}
	s.end = toPoint(p)
}

// Fill writes the sampler's current view into r. Static fields of r are left alone.
func (s *Sampler) Fill(r *Report) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r.Sampling = Sampling{
		BaseIntervalSeconds: s.base.Seconds(),
		MaxSamples:          s.maxSamples,
		Observed:            s.observed,
		Retained:            len(s.points),
		ReadErrors:          s.errors,
		LastError:           s.lastErr,
	}
	r.Points = Points{Start: s.start, End: s.end}
	r.RAM = s.ramUsage()
	r.CPU = s.cpuUsage()
	r.DiskIO = s.diskIO()
	r.MemoryEvents = s.memoryEvents()
	r.OOMKilled = r.MemoryEvents.OOMKill > 0 || r.MemoryEvents.OOMGroupKill > 0
}

// read performs one reading and updates the peaks. Must hold s.mu.
func (s *Sampler) read(now time.Time) (point, cgroup.Stats, bool) {
	if s.src == nil {
		return point{}, cgroup.Stats{}, false
	}
	st, err := s.src.Read()
	if err != nil {
		s.errors++
		s.lastErr = err.Error()
		return point{}, st, false
	}
	s.observed++
	p := s.toRawPoint(now, st)
	s.updatePeaks(st, p)
	if s.first == nil {
		first := p
		s.first = &first
	}
	prev := p
	s.prev = &prev
	return p, st, true
}

// toRawPoint converts a reading into a point and remembers the last controller readings.
func (s *Sampler) toRawPoint(now time.Time, st cgroup.Stats) point {
	p := point{at: now}
	if st.Memory != nil {
		p.hasRAM = true
		p.ram = st.Memory.Current
		p.ws = workingSet(st.Memory)
		s.lastMem = st.Memory
	}
	if st.CPU != nil {
		p.hasCPU = true
		p.cpuUsec = st.CPU.UsageUsec
		s.lastCPU = st.CPU
	}
	if st.IO != nil {
		p.hasIO = true
		p.ioR, p.ioW = st.IO.ReadBytes, st.IO.WriteBytes
	}
	return p
}

func (s *Sampler) updatePeaks(st cgroup.Stats, p point) {
	if st.Memory != nil {
		s.peaks.ramSampled = max(s.peaks.ramSampled, p.ram)
		if st.Memory.HasPeak {
			s.peaks.hasKernel = true
			s.peaks.kernel = max(s.peaks.kernel, st.Memory.Peak)
		}
	}
	if s.prev == nil {
		return
	}
	if r, ok := rateMilli(*s.prev, p); ok {
		s.peaks.cpuMilli = max(s.peaks.cpuMilli, r)
	}
	if r, w, ok := rateIO(*s.prev, p); ok {
		s.peaks.ioReadRate = max(s.peaks.ioReadRate, r)
		s.peaks.ioWriteRate = max(s.peaks.ioWriteRate, w)
	}
}

// downsample halves the series resolution in place once the budget is exhausted. Must hold s.mu.
func (s *Sampler) downsample() {
	for len(s.points) > s.maxSamples {
		kept := make([]point, 0, len(s.points)/2+1)
		for i := 0; i < len(s.points); i += 2 {
			kept = append(kept, s.points[i])
		}
		s.points = kept
		s.interval *= 2
		s.stride *= 2
	}
}

// ramUsage is the peak from the kernel where it has one, sampled otherwise, and the working-set
// series.
func (s *Sampler) ramUsage() Usage {
	u := Usage{Series: s.emptySeries()}
	switch {
	case s.peaks.hasKernel:
		u.Peak, u.PeakSource = s.peaks.kernel, peakSourceKernel
	case s.observed > 0:
		u.Peak, u.PeakSource = s.peaks.ramSampled, peakSourceSampled
	}
	for _, p := range s.points {
		if p.hasRAM {
			u.Series.Values = append(u.Series.Values, p.ws)
		}
	}
	return u
}

// cpuUsage is the series of rates between retained points, the peak from base-interval rates,
// and the cgroup totals.
func (s *Sampler) cpuUsage() CPUUsage {
	u := CPUUsage{Usage: Usage{Peak: s.peaks.cpuMilli, Series: s.emptySeries()}}
	if s.observed > 0 {
		u.PeakSource = peakSourceSampled
	}
	for i := 1; i < len(s.points); i++ {
		if v, ok := rateMilli(s.points[i-1], s.points[i]); ok {
			u.Series.Values = append(u.Series.Values, v)
		}
	}
	if s.first != nil && s.prev != nil && s.first.hasCPU && s.prev.hasCPU && s.prev.cpuUsec >= s.first.cpuUsec {
		u.UsageSeconds = float64(s.prev.cpuUsec-s.first.cpuUsec) / 1e6
	}
	if s.lastCPU != nil {
		u.ThrottledPeriods = s.lastCPU.NrThrottled
		u.ThrottledSeconds = float64(s.lastCPU.ThrottledUsec) / 1e6
	}
	return u
}

// diskIO is nil unless the source produced IO readings.
func (s *Sampler) diskIO() *DiskIO {
	if s.prev == nil || !s.prev.hasIO {
		return nil
	}
	d := &DiskIO{
		Read:  Usage{Peak: s.peaks.ioReadRate, PeakSource: peakSourceSampled, Series: s.emptySeries()},
		Write: Usage{Peak: s.peaks.ioWriteRate, PeakSource: peakSourceSampled, Series: s.emptySeries()},
	}
	for i := 1; i < len(s.points); i++ {
		if rd, wr, ok := rateIO(s.points[i-1], s.points[i]); ok {
			d.Read.Series.Values = append(d.Read.Series.Values, rd)
			d.Write.Series.Values = append(d.Write.Series.Values, wr)
		}
	}
	return d
}

// memoryEvents are the kernel counters as deltas since the run started.
func (s *Sampler) memoryEvents() MemoryEvents {
	var ev MemoryEvents
	if s.lastMem == nil {
		return ev
	}
	base := cgroup.MemoryStats{}
	if s.startEvents != nil {
		base = *s.startEvents
	}
	if s.lastMem.OOMKills >= base.OOMKills {
		ev.OOMKill = s.lastMem.OOMKills - base.OOMKills
	}
	if s.lastMem.OOMGroupKills >= base.OOMGroupKills {
		ev.OOMGroupKill = s.lastMem.OOMGroupKills - base.OOMGroupKills
	}
	return ev
}

func (s *Sampler) emptySeries() Series {
	return Series{IntervalSeconds: s.interval.Seconds(), Values: []uint64{}}
}

// workingSet is memory.current minus the reclaimable file cache, floored at zero.
func workingSet(m *cgroup.MemoryStats) uint64 {
	if m.InactiveFile >= m.Current {
		return 0
	}
	return m.Current - m.InactiveFile
}

// rateMilli is CPU usage between two points in millicores.
func rateMilli(a, b point) (uint64, bool) {
	if !a.hasCPU || !b.hasCPU || !b.at.After(a.at) || b.cpuUsec < a.cpuUsec {
		return 0, false
	}
	dt := b.at.Sub(a.at).Microseconds()
	if dt <= 0 {
		return 0, false
	}
	return (b.cpuUsec - a.cpuUsec) * 1000 / uint64(dt), true
}

// rateIO is IO throughput between two points in bytes per second.
func rateIO(a, b point) (r, w uint64, ok bool) {
	if !a.hasIO || !b.hasIO || !b.at.After(a.at) || b.ioR < a.ioR || b.ioW < a.ioW {
		return 0, 0, false
	}
	dt := b.at.Sub(a.at).Seconds()
	if dt <= 0 {
		return 0, 0, false
	}
	return uint64(float64(b.ioR-a.ioR) / dt), uint64(float64(b.ioW-a.ioW) / dt), true
}

func toPoint(p point) *Point {
	return &Point{
		At:              p.at.UnixMilli(),
		RAM:             p.ram,
		RAMWorkingSet:   p.ws,
		CPUUsageSeconds: float64(p.cpuUsec) / 1e6,
	}
}
