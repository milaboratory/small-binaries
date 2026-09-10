# @platforma-open/milaboratories.software-test-utils.docker-images

## 1.2.0

### Minor Changes

- a58930d: New `shell` entrypoint: a POSIX shell, whichever way the software runs.

  In a container it is busybox's `sh`; as a binary it is the host's own `sh`, with the existing
  `true` artifact standing in as the package payload so there is something real to download.

  It exists so tests can exercise a shell through the ordinary software path — importing an
  entrypoint and letting the runner resolve it — rather than through `exec.builder().cmd("sh")`.
  The two are not equivalent: a software descriptor is what the SDK and the backend actually patch
  and render, so a test built on `cmd()` can pass while every real block breaks.

## 1.1.2

### Patch Changes

- cca83eb: trigger docker build

## 1.1.1

### Patch Changes

- 934b370: trigger docker image rebuild

## 1.1.0

### Minor Changes

- c9919c3: Separate docker-only entrypoint

## 1.0.1

### Patch Changes

- 1ce7c87: Split production packages from test utils. Put all test utild under 'software-test-utils' name group
