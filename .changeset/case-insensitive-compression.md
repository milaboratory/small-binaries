---
"@platforma-open/milaboratories.software-small-binaries.line-counter": patch
---

Detect the compression extension case-insensitively. A file with an uppercase or
mixed-case suffix (`.GZ`, `.Zst`, …) is now decompressed before counting instead
of being read raw and miscounted.
