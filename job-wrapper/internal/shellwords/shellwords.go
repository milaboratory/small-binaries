// Package shellwords splits a POSIX shell command line into words.
//
// The wrapper never executes the result: the legacy PL_JOB_CMD_AND_ARGS contract hands the line
// to `sh -c` verbatim, exactly as job-script.sh did. The split exists so the wrapper can derive an
// execId (command name and a hash of the arguments) from a line the backend produced with
// single-quoted arguments.
package shellwords

import (
	"errors"
	"strings"
)

// ErrUnterminatedQuote is returned for a line whose quote is never closed.
var ErrUnterminatedQuote = errors.New("unterminated quote")

// Split breaks line into words the way a POSIX shell tokenizes them: single quotes are literal,
// double quotes keep everything except \" \\ \$ \` escapes, a backslash outside quotes escapes
// the next byte, and unquoted blanks separate words. Operators (`;`, `|`, `&&`) are not
// interpreted and land inside words.
func Split(line string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false

	flush := func() {
		if inWord {
			words = append(words, cur.String())
			cur.Reset()
			inWord = false
		}
	}

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch c {
		case '\'':
			inWord = true
			end := strings.IndexByte(line[i+1:], '\'')
			if end < 0 {
				return nil, ErrUnterminatedQuote
			}
			cur.WriteString(line[i+1 : i+1+end])
			i += end + 1

		case '"':
			inWord = true
			closed := false
			for i++; i < len(line); i++ {
				d := line[i]
				if d == '\\' && i+1 < len(line) && strings.IndexByte("\"\\$`\n", line[i+1]) >= 0 {
					i++
					if line[i] != '\n' {
						cur.WriteByte(line[i])
					}
					continue
				}
				if d == '"' {
					closed = true
					break
				}
				cur.WriteByte(d)
			}
			if !closed {
				return nil, ErrUnterminatedQuote
			}

		case '\\':
			inWord = true
			if i+1 < len(line) {
				i++
				if line[i] != '\n' {
					cur.WriteByte(line[i])
				}
			}

		case ' ', '\t', '\n', '\r':
			flush()

		default:
			inWord = true
			cur.WriteByte(c)
		}
	}
	flush()

	return words, nil
}
