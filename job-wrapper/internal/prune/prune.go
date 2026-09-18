// Package prune removes leftover output of an earlier attempt from a working directory before the
// command starts, exactly the way job-script.sh did it: files not in the expected-items list go,
// directories not in the list and not holding an expected item go, and the command's temporary
// subtree is never walked.
//
// Every filesystem operation goes through os.Root, so a symlink the previous attempt planted in
// the working directory cannot redirect a removal outside of it. Symlinks are otherwise left alone,
// as `find -type f` / `find -type d` left them alone.
package prune

import (
	"bufio"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// LogPrefix is the prefix job-script.sh put in front of every line it wrote to stderr. Kept
// verbatim so pod logs read the same whichever wrapper produced them.
const LogPrefix = "[job-script]"

// Items is the parsed expected-items list.
type Items struct {
	files map[string]struct{}
	dirs  map[string]struct{} // keys keep their trailing slash, as written
	all   []string
}

// ParseItems reads one relative path per line. Empty lines are skipped; entries ending in "/"
// are directories, everything else is a file.
func ParseItems(r io.Reader) (Items, error) {
	it := Items{files: map[string]struct{}{}, dirs: map[string]struct{}{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		it.all = append(it.all, line)
		if strings.HasSuffix(line, "/") {
			it.dirs[line] = struct{}{}
		} else {
			it.files[line] = struct{}{}
		}
	}
	return it, sc.Err()
}

// Len is the number of entries in the list.
func (it Items) Len() int { return len(it.all) }

func (it Items) expectedFile(rel string) bool {
	_, ok := it.files[rel]
	return ok
}

func (it Items) expectedDir(rel string) bool {
	_, ok := it.dirs[rel+"/"]
	return ok
}

// ancestorOfExpected reports whether any expected item lives under rel.
func (it Items) ancestorOfExpected(rel string) bool {
	prefix := rel + "/"
	for _, item := range it.all {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

// Logger receives one message per pruned entry, without the LogPrefix and without a newline.
type Logger func(msg string)

// Run prunes workdir against items. skip is a slash-separated path relative to workdir whose
// subtree is neither visited nor removed (the command's temporary directory).
//
// It mirrors the two passes of job-script.sh: first every unexpected regular file is removed,
// then directories are visited depth-first and an unexpected one that shelters no expected item
// is removed - with rmdir when it is empty by then, recursively otherwise.
func Run(workdir string, items Items, skip string, log Logger) error {
	root, err := os.OpenRoot(workdir)
	if err != nil {
		return err
	}
	defer root.Close()

	skip = path.Clean(skip)
	if err := pruneFiles(root, ".", items, skip, log); err != nil {
		return err
	}
	return pruneDirs(root, ".", items, skip, log)
}

// EmptyDir removes everything inside rel (relative to workdir) and keeps rel itself. Errors are
// swallowed, as `find "$dir" -mindepth 1 -delete 2>/dev/null || true` swallowed them.
func EmptyDir(workdir, rel string) {
	root, err := os.OpenRoot(workdir)
	if err != nil {
		return
	}
	defer root.Close()

	entries, err := readDir(root, rel)
	if err != nil {
		return
	}
	for _, e := range entries {
		sub := path.Join(rel, e.Name())
		if e.Type().IsDir() {
			removeAll(root, sub)
		} else {
			_ = root.Remove(sub)
		}
	}
}

func readDir(root *os.Root, rel string) ([]fs.DirEntry, error) {
	f, err := root.Open(rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func pruneFiles(root *os.Root, dir string, items Items, skip string, log Logger) error {
	entries, err := readDir(root, dir)
	if err != nil {
		if dir == "." {
			return err
		}
		// find keeps going past a directory it cannot read; so do we.
		return nil
	}
	for _, e := range entries {
		rel := path.Join(dir, e.Name())
		if rel == skip {
			continue
		}
		switch {
		case e.Type().IsDir():
			if err := pruneFiles(root, rel, items, skip, log); err != nil {
				return err
			}
		case e.Type().IsRegular():
			if !items.expectedFile(rel) {
				log("Pruning unexpected file: " + rel)
				_ = root.Remove(rel) // rm -f
			}
		default:
			// symlinks, fifos, sockets: neither -type f nor -type d, untouched.
		}
	}
	return nil
}

func pruneDirs(root *os.Root, dir string, items Items, skip string, log Logger) error {
	entries, err := readDir(root, dir)
	if err != nil {
		if dir == "." {
			return err
		}
		return nil
	}
	for _, e := range entries {
		rel := path.Join(dir, e.Name())
		if rel == skip || !e.Type().IsDir() {
			continue
		}
		// Depth-first: children decide before their parent.
		if err := pruneDirs(root, rel, items, skip, log); err != nil {
			return err
		}
		if items.expectedDir(rel) || items.ancestorOfExpected(rel) {
			continue
		}
		if err := root.Remove(rel); err == nil {
			log("Pruning empty directory: " + rel)
			continue
		}
		log("Pruning unexpected directory: " + rel)
		removeAll(root, rel)
	}
	return nil
}

// removeAll is rm -rf confined to root. os.Root gains RemoveAll only in Go 1.25.
func removeAll(root *os.Root, rel string) {
	entries, err := readDir(root, rel)
	if err == nil {
		for _, e := range entries {
			sub := path.Join(rel, e.Name())
			if e.Type().IsDir() {
				removeAll(root, sub)
			} else {
				_ = root.Remove(sub)
			}
		}
	}
	_ = root.Remove(rel)
}
