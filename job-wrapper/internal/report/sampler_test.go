package report

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/milaboratory/small-binaries/job-wrapper/internal/cgroup"
)

// fakeSource replays scripted readings.
type fakeSource struct {
	stats  []cgroup.Stats
	i      int
	limits cgroup.Limits
	err    error
}

func (f *fakeSource) Read() (cgroup.Stats, error) {
	if f.err != nil {
		return cgroup.Stats{}, f.err
	}
	if f.i >= len(f.stats) {
		return f.stats[len(f.stats)-1], nil
	}
	s := f.stats[f.i]
	f.i++
	return s, nil
}
func (f *fakeSource) Limits() cgroup.Limits { return f.limits }
func (f *fakeSource) Info() cgroup.Info {
	return cgroup.Info{Available: true, Version: 2, Path: "/fake"}
}

func mem(cur, inactive uint64) *cgroup.MemoryStats {
	return &cgroup.MemoryStats{Current: cur, InactiveFile: inactive}
}

func TestSamplerPeaksSeriesAndPoints(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	src := &fakeSource{stats: []cgroup.Stats{
		// start
		{Memory: mem(10, 2), CPU: &cgroup.CPUStats{UsageUsec: 0}, IO: &cgroup.IOStats{ReadBytes: 0, WriteBytes: 0}},
		// ticks: 1s apart; CPU 500m then 1000m; RAM climbs to 100 and drops; IO 10 B/s read
		{Memory: mem(50, 5), CPU: &cgroup.CPUStats{UsageUsec: 500_000}, IO: &cgroup.IOStats{ReadBytes: 10}},
		{
			Memory: mem(100, 20),
			CPU:    &cgroup.CPUStats{UsageUsec: 1_500_000},
			IO:     &cgroup.IOStats{ReadBytes: 20, WriteBytes: 5},
		},
		{
			Memory: mem(30, 0),
			CPU:    &cgroup.CPUStats{UsageUsec: 1_600_000, NrThrottled: 2, ThrottledUsec: 250_000},
			IO:     &cgroup.IOStats{ReadBytes: 20, WriteBytes: 5},
		},
		// finish
		{
			Memory: &cgroup.MemoryStats{Current: 20, OOMKills: 1},
			CPU:    &cgroup.CPUStats{UsageUsec: 1_700_000, NrThrottled: 2, ThrottledUsec: 250_000},
			IO:     &cgroup.IOStats{ReadBytes: 20, WriteBytes: 5},
		},
	}}
	s := NewSampler(src, time.Second, 240)
	s.Start(t0)
	for i := 1; i <= 3; i++ {
		s.Tick(t0.Add(time.Duration(i) * time.Second))
	}
	s.Finish(t0.Add(3500 * time.Millisecond))

	var r Report
	s.Fill(&r)

	if r.RAM.Peak != 100 || r.RAM.PeakSource != "sampled" {
		t.Errorf("ram peak: %+v", r.RAM)
	}
	wantWS := []uint64{8, 45, 80, 30}
	if len(r.RAM.Series.Values) != len(wantWS) {
		t.Fatalf("ram series: %v", r.RAM.Series.Values)
	}
	for i, v := range wantWS {
		if r.RAM.Series.Values[i] != v {
			t.Errorf("ram series[%d] = %d, want %d (working set = current - inactive_file)", i, r.RAM.Series.Values[i], v)
		}
	}
	if r.RAM.Series.IntervalSeconds != 1 {
		t.Errorf("interval: %v", r.RAM.Series.IntervalSeconds)
	}
	wantCPU := []uint64{500, 1000, 100}
	if len(r.CPU.Series.Values) != len(wantCPU) {
		t.Fatalf("cpu series: %v", r.CPU.Series.Values)
	}
	for i, v := range wantCPU {
		if r.CPU.Series.Values[i] != v {
			t.Errorf("cpu series[%d] = %d, want %d", i, r.CPU.Series.Values[i], v)
		}
	}
	if r.CPU.Peak != 1000 {
		t.Errorf("cpu peak = %d", r.CPU.Peak)
	}
	if r.CPU.UsageSeconds != 1.7 || r.CPU.ThrottledPeriods != 2 || r.CPU.ThrottledSeconds != 0.25 {
		t.Errorf("cpu totals: %+v", r.CPU)
	}
	if r.DiskIO == nil || r.DiskIO.Read.Peak != 10 || r.DiskIO.Write.Peak != 5 {
		t.Errorf("disk io: %+v", r.DiskIO)
	}
	if r.Points.Start == nil || r.Points.Start.RAM != 10 || r.Points.Start.RAMWorkingSet != 8 ||
		r.Points.Start.At != t0.UnixMilli() {
		t.Errorf("start point: %+v", r.Points.Start)
	}
	if r.Points.End == nil || r.Points.End.RAM != 20 || r.Points.End.CPUUsageSeconds != 1.7 {
		t.Errorf("end point: %+v", r.Points.End)
	}
	if !r.OOMKilled || r.MemoryEvents.OOMKill != 1 {
		t.Errorf("oom: killed=%v events=%+v", r.OOMKilled, r.MemoryEvents)
	}
	if r.Sampling.Observed != 5 || r.Sampling.Retained != 4 || r.Sampling.ReadErrors != 0 {
		t.Errorf("sampling: %+v", r.Sampling)
	}
}

func TestSamplerKernelPeakWins(t *testing.T) {
	src := &fakeSource{stats: []cgroup.Stats{
		{Memory: &cgroup.MemoryStats{Current: 10, Peak: 500, HasPeak: true}},
		{Memory: &cgroup.MemoryStats{Current: 40, Peak: 500, HasPeak: true}},
	}}
	s := NewSampler(src, time.Second, 240)
	t0 := time.Now()
	s.Start(t0)
	s.Tick(t0.Add(time.Second))
	var r Report
	s.Fill(&r)
	if r.RAM.Peak != 500 || r.RAM.PeakSource != "kernel" {
		t.Errorf("ram: %+v", r.RAM)
	}
	if r.DiskIO != nil {
		t.Error("no IO readings must mean no diskIo section")
	}
}

func TestSamplerOOMCountsAreDeltas(t *testing.T) {
	src := &fakeSource{stats: []cgroup.Stats{
		{Memory: &cgroup.MemoryStats{Current: 1, OOMKills: 3}},
		{Memory: &cgroup.MemoryStats{Current: 1, OOMKills: 3}},
	}}
	s := NewSampler(src, time.Second, 240)
	t0 := time.Now()
	s.Start(t0)
	s.Finish(t0.Add(time.Second))
	var r Report
	s.Fill(&r)
	if r.OOMKilled || r.MemoryEvents.OOMKill != 0 {
		t.Errorf("a counter that did not move during the run must not count: %+v", r.MemoryEvents)
	}
}

func TestSamplerDownsamplesToBudget(t *testing.T) {
	const budget = 8
	stats := make([]cgroup.Stats, 0, 200)
	for i := range 200 {
		stats = append(stats, cgroup.Stats{
			Memory: mem(uint64(i), 0),
			CPU:    &cgroup.CPUStats{UsageUsec: uint64(i) * 250_000}, // 250m constant
		})
	}
	src := &fakeSource{stats: stats}
	s := NewSampler(src, time.Second, budget)
	t0 := time.Now()
	s.Start(t0)
	for i := 1; i < 100; i++ {
		s.Tick(t0.Add(time.Duration(i) * time.Second))
	}
	var r Report
	s.Fill(&r)

	if len(r.RAM.Series.Values) > budget {
		t.Fatalf("retained %d > budget %d", len(r.RAM.Series.Values), budget)
	}
	if len(r.RAM.Series.Values) < budget/2 {
		t.Fatalf("retained too few: %d", len(r.RAM.Series.Values))
	}
	// 100 points at 1s -> halved to 2s (50) -> 4s (25) -> 8s (13) -> 16s (7): interval is a power of two.
	if r.RAM.Series.IntervalSeconds != 16 {
		t.Errorf("interval = %v, want 16", r.RAM.Series.IntervalSeconds)
	}
	// Retained points are equidistant: consecutive RAM values (== seconds) differ by the interval.
	for i := 1; i < len(r.RAM.Series.Values); i++ {
		if d := r.RAM.Series.Values[i] - r.RAM.Series.Values[i-1]; d != uint64(r.RAM.Series.IntervalSeconds) {
			t.Errorf("gap between retained points %d is %d, want %v", i, d, r.RAM.Series.IntervalSeconds)
		}
	}
	// The rate is the same at any resolution, and the peak keeps 1s resolution.
	for i, v := range r.CPU.Series.Values {
		if v != 250 {
			t.Errorf("cpu series[%d] = %d, want 250", i, v)
		}
	}
	if r.CPU.Peak != 250 {
		t.Errorf("cpu peak = %d", r.CPU.Peak)
	}
	if r.Sampling.Observed != 100 {
		t.Errorf("observed = %d", r.Sampling.Observed)
	}
}

func TestSamplerToleratesErrorsAndNilSource(t *testing.T) {
	s := NewSampler(nil, time.Second, 240)
	s.Start(time.Now())
	s.Tick(time.Now())
	s.Finish(time.Now())
	var r Report
	s.Fill(&r)
	if r.RAM.Series.Values == nil || len(r.RAM.Series.Values) != 0 || r.Points.Start != nil {
		t.Errorf("nil source must yield empty, non-null series: %+v", r)
	}

	src := &fakeSource{err: errors.New("boom")}
	s = NewSampler(src, time.Second, 240)
	s.Start(time.Now())
	if m := s.Tick(time.Now()); m != nil {
		t.Error("Tick must return nil memory on error")
	}
	s.Fill(&r)
	if r.Sampling.ReadErrors != 2 || r.Sampling.LastError != "boom" {
		t.Errorf("sampling: %+v", r.Sampling)
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pl", "usage.json")
	code := 3
	r := &Report{
		Version:  Version,
		State:    StateFinished,
		ExitCode: &code,
		Wrapper:  Wrapper{Name: "job-wrapper", Version: "test"},
	}

	err := WriteAtomic(path, r)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(path + ".tmp")
	if !os.IsNotExist(err) {
		t.Error("temporary file must not survive the rename")
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateFinished || got.ExitCode == nil || *got.ExitCode != 3 || got.Version != Version {
		t.Errorf("roundtrip: %+v", got)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm()&0o044 == 0 {
		t.Errorf("report must be readable by the runner's uid: %v", st.Mode())
	}

	r.State = StateRunning
	err = WriteAtomic(path, r)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = Read(path)
	if got.State != StateRunning {
		t.Error("rewrite must replace the previous report")
	}
}
