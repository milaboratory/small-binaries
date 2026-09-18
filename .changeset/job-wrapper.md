---
"@platforma-open/milaboratories.software-small-binaries.job-wrapper": minor
"@platforma-open/milaboratories.software-small-binaries": minor
---

Add `job-wrapper`: a static Go entrypoint for job containers that replaces `job-script.sh`(PATH/LD_LIBRARY_PATH prepend, workdir prune, stdout/stderr mirroring, completion marker, exit code) and, while the command runs, samples the container cgroup and publishes CPU/RAM usage, peaks, OOM-kill events, duration and a basic execId to `.pl/usage.json`.
