# @platforma-open/milaboratories.software-small-binaries.line-counter

## 1.1.1

### Patch Changes

- 8676a55: Detect the compression extension case-insensitively. A file with an uppercase or
  mixed-case suffix (`.GZ`, `.Zst`, …) is now decompressed before counting instead
  of being read raw and miscounted.

## 1.1.0

### Minor Changes

- 028890d: Add line-counter binary: streams a file (optionally .gz/.bz2/.zst) and writes the exact line count.
