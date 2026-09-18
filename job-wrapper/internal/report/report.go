// Package report defines the usage report the wrapper writes for the runner (`.pl/usage.json`),
// the sampler that fills it from a cgroup.Source, and the atomic writer that publishes it.
//
// The shape mirrors the Usage / Series / FailDetails types of the resources-sizing core structure
// and is driver-independent on purpose: any runner driver can produce the same file.
package report

import (
	"github.com/milaboratory/small-binaries/job-wrapper/internal/cgroup"
	"github.com/milaboratory/small-binaries/job-wrapper/internal/execid"
)

// Version is the report schema version. Bump on any incompatible change.
const Version = 1

// States of a run as the report sees it.
const (
	StateRunning  = "running"
	StateFinished = "finished"
)

// Report is the whole file.
type Report struct {
	Version int     `json:"version"`
	Wrapper Wrapper `json:"wrapper"`

	// State is "running" while the command runs and "finished" once the final report is written.
	// A file left in "running" belongs to an attempt that died with its container.
	State string `json:"state"`

	StartedAt  int64    `json:"startedAt"`            // epoch milliseconds, just before the command started
	UpdatedAt  int64    `json:"updatedAt"`            // epoch milliseconds of this write
	FinishedAt *int64   `json:"finishedAt,omitempty"` // epoch milliseconds the command exited
	Duration   *float64 `json:"duration,omitempty"`   // seconds of wall clock the command took

	ExitCode *int   `json:"exitCode,omitempty"` // as the wrapper exits: the command's code, 128+N for a signal
	Signal   string `json:"signal,omitempty"`   // the signal that terminated the command, if any
	// TerminationSignal is a signal the wrapper itself received and forwarded (SIGTERM / SIGINT).
	TerminationSignal string `json:"terminationSignal,omitempty"`

	// OOMKilled is true when the kernel OOM killer took a process from the cgroup during the run.
	OOMKilled bool `json:"oomKilled"`

	ExecID string       `json:"execId,omitempty"`
	Exec   *execid.Exec `json:"exec,omitempty"`

	// Granted is what the cgroup grants the run - what usage should be compared against.
	Granted cgroup.Limits `json:"granted"`
	Cgroup  cgroup.Info   `json:"cgroup"`

	CPU    CPUUsage `json:"cpu"`              // millicores
	RAM    Usage    `json:"ram"`              // bytes
	DiskIO *DiskIO  `json:"diskIo,omitempty"` // bytes per second, syscall level

	MemoryEvents MemoryEvents `json:"memoryEvents"`
	Points       Points       `json:"points"`
	Sampling     Sampling     `json:"sampling"`
}

// Wrapper identifies the producer.
type Wrapper struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Usage is one resource's high-water mark and its shape over the run.
type Usage struct {
	Peak uint64 `json:"peak"`
	// PeakSource is "kernel" when Peak is the cgroup's own high-water mark (memory.peak /
	// memory.max_usage_in_bytes) and "sampled" when it is the maximum over samples.
	PeakSource string `json:"peakSource,omitempty"`
	Series     Series `json:"series"`
}

// CPUUsage extends Usage with cgroup CPU totals.
type CPUUsage struct {
	Usage
	UsageSeconds     float64 `json:"usageSeconds"`
	ThrottledSeconds float64 `json:"throttledSeconds"`
	ThrottledPeriods uint64  `json:"throttledPeriods"`
}

// DiskIO is read and write throughput. Approximate by construction: it sums rchar/wchar of the
// cgroup's processes, which counts page-cache hits and misses processes that live shorter than a
// sample.
type DiskIO struct {
	Read  Usage `json:"read"`
	Write Usage `json:"write"`
}

// Series is a sampled time series with one uniform interval.
type Series struct {
	IntervalSeconds float64  `json:"intervalSeconds"`
	Values          []uint64 `json:"values"`
}

// MemoryEvents are the kernel's memory event counters, as deltas since the run started.
type MemoryEvents struct {
	OOMKill      uint64 `json:"oomKill"`
	OOMGroupKill uint64 `json:"oomGroupKill"`
}

// Point is a snapshot at a moment of interest.
type Point struct {
	At              int64   `json:"at"` // epoch milliseconds
	RAM             uint64  `json:"ram"`
	RAMWorkingSet   uint64  `json:"ramWorkingSet"`
	CPUUsageSeconds float64 `json:"cpuUsageSeconds"`
}

// Points are the key points of the run: just before the command started, and right after it ended.
type Points struct {
	Start *Point `json:"start,omitempty"`
	End   *Point `json:"end,omitempty"`
}

// Sampling documents how the series were built.
type Sampling struct {
	BaseIntervalSeconds float64 `json:"baseIntervalSeconds"`
	MaxSamples          int     `json:"maxSamples"`
	Observed            int     `json:"observed"` // raw reads that succeeded
	Retained            int     `json:"retained"` // points kept in the series
	ReadErrors          int     `json:"readErrors"`
	LastError           string  `json:"lastError,omitempty"`
}
