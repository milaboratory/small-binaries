# Repository structure

Each go file = single binary. 
This repo intentionally has no go.mod file as it is designed
for small binary utilities with small implementation.

The binary version is detected from git tag.

Main development flow is:
* Change the code
* Commit
* Create version tag
* Run ./release.sh

# Release

Build new version of all binaries, pack them into archives and upload them to our binary registry
(publically available)

The binary version is detected from git tag.

```bash
./release.sh
```

# Build

## Single binary
```bash
go build "./<go file>"
```

## All binaries for all supported architectures:

```bash
./build.sh
```

## Pack all built binaries into archives

```bash
./pack.sh
```
