# @platforma-open/milaboratories.software-small-binaries.java-stub

## 1.0.7

### Patch Changes

- 8668eb1: Add a `docker` entrypoint to the python and java stub packages, so tests that
  run them work on runners without local binary execution (Kubernetes).

  The images run real python and real java. They print the same command line
  report as the fake run environments of the binary distribution, byte for byte,
  so the output contract of the consuming tests does not change.

## 1.0.6

### Patch Changes

- cca83eb: trigger docker build

## 1.0.5

### Patch Changes

- 1ce7c87: Split production packages from test utils. Put all test utild under 'software-test-utils' name group

## 1.0.4

### Patch Changes

- 225cc43: Technical release: no CVEs for docker images

## 1.0.3

### Patch Changes

- c99cad1: Republish all packages to upload them to platforma registry for all platforms

## 1.0.2

### Patch Changes

- 8cb1475: Make all subpackages public to make changesets work :(

## 1.0.1

### Patch Changes

- f8fe77c: Switch to new workflow. No functional changes in packages are expected
