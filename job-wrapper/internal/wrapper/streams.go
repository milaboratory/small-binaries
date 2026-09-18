package wrapper

import (
	"io"
	"os"
	"sync"
)

// fanout writes to every destination, ignoring per-destination errors: a failing log file must
// not stall the pipe the command writes to. Writes are serialized so lines from the command and
// the wrapper's own messages never interleave mid-line.
type fanout struct {
	mu sync.Mutex
	ws []io.Writer
}

func (f *fanout) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range f.ws {
		_, _ = w.Write(p)
	}
	return len(p), nil
}

// streams is the command's stdout/stderr plumbing.
//
// Where no redirection is asked for and the wrapper's own stream is a real file descriptor, the
// command inherits it directly, the way `sh -c` inherited the script's descriptors. Where a stream
// is mirrored to a file, the command writes to a pipe and a goroutine fans every byte out to the
// file and the real stream - the `tee` of the shell version. Both streams pointing at the same
// file share one pipe, so their interleaving is preserved and, as before, everything lands on the
// wrapper's real stderr.
type streams struct {
	childStdout *os.File
	childStderr *os.File
	// wrapperStderr is where the wrapper's post-run messages go: the command's stderr sink, so
	// they reach the stderr file as they did when the script itself was redirected.
	wrapperStderr io.Writer

	wg       sync.WaitGroup
	closeOwn []*os.File // write ends we must close once the command started
	files    []*os.File // log files, closed after the copies drain
}

func openStreams(cfg Config, realOut, realErr io.Writer) (*streams, error) {
	s := &streams{}

	openLog := func(path string) (*os.File, error) {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
		if err != nil {
			return nil, err
		}
		s.files = append(s.files, f)
		return f, nil
	}

	switch {
	case cfg.StderrPath != "" && cfg.StdoutPath == cfg.StderrPath:
		// One file, one pipe, both streams; the script did `exec 1>&2` here.
		f, err := openLog(cfg.StderrPath)
		if err != nil {
			return nil, err
		}
		sink := &fanout{ws: []io.Writer{f, realErr}}
		w, err := s.pipeTo(sink)
		if err != nil {
			return nil, err
		}
		s.childStdout, s.childStderr, s.wrapperStderr = w, w, sink
		return s, nil

	default:
		var err error
		if cfg.StderrPath != "" {
			f, err := openLog(cfg.StderrPath)
			if err != nil {
				return nil, err
			}
			sink := &fanout{ws: []io.Writer{f, realErr}}
			if s.childStderr, err = s.pipeTo(sink); err != nil {
				return nil, err
			}
			s.wrapperStderr = sink
		} else {
			if s.childStderr, err = s.direct(realErr); err != nil {
				return nil, err
			}
			s.wrapperStderr = &fanout{ws: []io.Writer{realErr}}
		}

		if cfg.StdoutPath != "" {
			f, err := openLog(cfg.StdoutPath)
			if err != nil {
				return nil, err
			}
			if s.childStdout, err = s.pipeTo(&fanout{ws: []io.Writer{f, realOut}}); err != nil {
				return nil, err
			}
		} else {
			if s.childStdout, err = s.direct(realOut); err != nil {
				return nil, err
			}
		}
		return s, nil
	}
}

// direct hands w to the command as is when it is a file descriptor, and through a pipe otherwise.
// The command only ever gets *os.File ends: the wrapper reaps the command itself and never calls
// exec.Cmd.Wait, which is what would drain the pipes os/exec creates for plain io.Writers.
func (s *streams) direct(w io.Writer) (*os.File, error) {
	if f, ok := w.(*os.File); ok {
		return f, nil
	}
	return s.pipeTo(w)
}

func (s *streams) pipeTo(dst io.Writer) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.closeOwn = append(s.closeOwn, w)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		_, _ = io.Copy(dst, r)
		_ = r.Close()
	}()
	return w, nil
}

// started releases the wrapper's copies of the pipe write ends, so the readers see EOF once the
// command and everything that inherited its descriptors have exited.
func (s *streams) started() {
	for _, w := range s.closeOwn {
		_ = w.Close()
	}
	s.closeOwn = nil
}

// drain waits for the copies to reach EOF and closes the log files: the `wait` for the tees.
func (s *streams) drain() {
	s.started()
	s.wg.Wait()
	for _, f := range s.files {
		_ = f.Close()
	}
	s.files = nil
}
