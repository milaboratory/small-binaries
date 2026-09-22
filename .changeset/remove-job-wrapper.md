---
"@platforma-open/milaboratories.software-small-binaries": minor
---

Remove the `job-wrapper` entrypoint: the job-container entrypoint now lives in the platforma
backend repository (`cmd/job-wrapper`), built and versioned together with the runner that reads
its usage report. The 1.1.x packages published from here stay available but are superseded; the
entrypoint existed for one release and had no consumer in a block.
