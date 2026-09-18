package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverOptions steer Discover. Zero values mean the real system locations.
type DiscoverOptions struct {
	// Dir forces a cgroup directory: a unified (v2) directory, or a v1 mount root holding
	// `memory/` and `cpu,cpuacct/` (or `cpuacct/`) subdirectories.
	Dir string
	// SysFS is the cgroup filesystem mount point, default /sys/fs/cgroup.
	SysFS string
	// ProcRoot is where /proc is mounted, default /proc. Set to "-" to disable IO accounting.
	ProcRoot string
}

// selfCgroup is the parsed content of /proc/self/cgroup.
type selfCgroup struct {
	v2Path  string
	hasV2   bool
	memPath string // v1 memory controller path
	cpuPath string // v1 cpu / cpuacct controller path
}

// Discover finds the cgroup this process belongs to. It looks at /proc/self/cgroup and tries the
// path it names under SysFS first and SysFS itself second: with a private cgroup namespace
// (Kubernetes, Docker on cgroup v2) the container's own cgroup is mounted at the root, while with
// a host namespace the named path exists under it.
func Discover(opts DiscoverOptions) (Source, error) {
	sysfs := opts.SysFS
	if sysfs == "" {
		sysfs = "/sys/fs/cgroup"
	}
	procRoot := opts.ProcRoot
	switch procRoot {
	case "":
		procRoot = "/proc"
	case "-":
		procRoot = ""
	}

	if opts.Dir != "" {
		return fromDir(opts.Dir, procRoot)
	}

	raw, err := readSelfCgroup(procRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	self := parseSelfCgroup(raw)

	if self.hasV2 {
		if dir := findV2Dir(sysfs, self.v2Path); dir != "" {
			return NewV2(dir, procRoot), nil
		}
	}

	memDir, cpuDir := findV1Dirs(sysfs, self)
	if memDir == "" && cpuDir == "" {
		return nil, fmt.Errorf(
			"%w: no readable cgroup under %s (self: %q)",
			ErrUnavailable,
			sysfs,
			strings.TrimSpace(raw),
		)
	}
	return NewV1(memDir, cpuDir, procRoot), nil
}

// readSelfCgroup reads /proc/self/cgroup, from the real /proc when procRoot is empty.
func readSelfCgroup(procRoot string) (string, error) {
	if procRoot == "" {
		procRoot = "/proc"
	}
	b, err := os.ReadFile(filepath.Join(procRoot, "self", "cgroup"))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseSelfCgroup reads "hierarchy:controllers:path" lines.
func parseSelfCgroup(raw string) selfCgroup {
	var self selfCgroup
	for line := range strings.SplitSeq(raw, "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		controllers, p := parts[1], parts[2]
		if controllers == "" {
			self.v2Path, self.hasV2 = p, true
			continue
		}
		for c := range strings.SplitSeq(controllers, ",") {
			switch c {
			case "memory":
				self.memPath = p
			case "cpu", "cpuacct":
				self.cpuPath = p
			}
		}
	}
	return self
}

func findV2Dir(sysfs, v2Path string) string {
	for _, dir := range []string{filepath.Join(sysfs, v2Path), sysfs} {
		if fileExists(filepath.Join(dir, "memory.current")) || fileExists(filepath.Join(dir, "cpu.stat")) {
			return dir
		}
	}
	return ""
}

func findV1Dirs(sysfs string, self selfCgroup) (memDir, cpuDir string) {
	memDir = firstDir(
		filepath.Join(sysfs, "memory", self.memPath),
		filepath.Join(sysfs, "memory"),
	)
	cpuDir = firstDir(
		filepath.Join(sysfs, "cpu,cpuacct", self.cpuPath),
		filepath.Join(sysfs, "cpuacct", self.cpuPath),
		filepath.Join(sysfs, "cpu", self.cpuPath),
		filepath.Join(sysfs, "cpu,cpuacct"),
		filepath.Join(sysfs, "cpuacct"),
	)
	if memDir != "" && !fileExists(filepath.Join(memDir, "memory.usage_in_bytes")) {
		memDir = ""
	}
	if cpuDir != "" && !fileExists(filepath.Join(cpuDir, "cpuacct.usage")) {
		cpuDir = ""
	}
	return memDir, cpuDir
}

func fromDir(dir, procRoot string) (Source, error) {
	if fileExists(filepath.Join(dir, "memory.current")) || fileExists(filepath.Join(dir, "cpu.stat")) {
		return NewV2(dir, procRoot), nil
	}
	memDir := firstDir(filepath.Join(dir, "memory"))
	cpuDir := firstDir(
		filepath.Join(dir, "cpu,cpuacct"),
		filepath.Join(dir, "cpuacct"),
		filepath.Join(dir, "cpu"),
	)
	if memDir == "" && cpuDir == "" {
		return nil, fmt.Errorf("%w: %s is neither a cgroup v2 directory nor a v1 mount root", ErrUnavailable, dir)
	}
	return NewV1(memDir, cpuDir, procRoot), nil
}

func firstDir(candidates ...string) string {
	for _, c := range candidates {
		if dirExists(c) {
			return c
		}
	}
	return ""
}
