/**
 * Prints the report of its own command line.
 *
 * The binary distribution of this package runs a fake java run environment
 * (a small Go program) that prints such report instead of the real work.
 * Real java cannot take the command line of the fake run environment, so the
 * docker distribution runs this class with real java and keeps the output
 * byte-compatible with the Go program.
 *
 * Usage: java Report <reported-command-name> [<argument> ...]
 */
public class Report {
    private static final int COLUMN_WIDTH = 8;

    public static void main(String[] args) {
        if (args.length < 1) {
            System.err.println("Report: the name of reported command is required");
            System.exit(1);
        }

        StringBuilder report = new StringBuilder();
        appendLine(report, "cmd", args[0]);
        for (int i = 1; i < args.length; i++) {
            appendLine(report, "arg[" + (i - 1) + "]", args[i]);
        }

        // The Go program prints one more empty line after the report.
        System.out.print(report + "\n");
    }

    private static void appendLine(StringBuilder report, String label, String value) {
        report.append(String.format("%" + COLUMN_WIDTH + "s = %s", label, quote(value)));
        report.append("\n");
    }

    /** Quotes the string in the same way as the Go '%q' format verb does. */
    private static String quote(String value) {
        StringBuilder result = new StringBuilder("\"");
        for (int i = 0; i < value.length(); i++) {
            char c = value.charAt(i);
            switch (c) {
                case '"': result.append("\\\""); break;
                case '\\': result.append("\\\\"); break;
                case 0x07: result.append("\\a"); break;
                case '\b': result.append("\\b"); break;
                case '\f': result.append("\\f"); break;
                case '\n': result.append("\\n"); break;
                case '\r': result.append("\\r"); break;
                case '\t': result.append("\\t"); break;
                case 0x0b: result.append("\\v"); break;
                default:
                    if (c < 0x20 || c == 0x7f) {
                        result.append(String.format("\\x%02x", (int) c));
                    } else {
                        result.append(c);
                    }
            }
        }
        result.append('"');

        return result.toString();
    }
}
