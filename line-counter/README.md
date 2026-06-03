# How to use

```
Usage:
	line-counter --input <file> --output <file>

  -input string
    	input file (optionally .gz/.bz2/.zst)
  -output string
    	output file for the line count
```

Counts the number of lines (`'\n'` bytes) in `--input` and writes the exact
count as a base-10 integer (no trailing newline) into `--output`. Compression
is inferred from the input file extension: `.gz`, `.bz2`, `.zst`, otherwise the
file is read as-is. The file is streamed, so memory usage is O(1) regardless of
file size.
