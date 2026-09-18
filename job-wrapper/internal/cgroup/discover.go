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

	b, err := os.ReadFile(filepath.Join(procRoot, "self", "cgroup"))
	if err != nil {
		if procRoot == "" {
			b, err = os.ReadFile("/proc/self/cgroup")
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
	}

	var v2Path, memPath, cpuPath string
	hasV2 := false
	for _, line := range strings.Split(string(b), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		controllers, p := parts[1], parts[2]
		switch {
		case controllers == "":
			v2Path, hasV2 = p, true
		default:
			for _, c := range strings.Split(controllers, ",") {
				switch c {
				case "memory":
					memPath = p
				case "cpu", "cpuacct":
					cpuPath = p
				}
			}
		}
	}

	if hasV2 {
		for _, dir := range []string{filepath.Join(sysfs, v2Path), sysfs} {
			if fileExists(filepath.Join(dir, "memory.current")) || fileExists(filepath.Join(dir, "cpu.stat")) {
				return NewV2(dir, procRoot), nil
			}
		}
	}

	memDir := firstDir(
		filepath.Join(sysfs, "memory", memPath),
		filepath.Join(sysfs, "memory"),
	)
	cpuDir := firstDir(
		filepath.Join(sysfs, "cpu,cpuacct", cpuPath),
		filepath.Join(sysfs, "cpuacct", cpuPath),
		filepath.Join(sysfs, "cpu", cpuPath),
		filepath.Join(sysfs, "cpu,cpuacct"),
		filepath.Join(sysfs, "cpuacct"),
	)
	if memDir != "" && !fileExists(filepath.Join(memDir, "memory.usage_in_bytes")) {
		memDir = ""
	}
	if cpuDir != "" && !fileExists(filepath.Join(cpuDir, "cpuacct.usage")) {
		cpuDir = ""
	}
	if memDir == "" && cpuDir == "" {
		return nil, fmt.Errorf("%w: no readable cgroup under %s (self: %q)", ErrUnavailable, sysfs, strings.TrimSpace(string(b)))
	}
	return NewV1(memDir, cpuDir, procRoot), nil
}

func fromDir(dir, procRoot string) (Source, error) {
	if fileExists(filepath.Join(dir, "memory.current")) || fileExists(filepath.Join(dir, "cpu.stat")) {
		return NewV2(dir, procRoot), nil
	}
	memDir := firstDir(filepath.Join(dir, "memory"))
	cpuDir := firstDir(filepath.Join(dir, "cpu,cpuacct"), filepath.Join(dir, "cpuacct"), filepath.Join(dir, "cpu"))
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
