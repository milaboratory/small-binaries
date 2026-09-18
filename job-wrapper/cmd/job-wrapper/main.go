// job-wrapper is the entrypoint of a Platforma job container.
//
// It replaces job-script.sh: it prepares PATH and LD_LIBRARY_PATH, prunes leftovers of an earlier
// attempt from the working directory, mirrors the command's stdout/stderr to files, writes the
// completion marker with the exit code, and exits with that code. New with the wrapper, it samples
// the container cgroup while the command runs and publishes CPU/RAM usage to .pl/usage.json.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/milaboratory/small-binaries/job-wrapper/internal/wrapper"
)

// version is stamped at build time: -ldflags "-X main.version=<package.json version>".
var version = "dev"

func usage(fs *flag.FlagSet) {
	fmt.Fprintf(fs.Output(), `Usage:
  job-wrapper [flags] -- <command> [args...]
  job-wrapper [flags]              (command taken from $%s, run through 'sh -c')

Environment (legacy job-script.sh contract, all optional):
  %-30s shell line with single-quoted arguments; used when no command is given
  %-30s the command's working directory
  %-30s file listing the expected workdir items, one per line; enables the prune
  %-30s mirror the command's stdout / stderr into these files (appending)
  %-30s write the exit code here when the command finishes
  %-30s prepended to PATH (runenv entries win over GPU entries)
  %-30s prepended to PATH / LD_LIBRARY_PATH on GPU nodes

Environment (usage report):
  %-30s report path; default <%s>/%s; "%s" disables
  %-30s rewrite cadence while running (default %s)
  %-30s cgroup sampling cadence (default %s)
  %-30s series budget before halving the resolution (default %d)
  %-30s force the cgroup directory instead of discovering it
  %-30s execId to report instead of the one derived from the command line

Flags:
`,
		wrapper.EnvCmdAndArgs,
		wrapper.EnvCmdAndArgs, wrapper.EnvWorkdir, wrapper.EnvExpectedItemsFile,
		wrapper.EnvStdoutPath+" / "+wrapper.EnvStderrPath, wrapper.EnvMarkerPath, wrapper.EnvJobPath,
		wrapper.EnvGPUBinPath+" / "+wrapper.EnvGPULibPath,
		wrapper.EnvReportPath, wrapper.EnvWorkdir, wrapper.DefaultReportRel, wrapper.ReportDisabled,
		wrapper.EnvFlushInterval, wrapper.DefaultFlushInterval,
		wrapper.EnvSampleInterval, wrapper.DefaultSampleInterval,
		wrapper.EnvMaxSamples, wrapper.DefaultMaxSamples,
		wrapper.EnvCgroupDir, wrapper.EnvExecID,
	)
	fs.PrintDefaults()
}

func main() {
	cfg := wrapper.FromEnv(os.Getenv)
	cfg.Version = version
	cfg.Stdout, cfg.Stderr = os.Stdout, os.Stderr

	fs := flag.NewFlagSet("job-wrapper", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { usage(fs) }
	reportPath := fs.String("report", "", "usage report path (\"none\" disables); overrides $"+wrapper.EnvReportPath)
	flushInterval := fs.Duration("flush-interval", 0, "report rewrite cadence; overrides $"+wrapper.EnvFlushInterval)
	sampleInterval := fs.Duration("sample-interval", 0, "cgroup sampling cadence; overrides $"+wrapper.EnvSampleInterval)
	maxSamples := fs.Int("max-samples", 0, "series budget; overrides $"+wrapper.EnvMaxSamples)
	cgroupDir := fs.String("cgroup-dir", "", "cgroup directory; overrides $"+wrapper.EnvCgroupDir)
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		os.Exit(2)
	}
	if *showVersion {
		fmt.Println(wrapper.Name, version)
		return
	}

	if *reportPath != "" {
		cfg.ReportPath = *reportPath
	}
	if *flushInterval > 0 {
		cfg.FlushInterval = *flushInterval
	}
	if *sampleInterval > 0 {
		cfg.SampleInterval = *sampleInterval
	}
	if *maxSamples >= 2 {
		cfg.MaxSamples = *maxSamples
	}
	if *cgroupDir != "" {
		cfg.CgroupDir = *cgroupDir
	}
	cfg.Argv = fs.Args()
	cfg.ResolveReportPath()

	if len(cfg.Argv) == 0 && cfg.ShellCommand == "" {
		fmt.Fprintf(os.Stderr, "job-wrapper: no command given and $%s is empty\n\n", wrapper.EnvCmdAndArgs)
		usage(fs)
		os.Exit(2)
	}

	os.Exit(wrapper.Run(cfg))
}
