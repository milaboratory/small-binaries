"""Prints the report of its own command line.

The binary distribution of this package runs a fake python run environment
(a small Go program) that prints such report instead of the real work.
Real python cannot take the command line of the fake run environment, so the
docker distribution runs this script with real python and keeps the output
byte-compatible with the Go program.

Usage: python report.py <reported-command-name> [<argument> ...]
"""

import sys

COLUMN_WIDTH = 8

_SHORT_ESCAPES = {
    '"': '\\"',
    "\\": "\\\\",
    "\a": "\\a",
    "\b": "\\b",
    "\f": "\\f",
    "\n": "\\n",
    "\r": "\\r",
    "\t": "\\t",
    "\v": "\\v",
}


def go_quote(value):
    """Quotes the string in the same way as the Go '%q' format verb does."""

    result = ['"']
    for char in value:
        escape = _SHORT_ESCAPES.get(char)
        if escape is not None:
            result.append(escape)
        elif char.isprintable():
            result.append(char)
        else:
            result.append("\\x%02x" % ord(char))
    result.append('"')

    return "".join(result)


def report_line(label, value):
    return "%*s = %s\n" % (COLUMN_WIDTH, label, go_quote(value))


def main():
    if len(sys.argv) < 2:
        sys.stderr.write("report.py: the name of reported command is required\n")
        return 1

    report = report_line("cmd", sys.argv[1])
    for i, arg in enumerate(sys.argv[2:]):
        report += report_line("arg[%d]" % i, arg)

    # The Go program prints one more empty line after the report.
    sys.stdout.write(report + "\n")

    return 0


if __name__ == "__main__":
    sys.exit(main())
