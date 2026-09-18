// Package wrapper is the job entrypoint: it prepares the working directory and environment the way
// job-script.sh did, runs the command, mirrors its streams to files, writes the completion marker,
// and - new - measures the command's consumption from the container cgroup and publishes it to
// `.pl/usage.json` while the command runs.
package wrapper

import (
	"io"
	"strconv"
	"time"

	"github.com/milaboratory/small-binaries/job-wrapper/internal/report"
)

// Environment variables of the legacy job-script.sh contract. Set by the job template.
const (
	EnvCmdAndArgs        = "PL_JOB_CMD_AND_ARGS"
	EnvWorkdir           = "PL_JOB_WORKDIR"
	EnvStdoutPath        = "PL_JOB_STDOUT_PATH"
	EnvStderrPath        = "PL_JOB_STDERR_PATH"
	EnvExpectedItemsFile = "PL_JOB_EXPECTED_ITEMS_FILE"
	EnvMarkerPath        = "PL_JOB_COMPLETION_MARKER_PATH"
	EnvJobPath           = "PL_JOB_PATH"
	EnvGPUBinPath        = "PL_GPU_BIN_PATH"
	EnvGPULibPath        = "PL_GPU_LIB_PATH"
)

// Environment variables the wrapper adds. All optional.
const (
	// EnvReportPath is where the usage report goes. Default: <PL_JOB_WORKDIR>/.pl/usage.json;
	// no report when PL_JOB_WORKDIR is unset. The value "none" disables the report.
	EnvReportPath = "PL_JOB_USAGE_REPORT_PATH"
	// EnvFlushInterval is how often the report is rewritten while the command runs (Go duration).
	EnvFlushInterval = "PL_JOB_USAGE_FLUSH_INTERVAL"
	// EnvSampleInterval is the cgroup sampling cadence (Go duration).
	EnvSampleInterval = "PL_JOB_USAGE_SAMPLE_INTERVAL"
	// EnvMaxSamples is the series budget before the resolution is halved.
	EnvMaxSamples = "PL_JOB_USAGE_MAX_SAMPLES"
	// EnvCgroupDir forces the cgroup directory instead of discovering it.
	EnvCgroupDir = "PL_JOB_CGROUP_DIR"
	// EnvExecID is an execId computed upstream; when set it replaces the one derived from argv.
	EnvExecID = "PL_JOB_EXEC_ID"
)

// Service paths relative to the working directory. `.pl` is the directory Platforma reserves for
// its own files inside a working directory.
const (
	ServiceDirRel    = ".pl"
	TmpDirRel        = ".pl/tmp"
	DefaultReportRel = ".pl/usage.json"
)

// ReportDisabled is the report path value that turns the report off.
const ReportDisabled = "none"

// Defaults for the sampling knobs.
const (
	DefaultFlushInterval  = 15 * time.Second
	DefaultSampleInterval = report.DefaultBaseInterval
	DefaultMaxSamples     = report.DefaultMaxSamples
)

// Config is everything Run needs.
type Config struct {
	// Argv is the command to exec directly. When empty, ShellCommand is run through `sh -c`.
	Argv []string
	// ShellCommand is PL_JOB_CMD_AND_ARGS: a shell line with every argument single-quoted.
	ShellCommand string

	Workdir           string
	ExpectedItemsFile string
	StdoutPath        string
	StderrPath        string
	MarkerPath        string

	JobPath    string
	GPUBinPath string
	GPULibPath string

	// ReportPath is the resolved report location; empty disables the report.
	ReportPath     string
	FlushInterval  time.Duration
	SampleInterval time.Duration
	MaxSamples     int
	CgroupDir      string
	ExecIDOverride string

	// Version is stamped into the report.
	Version string

	// Stdout and Stderr are the wrapper's real streams. *os.File values are handed to the command
	// directly where no redirection is asked for.
	Stdout io.Writer
	Stderr io.Writer
}

// FromEnv reads the legacy contract and the wrapper's own knobs from getenv.
func FromEnv(getenv func(string) string) Config {
	cfg := Config{
		ShellCommand:      getenv(EnvCmdAndArgs),
		Workdir:           getenv(EnvWorkdir),
		ExpectedItemsFile: getenv(EnvExpectedItemsFile),
		StdoutPath:        getenv(EnvStdoutPath),
		StderrPath:        getenv(EnvStderrPath),
		MarkerPath:        getenv(EnvMarkerPath),
		JobPath:           getenv(EnvJobPath),
		GPUBinPath:        getenv(EnvGPUBinPath),
		GPULibPath:        getenv(EnvGPULibPath),
		ReportPath:        getenv(EnvReportPath),
		CgroupDir:         getenv(EnvCgroupDir),
		ExecIDOverride:    getenv(EnvExecID),
		FlushInterval:     DefaultFlushInterval,
		SampleInterval:    DefaultSampleInterval,
		MaxSamples:        DefaultMaxSamples,
	}
	if d, err := time.ParseDuration(getenv(EnvFlushInterval)); err == nil && d > 0 {
		cfg.FlushInterval = d
	}
	if d, err := time.ParseDuration(getenv(EnvSampleInterval)); err == nil && d > 0 {
		cfg.SampleInterval = d
	}
	if n, err := strconv.Atoi(getenv(EnvMaxSamples)); err == nil && n >= 2 {
		cfg.MaxSamples = n
	}
	return cfg
}

// ResolveReportPath applies the default and the "none" switch to ReportPath.
func (c *Config) ResolveReportPath() {
	switch {
	case c.ReportPath == ReportDisabled:
		c.ReportPath = ""
	case c.ReportPath == "" && c.Workdir != "":
		c.ReportPath = c.Workdir + "/" + DefaultReportRel
	}
}
