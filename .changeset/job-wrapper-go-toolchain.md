---
"@platforma-open/milaboratories.software-small-binaries.job-wrapper": patch
---

Move job-wrapper to Go 1.26.6 (`go` directive): the 1.24 standard library carried 19 HIGH CVEs
flagged by the backend image scan, among them the os.Root symlink-following bug (CVE-2026-39822)
that the workdir prune relies on.
