# @platforma-open/milaboratories.software-test-utils

## 1.2.0

### Minor Changes

- a58930d: New `shell` entrypoint: a POSIX shell, whichever way the software runs.

  In a container it is busybox's `sh`; as a binary it is the host's own `sh`, with the existing
  `true` artifact standing in as the package payload so there is something real to download.

  It exists so tests can exercise a shell through the ordinary software path — importing an
  entrypoint and letting the runner resolve it — rather than through `exec.builder().cmd("sh")`.
  The two are not equivalent: a software descriptor is what the SDK and the backend actually patch
  and render, so a test built on `cmd()` can pass while every real block breaks.

### Patch Changes

- Updated dependencies [a58930d]
  - @platforma-open/milaboratories.software-test-utils.docker-images@1.2.0

## 1.1.7

### Patch Changes

- 8668eb1: Add a `docker` entrypoint to the python and java stub packages, so tests that
  run them work on runners without local binary execution (Kubernetes).

  The images run real python and real java. They print the same command line
  report as the fake run environments of the binary distribution, byte for byte,
  so the output contract of the consuming tests does not change.

- Updated dependencies [8668eb1]
  - @platforma-open/milaboratories.software-test-utils.python-stub@1.0.7
  - @platforma-open/milaboratories.software-test-utils.java-stub@1.0.7

## 1.1.6

### Patch Changes

- cca83eb: trigger docker build
- Updated dependencies [cca83eb]
  - @platforma-open/milaboratories.software-small-binaries.hello-world@1.1.7
  - @platforma-open/milaboratories.software-small-binaries.hello-world-py@1.0.10
  - @platforma-open/milaboratories.software-test-utils.docker-images@1.1.2
  - @platforma-open/milaboratories.software-test-utils.guided-command@1.1.6
  - @platforma-open/milaboratories.software-test-utils.java-stub@1.0.6
  - @platforma-open/milaboratories.software-test-utils.python-stub@1.0.6
  - @platforma-open/milaboratories.software-test-utils.read-with-sleep@1.1.6
  - @platforma-open/milaboratories.software-test-utils.runenv-java-stub@1.0.8
  - @platforma-open/milaboratories.software-test-utils.runenv-python-stub@1.0.9
  - @platforma-open/milaboratories.software-test-utils.sleep@1.1.6
  - @platforma-open/milaboratories.software-test-utils.small-asset@1.1.7

## 1.1.5

### Patch Changes

- Updated dependencies [b1e1588]
  - @platforma-open/milaboratories.software-test-utils.read-with-sleep@1.1.5
  - @platforma-open/milaboratories.software-test-utils.guided-command@1.1.5
  - @platforma-open/milaboratories.software-test-utils.sleep@1.1.5
  - @platforma-open/milaboratories.software-small-binaries.hello-world@1.1.6

## 1.1.4

### Patch Changes

- Updated dependencies [934b370]
  - @platforma-open/milaboratories.software-test-utils.docker-images@1.1.1

## 1.1.3

### Patch Changes

- 63b68d3: bump test image version

## 1.1.2

### Patch Changes

- Updated dependencies [4409a69]
  - @platforma-open/milaboratories.software-small-binaries.hello-world@1.1.5
  - @platforma-open/milaboratories.software-test-utils.guided-command@1.1.4
  - @platforma-open/milaboratories.software-test-utils.read-with-sleep@1.1.4
  - @platforma-open/milaboratories.software-test-utils.sleep@1.1.4

## 1.1.1

### Patch Changes

- 34c1717: Adjust with check-network tool: hello world is actually prod software
- Updated dependencies [34c1717]
  - @platforma-open/milaboratories.software-test-utils.small-asset@1.1.6
  - @platforma-open/milaboratories.software-test-utils.sleep@1.1.3
  - @platforma-open/milaboratories.software-small-binaries.hello-world-py@1.0.9
  - @platforma-open/milaboratories.software-small-binaries.hello-world@1.1.4

## 1.1.0

### Minor Changes

- c9919c3: Separate docker-only entrypoint

### Patch Changes

- Updated dependencies [c9919c3]
  - @platforma-open/milaboratories.software-test-utils.docker-images@1.1.0
