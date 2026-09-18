package cgroup

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	err := os.MkdirAll(filepath.Dir(p), 0o750)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(p, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

// fakeProc lays out /proc/<pid>/io for the given pids.
func fakeProc(t *testing.T, root string, io map[int][2]uint64) {
	t.Helper()
	for pid, c := range io {
		write(t, root, filepath.Join(strconv.Itoa(pid), "io"),
			"rchar: "+strconv.FormatUint(c[0], 10)+"\nwchar: "+strconv.FormatUint(c[1], 10)+"\nsyscr: 1\nsyscw: 1\n")
	}
}

func v2Fixture(t *testing.T) (dir, proc string) {
	t.Helper()
	dir = t.TempDir()
	proc = t.TempDir()
	write(t, dir, "cgroup.controllers", "cpu memory io\n")
	write(t, dir, "memory.current", "104857600\n")
	write(t, dir, "memory.peak", "209715200\n")
	write(t, dir, "memory.max", "536870912\n")
	write(t, dir, "memory.stat", "anon 90000000\nfile 14857600\ninactive_file 4857600\nactive_file 10000000\n")
	write(t, dir, "memory.events", "low 0\nhigh 0\nmax 3\noom 1\noom_kill 1\noom_group_kill 0\n")
	write(t, dir, "cpu.stat",
		"usage_usec 2500000\nuser_usec 2000000\nsystem_usec 500000\nnr_periods 30\nnr_throttled 2\nthrottled_usec 150000\n")
	write(t, dir, "cpu.max", "200000 100000\n")
	write(t, dir, "cgroup.procs", "1\n42\n43\n")
	fakeProc(t, proc, map[int][2]uint64{1: {5, 5}, 42: {1000, 200}, 43: {300, 100}})
	return dir, proc
}

func TestV2Read(t *testing.T) {
	dir, proc := v2Fixture(t)
	src := NewV2(dir, proc)

	st, err := src.Read()
	if err != nil {
		t.Fatal(err)
	}
	if st.Memory == nil || st.CPU == nil || st.IO == nil {
		t.Fatalf("missing sections: %+v", st)
	}
	if st.Memory.Current != 104857600 || !st.Memory.HasPeak || st.Memory.Peak != 209715200 {
		t.Errorf("memory: %+v", *st.Memory)
	}
	if st.Memory.InactiveFile != 4857600 || st.Memory.OOMKills != 1 || st.Memory.OOMGroupKills != 0 {
		t.Errorf("memory details: %+v", *st.Memory)
	}
	if st.CPU.UsageUsec != 2500000 || st.CPU.NrThrottled != 2 || st.CPU.ThrottledUsec != 150000 {
		t.Errorf("cpu: %+v", *st.CPU)
	}
	if st.IO.ReadBytes != 1305 || st.IO.WriteBytes != 305 {
		t.Errorf("io: %+v (pids 1, 42, 43 - the reader's own pid is excluded only when it matches os.Getpid)", *st.IO)
	}

	lim := src.Limits()
	if lim.MemoryBytes != 536870912 || lim.CPUMillicores != 2000 {
		t.Errorf("limits: %+v", lim)
	}
	if info := src.Info(); !info.Available || info.Version != 2 || info.Path != dir {
		t.Errorf("info: %+v", info)
	}
}

func TestV2NoPeakNoLimits(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "memory.current", "1000\n")
	write(t, dir, "memory.max", "max\n")
	write(t, dir, "cpu.max", "max 100000\n")
	write(t, dir, "cpu.stat", "usage_usec 10\n")

	src := NewV2(dir, "")
	st, err := src.Read()
	if err != nil {
		t.Fatal(err)
	}
	if st.Memory.HasPeak {
		t.Error("no memory.peak file must mean HasPeak=false")
	}
	if st.IO != nil {
		t.Error("IO accounting must be off without a proc root")
	}
	if lim := src.Limits(); lim != (Limits{}) {
		t.Errorf("limits must be zero: %+v", lim)
	}
}

func TestV2IOTrackerSurvivesProcessExit(t *testing.T) {
	dir := t.TempDir()
	proc := t.TempDir()
	write(t, dir, "memory.current", "1\n")
	write(t, dir, "cgroup.procs", "42\n43\n")
	fakeProc(t, proc, map[int][2]uint64{42: {100, 10}, 43: {50, 5}})
	src := NewV2(dir, proc)

	st, _ := src.Read()
	if st.IO.ReadBytes != 150 || st.IO.WriteBytes != 15 {
		t.Fatalf("first read: %+v", *st.IO)
	}

	// 43 exits, 42 keeps reading, 44 appears.
	write(t, dir, "cgroup.procs", "42\n44\n")
	err := os.RemoveAll(filepath.Join(proc, "43"))
	if err != nil {
		t.Fatal(err)
	}
	fakeProc(t, proc, map[int][2]uint64{42: {200, 10}, 44: {7, 7}})

	st, _ = src.Read()
	if st.IO.ReadBytes != 257 || st.IO.WriteBytes != 22 {
		t.Fatalf("after exit the total must be monotonic: %+v", *st.IO)
	}
}

func TestV1Read(t *testing.T) {
	root := t.TempDir()
	mem := filepath.Join(root, "memory")
	cpu := filepath.Join(root, "cpu,cpuacct")
	write(t, mem, "memory.usage_in_bytes", "3000\n")
	write(t, mem, "memory.max_usage_in_bytes", "5000\n")
	write(t, mem, "memory.limit_in_bytes", "9223372036854771712\n")
	write(t, mem, "memory.stat", "cache 100\ntotal_inactive_file 40\n")
	write(t, mem, "memory.oom_control", "oom_kill_disable 0\nunder_oom 0\noom_kill 2\n")
	write(t, mem, "cgroup.procs", "")
	write(t, cpu, "cpuacct.usage", "4000000000\n")
	write(t, cpu, "cpu.stat", "nr_periods 10\nnr_throttled 3\nthrottled_time 9000000\n")
	write(t, cpu, "cpu.cfs_quota_us", "-1\n")
	write(t, cpu, "cpu.cfs_period_us", "100000\n")

	src, err := Discover(DiscoverOptions{Dir: root, ProcRoot: "-"})
	if err != nil {
		t.Fatal(err)
	}
	st, err := src.Read()
	if err != nil {
		t.Fatal(err)
	}
	if st.Memory.Current != 3000 || st.Memory.Peak != 5000 || !st.Memory.HasPeak ||
		st.Memory.InactiveFile != 40 || st.Memory.OOMKills != 2 {
		t.Errorf("memory: %+v", *st.Memory)
	}
	if st.CPU.UsageUsec != 4000000 || st.CPU.NrThrottled != 3 || st.CPU.ThrottledUsec != 9000 {
		t.Errorf("cpu: %+v", *st.CPU)
	}
	if lim := src.Limits(); lim != (Limits{}) {
		t.Errorf("unlimited v1 cgroup must report zero limits: %+v", lim)
	}
	if info := src.Info(); info.Version != 1 || info.Path != mem || info.CPUPath != cpu {
		t.Errorf("info: %+v", info)
	}

	write(t, mem, "memory.limit_in_bytes", "268435456\n")
	write(t, cpu, "cpu.cfs_quota_us", "50000\n")
	if lim := src.Limits(); lim.MemoryBytes != 268435456 || lim.CPUMillicores != 500 {
		t.Errorf("limits: %+v", lim)
	}
}

func TestDiscoverV2PrivateNamespace(t *testing.T) {
	sysfs, proc := v2Fixture(t)
	// A private cgroup namespace reports "0::/" and the cgroup is mounted at the root.
	write(t, proc, "self/cgroup", "0::/\n")

	src, err := Discover(DiscoverOptions{SysFS: sysfs, ProcRoot: proc})
	if err != nil {
		t.Fatal(err)
	}
	if info := src.Info(); info.Version != 2 || filepath.Clean(info.Path) != filepath.Clean(sysfs) {
		t.Errorf("info: %+v", info)
	}
}

func TestDiscoverV2HostNamespace(t *testing.T) {
	sysfs := t.TempDir()
	proc := t.TempDir()
	inner := filepath.Join(sysfs, "kubepods", "pod1", "ctr1")
	write(t, inner, "memory.current", "1\n")
	write(t, proc, "self/cgroup", "0::/kubepods/pod1/ctr1\n")

	src, err := Discover(DiscoverOptions{SysFS: sysfs, ProcRoot: proc})
	if err != nil {
		t.Fatal(err)
	}
	if info := src.Info(); info.Version != 2 || info.Path != inner {
		t.Errorf("info: %+v", info)
	}
}

func TestDiscoverV1(t *testing.T) {
	sysfs := t.TempDir()
	proc := t.TempDir()
	write(t, filepath.Join(sysfs, "memory", "docker", "abc"), "memory.usage_in_bytes", "1\n")
	write(t, filepath.Join(sysfs, "cpu,cpuacct", "docker", "abc"), "cpuacct.usage", "1\n")
	write(t, proc, "self/cgroup", "12:memory:/docker/abc\n11:cpu,cpuacct:/docker/abc\n1:name=systemd:/docker/abc\n")

	src, err := Discover(DiscoverOptions{SysFS: sysfs, ProcRoot: proc})
	if err != nil {
		t.Fatal(err)
	}
	info := src.Info()
	wantMem := filepath.Join(sysfs, "memory", "docker", "abc")
	wantCPU := filepath.Join(sysfs, "cpu,cpuacct", "docker", "abc")
	if info.Version != 1 || info.Path != wantMem || info.CPUPath != wantCPU {
		t.Errorf("info: %+v", info)
	}
}

func TestDiscoverUnavailable(t *testing.T) {
	sysfs := t.TempDir()
	proc := t.TempDir()
	write(t, proc, "self/cgroup", "0::/\n")
	_, err := Discover(DiscoverOptions{SysFS: sysfs, ProcRoot: proc})
	if err == nil {
		t.Fatal("expected an error when no cgroup files exist")
	}
	_, err = Discover(DiscoverOptions{SysFS: sysfs, ProcRoot: t.TempDir()})
	if err == nil {
		t.Fatal("expected an error when /proc/self/cgroup is missing")
	}
}

func TestParseCPUQuota(t *testing.T) {
	cases := map[string]uint64{
		"max 100000":    0,
		"100000 100000": 1000,
		"50000 100000":  500,
		"250000 100000": 2500,
		"garbage":       0,
		"1 0":           0,
	}
	for in, want := range cases {
		if got := parseCPUQuota(in); got != want {
			t.Errorf("parseCPUQuota(%q) = %d, want %d", in, got, want)
		}
	}
}
