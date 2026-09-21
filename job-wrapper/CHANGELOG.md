# @platforma-open/milaboratories.software-small-binaries.job-wrapper

## 1.1.1

### Patch Changes

- 40b55bd: Move job-wrapper to Go 1.26.6 (`go` directive): the 1.24 standard library carried 19 HIGH CVEs
  flagged by the backend image scan, among them the os.Root symlink-following bug (CVE-2026-39822)
  that the workdir prune relies on.

## 1.1.0

### Minor Changes

- 22018cf: Add `job-wrapper`: a static Go entrypoint for job containers that replaces `job-script.sh`(PATH/LD_LIBRARY_PATH prepend, workdir prune, stdout/stderr mirroring, completion marker, exit code) and, while the command runs, samples the container cgroup and publishes CPU/RAM usage, peaks, OOM-kill events, duration and a basic execId to `.pl/usage.json`.
