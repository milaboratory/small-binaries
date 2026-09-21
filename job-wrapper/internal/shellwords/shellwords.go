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

// splitter walks one line and accumulates words.
type splitter struct {
	line   string
	words  []string
	cur    strings.Builder
	inWord bool
}

// Split breaks line into words the way a POSIX shell tokenizes them: single quotes are literal,
// double quotes keep everything except \" \\ \$ \` escapes, a backslash outside quotes escapes
// the next byte, and unquoted blanks separate words. Operators (`;`, `|`, `&&`) are not
// interpreted and land inside words.
func Split(line string) ([]string, error) {
	s := &splitter{line: line}
	for i := 0; i < len(line); i++ {
		next, err := s.step(i)
		if err != nil {
			return nil, err
		}
		i = next
	}
	s.flush()
	return s.words, nil
}

// step consumes the token starting at i and returns the index of its last byte.
func (s *splitter) step(i int) (int, error) {
	switch s.line[i] {
	case '\'':
		return s.singleQuoted(i)
	case '"':
		return s.doubleQuoted(i)
	case '\\':
		s.inWord = true
		if i+1 < len(s.line) {
			i++
			if s.line[i] != '\n' {
				s.cur.WriteByte(s.line[i])
			}
		}
		return i, nil
	case ' ', '\t', '\n', '\r':
		s.flush()
		return i, nil
	default:
		s.inWord = true
		s.cur.WriteByte(s.line[i])
		return i, nil
	}
}

func (s *splitter) singleQuoted(i int) (int, error) {
	s.inWord = true
	end := strings.IndexByte(s.line[i+1:], '\'')
	if end < 0 {
		return 0, ErrUnterminatedQuote
	}
	s.cur.WriteString(s.line[i+1 : i+1+end])
	return i + end + 1, nil
}

func (s *splitter) doubleQuoted(i int) (int, error) {
	s.inWord = true
	for i++; i < len(s.line); i++ {
		d := s.line[i]
		if d == '\\' && i+1 < len(s.line) && strings.IndexByte("\"\\$`\n", s.line[i+1]) >= 0 {
			i++
			if s.line[i] != '\n' {
				s.cur.WriteByte(s.line[i])
			}
			continue
		}
		if d == '"' {
			return i, nil
		}
		s.cur.WriteByte(d)
	}
	return 0, ErrUnterminatedQuote
}

func (s *splitter) flush() {
	if s.inWord {
		s.words = append(s.words, s.cur.String())
		s.cur.Reset()
		s.inWord = false
	}
}
