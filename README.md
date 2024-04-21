# Release

Build new version of all binaries, pack them into archives and upload them to our binary registry
(publically available)

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
