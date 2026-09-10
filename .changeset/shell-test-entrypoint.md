---
"@platforma-open/milaboratories.software-test-utils.docker-images": minor
"@platforma-open/milaboratories.software-test-utils": minor
---

New `shell` entrypoint: a POSIX shell, whichever way the software runs.

In a container it is busybox's `sh`; as a binary it is the host's own `sh`, with the existing
`true` artifact standing in as the package payload so there is something real to download.

It exists so tests can exercise a shell through the ordinary software path — importing an
entrypoint and letting the runner resolve it — rather than through `exec.builder().cmd("sh")`.
The two are not equivalent: a software descriptor is what the SDK and the backend actually patch
and render, so a test built on `cmd()` can pass while every real block breaks.
