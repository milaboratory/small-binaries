# job-wrapper

Entrypoint of a Platforma job container. A static Go binary that replaces the shell
`job-script.sh` the Kubernetes job template mounts today, keeps every one of its behaviours,
and adds what the resources-sizing work needs: **CPU and RAM consumption measured from inside
the container**, published to a file the runner can read while the command is still running.

Spec: `docs/text/work/projects/resources-sizing/017-pod-metrics-collection.md`.

## Usage

```
job-wrapper [flags] -- <command> [args...]      # exec the command directly
job-wrapper [flags]                             # command from $PL_JOB_CMD_AND_ARGS, via `sh -c`
```

The second form is the drop-in replacement for `job-script.sh`: point the job template's
`command` at the binary and leave every environment variable as it is.

## What it does, in order

1. Prepends `PL_GPU_BIN_PATH` to `PATH` and `PL_GPU_LIB_PATH` to `LD_LIBRARY_PATH`, then
   `PL_JOB_PATH` to `PATH` so runenv entries win over GPU entries.
2. Prunes the working directory when `PL_JOB_EXPECTED_ITEMS_FILE` names a non-empty list:
   files not in the list are removed, directories not in the list and sheltering no listed
   item are removed, `.pl/tmp` is never walked. Same messages as before
   (`[job-script] Pruning unexpected file: …`). A missing or empty list disables the prune.
3. Empties `<workdir>/.pl/tmp` so the command starts on a clean temporary directory.
4. Mirrors stdout / stderr to `PL_JOB_STDOUT_PATH` / `PL_JOB_STDERR_PATH` (appending) while
   still writing them to the container's own streams. Both pointing at one file share one
   stream and, as before, land on the wrapper's stderr.
5. Runs the command, sampling the cgroup every second and rewriting the usage report every
   15 s (see below).
6. Writes the exit code to `PL_JOB_COMPLETION_MARKER_PATH` (creating `.pl` if needed). The
   marker is written for every exit code and is absent only when the wrapper itself was
   killed, which is what the runner reads as an OOM kill today.
7. Prints `[job-script] Process exited with code N` (and the out-of-memory hint for 137) to
   the command's stderr sink, waits for the mirrored streams to drain, exits with `N`.
   A command killed by a signal exits `128+signal`, as `sh` reports it.

Differences from the shell script, all deliberate:

- **No `awk` dependency.** The prune always runs; there is no "awk not found" downgrade.
- **Signals are forwarded.** `SIGTERM` / `SIGINT` received by the wrapper go to the
  command's process group, the report is flushed, and the wrapper waits for the command.
  The shell script died and left the command to be `SIGKILL`ed at the end of the grace period.
- **Orphans are reaped.** The wrapper is PID 1 in the container and reaps grandchildren that
  outlive their parent, so no zombies accumulate over a long run.
- **A log file that cannot be opened fails the job** with exit code 1 instead of hanging on
  a FIFO with no reader.

## Usage report

Written atomically (temp file + rename) to `<PL_JOB_WORKDIR>/.pl/usage.json`; nowhere when
`PL_JOB_WORKDIR` is unset. Rewritten right after start, every `--flush-interval` (15 s),
once when RAM first reaches 95 % of the cgroup limit, on a forwarded signal, and once more
after the command exits with `"state": "finished"`. A file left in `"running"` belongs to an
attempt that died with its container; its last flush is what survives a hard kill.

```jsonc
{
  "version": 1,
  "wrapper": { "name": "job-wrapper", "version": "1.0.0" },
  "state": "finished",                 // "running" | "finished"
  "startedAt": 1758200000000,          // epoch ms, just before the command started
  "updatedAt": 1758200003210,
  "finishedAt": 1758200003200,
  "duration": 3.2,                     // seconds of wall clock the command took
  "exitCode": 137,
  "signal": "SIGKILL",                 // when the command was terminated by a signal
  "terminationSignal": "SIGTERM",      // when the wrapper received and forwarded one
  "oomKilled": true,                   // memory.events oom_kill moved during the run
  "execId": "mixcr:3f9a0c1d2e4b",      // <basename(argv[0])>:<sha256(args)[:12]>; $PL_JOB_EXEC_ID wins
  "exec": { "command": "mixcr", "argsHash": "3f9a0c1d2e4b", "argCount": 5, "mode": "argv" },
  "granted": { "cpu": 4000, "ram": 17179869184 },   // cgroup limits: millicores, bytes
  "cgroup": { "available": true, "version": 2, "path": "/sys/fs/cgroup" },
  "cpu": {                             // millicores
    "peak": 3980, "peakSource": "sampled",
    "series": { "intervalSeconds": 1, "values": [120, 3950, 3980] },
    "usageSeconds": 8.05, "throttledSeconds": 0.2, "throttledPeriods": 4
  },
  "ram": {                             // bytes; series = working set (memory.current - inactive_file)
    "peak": 17179865088, "peakSource": "kernel",   // memory.peak; "sampled" where the kernel has none
    "series": { "intervalSeconds": 1, "values": [8000000, 900000000, 17000000000] }
  },
  "diskIo": {                          // bytes/s at the syscall boundary (rchar/wchar), approximate
    "read":  { "peak": 52428800, "peakSource": "sampled", "series": { "intervalSeconds": 1, "values": [] } },
    "write": { "peak": 1048576,  "peakSource": "sampled", "series": { "intervalSeconds": 1, "values": [] } }
  },
  "memoryEvents": { "oomKill": 1, "oomGroupKill": 0 },
  "points": {                          // key points: just before start, right after exit
    "start": { "at": 1758200000000, "ram": 8000000, "ramWorkingSet": 7900000, "cpuUsageSeconds": 0.01 },
    "end":   { "at": 1758200003200, "ram": 12000000, "ramWorkingSet": 11000000, "cpuUsageSeconds": 8.06 }
  },
  "sampling": { "baseIntervalSeconds": 1, "maxSamples": 240, "observed": 4, "retained": 4, "readErrors": 0 }
}
```

Series keep **one uniform interval**: samples are taken every second; once more than
`--max-samples` (240) points are retained, every second point is dropped and the interval
doubles, in place, as often as needed. Peaks keep one-second resolution regardless, and the
RAM peak comes from the kernel's own high-water mark where available. `cpu.series` has one
value fewer than `ram.series`: a rate needs two points.

Where numbers come from (cgroup v2, with the v1 fallback):

| Number | Source |
|--------|--------|
| RAM peak | `memory.peak` (`memory.max_usage_in_bytes`), else max of sampled `memory.current` |
| RAM series | `memory.current` − `memory.stat:inactive_file` |
| CPU | `cpu.stat:usage_usec` deltas → millicores; `nr_throttled`, `throttled_usec` (`cpuacct.usage`, `cpu.stat`) |
| OOM | `memory.events:oom_kill`, `oom_group_kill` (`memory.oom_control:oom_kill`) |
| Granted | `memory.max`, `cpu.max` (`memory.limit_in_bytes`, `cpu.cfs_quota_us` / `cpu.cfs_period_us`) |
| Disk IO | Σ `rchar` / `wchar` of `/proc/<pid>/io` over `cgroup.procs`, monotonic across exits |
| Duration, exit code | the wrapper's own clock and `wait4` |

The cgroup is found through `/proc/self/cgroup`: the named path under `/sys/fs/cgroup`
first (host cgroup namespace), then `/sys/fs/cgroup` itself (private namespace, the
Kubernetes and Docker-on-v2 case). Off Linux, or where nothing is readable, the report still
carries the outcome, duration and execId with `"cgroup": {"available": false, "error": …}`.

Not measured yet: VRAM (needs `nvidia-smi` on a GPU node), workdir and scratch occupancy.

## Configuration

| Flag | Environment | Default | Meaning |
|------|-------------|---------|---------|
| `--report PATH` | `PL_JOB_USAGE_REPORT_PATH` | `<PL_JOB_WORKDIR>/.pl/usage.json` | report location; `none` disables |
| `--flush-interval D` | `PL_JOB_USAGE_FLUSH_INTERVAL` | `15s` | rewrite cadence while running |
| `--sample-interval D` | `PL_JOB_USAGE_SAMPLE_INTERVAL` | `1s` | cgroup sampling cadence |
| `--max-samples N` | `PL_JOB_USAGE_MAX_SAMPLES` | `240` | series budget before halving |
| `--cgroup-dir DIR` | `PL_JOB_CGROUP_DIR` | discovered | force the cgroup directory |
| | `PL_JOB_EXEC_ID` | derived | execId to report instead of the derived one |

Legacy variables (`PL_JOB_CMD_AND_ARGS`, `PL_JOB_WORKDIR`, `PL_JOB_EXPECTED_ITEMS_FILE`,
`PL_JOB_STDOUT_PATH`, `PL_JOB_STDERR_PATH`, `PL_JOB_COMPLETION_MARKER_PATH`, `PL_JOB_PATH`,
`PL_GPU_BIN_PATH`, `PL_GPU_LIB_PATH`) keep their `job-script.sh` meaning.

## Wiring it into the runner

The minimal change is in the job template: `command: ["<path>/job-wrapper"]` with no `args`,
everything else untouched. The runner then reads `.pl/usage.json` next to `.pl/completed`.
If the binary is delivered by hardlinking it into `<workdir>/.pl/` (the spec's route), it
must be added to the expected-items list or the prune removes it before it runs.

## Development

```
go test ./...                 # unit + host integration tests (Docker suite skipped)
./test.sh                     # everything, Docker suite included (mandatory unless SKIP_DOCKER_TESTS=1)
JOB_WRAPPER_DOCKER=1 go test ./tests/ -run TestDocker -v
pnpm lint                     # golangci-lint over the whole module (needs golangci-lint v2 on PATH)
```

Linting uses `.golangci.yaml`, adapted from `core/pl`: the same linter set, but the whole module
is linted on every run rather than only changed files, and the pl-specific rules (`mierr`
enforcement, replace-directive allow list, path exclusions) are dropped. Deliberate deviations
are documented in the config: 0755/0644 service files, and no G304/G703 since every path comes
from the job template or cgroup discovery.

The Docker suite runs the Linux build as PID 1 of a `busybox` container under `--memory` /
`--cpus` limits and checks peaks, series, granted limits, the online report, an OOM kill
(`exitCode 137`, `oomKilled true`, marker `137`), the legacy contract, and zombie reaping.
Layout: `cmd/job-wrapper` (CLI), `internal/wrapper` (orchestration), `internal/prune`,
`internal/cgroup`, `internal/report`, `internal/execid`, `internal/shellwords`, `tests/`.
