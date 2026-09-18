package prune

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	itemsFile = ".pl/expected_items"
	tmpRel    = ".pl/tmp"
)

type run struct {
	logs []string
}

func (r *run) log(msg string) { r.logs = append(r.logs, msg) }

func (r *run) joined() string { return strings.Join(r.logs, "\n") }

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(parts...)
	err := os.MkdirAll(p, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func write(t *testing.T, p string, content string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(p), 0o750)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(p, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

// itemsFor mirrors the runner: one path per line, the list file itself last.
func itemsFor(t *testing.T, entries ...string) Items {
	t.Helper()
	content := strings.Join(entries, "\n") + "\n" + itemsFile + "\n"
	it, err := ParseItems(strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func prune(t *testing.T, workdir string, items Items) *run {
	t.Helper()
	r := &run{}
	err := Run(workdir, items, tmpRel, r.log)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return r
}

func TestParseItems(t *testing.T) {
	it, err := ParseItems(strings.NewReader("a.txt\n\nout/\nsub/b.txt\n"))
	if err != nil {
		t.Fatal(err)
	}
	if it.Len() != 3 {
		t.Fatalf("Len = %d", it.Len())
	}
	if !it.expectedFile("a.txt") || it.expectedFile("out") {
		t.Error("file classification wrong")
	}
	if !it.expectedDir("out") || it.expectedDir("a.txt") {
		t.Error("dir classification wrong")
	}
	if !it.ancestorOfExpected("sub") || it.ancestorOfExpected("su") {
		t.Error("ancestor test wrong")
	}
}

func TestRemovesUnexpectedFiles(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	write(t, filepath.Join(wd, "expected.txt"), "data")
	write(t, filepath.Join(wd, "leftover.bin"), "stale")
	write(t, filepath.Join(wd, itemsFile), "")

	r := prune(t, wd, itemsFor(t, "expected.txt"))

	if !exists(filepath.Join(wd, "expected.txt")) || exists(filepath.Join(wd, "leftover.bin")) {
		t.Fatal("wrong survivors")
	}
	if !strings.Contains(r.joined(), "Pruning unexpected file: leftover.bin") {
		t.Fatalf("logs: %q", r.joined())
	}
}

func TestNestedPaths(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	write(t, filepath.Join(wd, "subdir", "nested.txt"), "keep")
	write(t, filepath.Join(wd, "subdir", "stale.bin"), "stale")
	write(t, filepath.Join(wd, "top.txt"), "top")

	prune(t, wd, itemsFor(t, "subdir/nested.txt", "top.txt"))

	if !exists(filepath.Join(wd, "subdir", "nested.txt")) || !exists(filepath.Join(wd, "top.txt")) {
		t.Fatal("expected files must survive")
	}
	if exists(filepath.Join(wd, "subdir", "stale.bin")) {
		t.Fatal("stale nested file must go")
	}
}

func TestEmptyWorkdirLogsNothing(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	r := prune(t, wd, itemsFor(t, "some.txt"))
	if len(r.logs) != 0 {
		t.Fatalf("logs: %q", r.joined())
	}
}

func TestFilesWithSpaces(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	write(t, filepath.Join(wd, "file with spaces.txt"), "keep")
	write(t, filepath.Join(wd, "stale file.bin"), "stale")

	prune(t, wd, itemsFor(t, "file with spaces.txt"))

	if !exists(filepath.Join(wd, "file with spaces.txt")) || exists(filepath.Join(wd, "stale file.bin")) {
		t.Fatal("wrong survivors")
	}
}

func TestPreservesExpectedDirectories(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	mkdir(t, wd, "expected_empty")
	mkdir(t, wd, "unexpected_empty")
	write(t, filepath.Join(wd, "subdir", "keep.txt"), "keep")

	r := prune(t, wd, itemsFor(t, "subdir/keep.txt", "expected_empty/"))

	if !exists(filepath.Join(wd, "subdir", "keep.txt")) {
		t.Fatal("keep.txt must survive")
	}
	if !exists(filepath.Join(wd, "expected_empty")) {
		t.Fatal("expected empty dir must survive")
	}
	if exists(filepath.Join(wd, "unexpected_empty")) {
		t.Fatal("unexpected empty dir must go")
	}
	if !strings.Contains(r.joined(), "Pruning empty directory: unexpected_empty") {
		t.Fatalf("logs: %q", r.joined())
	}
}

func TestRemovesUnexpectedDirectoriesRecursively(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	write(t, filepath.Join(wd, "pframe_1", "data.bin"), "stale")
	write(t, filepath.Join(wd, "pframe_1", "subdir", "nested.bin"), "stale")
	write(t, filepath.Join(wd, "input.txt"), "input")

	r := prune(t, wd, itemsFor(t, "input.txt"))

	if !exists(filepath.Join(wd, "input.txt")) {
		t.Fatal("input.txt must survive")
	}
	if exists(filepath.Join(wd, "pframe_1")) {
		t.Fatal("stale output dir must be removed entirely")
	}
	if !strings.Contains(r.joined(), "Pruning unexpected file: pframe_1/data.bin") {
		t.Fatalf("logs: %q", r.joined())
	}
}

func TestCleansExpectedDirContents(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	write(t, filepath.Join(wd, "output", "stale.bin"), "stale")
	write(t, filepath.Join(wd, "input.txt"), "input")

	r := prune(t, wd, itemsFor(t, "input.txt", "output/"))

	if !exists(filepath.Join(wd, "output")) {
		t.Fatal("expected dir must survive")
	}
	if exists(filepath.Join(wd, "output", "stale.bin")) {
		t.Fatal("stale file inside expected dir must go")
	}
	if !strings.Contains(r.joined(), "Pruning unexpected file: output/stale.bin") {
		t.Fatalf("logs: %q", r.joined())
	}
}

func TestServiceDirSurvivesButStaleMarkerDoesNot(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	write(t, filepath.Join(wd, "input.txt"), "keep")
	write(t, filepath.Join(wd, itemsFile), "input.txt\n"+itemsFile+"\n")
	write(t, filepath.Join(wd, ".pl", "completed"), "0\n")
	write(t, filepath.Join(wd, ".pl", "usage.json"), "{}")

	prune(t, wd, itemsFor(t, "input.txt"))

	if !exists(filepath.Join(wd, ".pl")) || !exists(filepath.Join(wd, itemsFile)) {
		t.Fatal("service dir and item list must survive")
	}
	if exists(filepath.Join(wd, ".pl", "completed")) {
		t.Fatal("a marker from the killed attempt must not survive")
	}
	if exists(filepath.Join(wd, ".pl", "usage.json")) {
		t.Fatal("a usage report from the killed attempt must not survive")
	}
	if !exists(filepath.Join(wd, "input.txt")) {
		t.Fatal("input.txt must survive")
	}
}

func TestOnlyDirectoriesExpected(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	mkdir(t, wd, "output_dir")
	write(t, filepath.Join(wd, "stale.bin"), "stale")

	it, err := ParseItems(strings.NewReader("output_dir/\n"))
	if err != nil {
		t.Fatal(err)
	}
	r := prune(t, wd, it)

	if !exists(filepath.Join(wd, "output_dir")) || exists(filepath.Join(wd, "stale.bin")) {
		t.Fatal("wrong survivors")
	}
	if !strings.Contains(r.joined(), "Pruning unexpected file: stale.bin") {
		t.Fatalf("logs: %q", r.joined())
	}
}

func TestLargeItemsList(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	mkdir(t, wd, "pdbs")
	const n = 9000
	items := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		item := fmt.Sprintf("pdbs/s%07d.pdb", i)
		items = append(items, item)
		write(t, filepath.Join(wd, item), "x")
	}
	write(t, filepath.Join(wd, "leftover.bin"), "stale")

	prune(t, wd, itemsFor(t, items...))

	last := fmt.Sprintf("s%07d.pdb", n)
	if !exists(filepath.Join(wd, "pdbs", "s0000001.pdb")) || !exists(filepath.Join(wd, "pdbs", last)) {
		t.Fatal("expected files must survive")
	}
	if exists(filepath.Join(wd, "leftover.bin")) {
		t.Fatal("leftover must go")
	}
}

func TestTmpSubtreeNeverWalked(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	write(t, filepath.Join(wd, "expected.txt"), "data")
	write(t, filepath.Join(wd, tmpRel, "leftover.tmp"), "stale")
	write(t, filepath.Join(wd, tmpRel, "nested", "deeper.tmp"), "stale")

	r := prune(t, wd, itemsFor(t, "expected.txt"))

	if strings.Contains(r.joined(), "leftover.tmp") || strings.Contains(r.joined(), "deeper.tmp") {
		t.Fatalf("temporary subtree must never be reported: %q", r.joined())
	}
	if strings.Contains(r.joined(), "Pruning unexpected directory: .pl/tmp") ||
		strings.Contains(r.joined(), "Pruning empty directory: .pl/tmp") {
		t.Fatalf("the temporary directory itself must survive: %q", r.joined())
	}
	if !exists(filepath.Join(wd, tmpRel, "leftover.tmp")) {
		t.Fatal("prune must not touch the temporary subtree")
	}
}

func TestSymlinksAreLeftAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	wd := mkdir(t, t.TempDir(), "workdir")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	write(t, outside, "outside")
	write(t, filepath.Join(wd, "keep.txt"), "keep")
	err := os.Symlink(outside, filepath.Join(wd, "link.txt"))
	if err != nil {
		t.Fatal(err)
	}
	outsideDir := mkdir(t, t.TempDir(), "outside_dir")
	write(t, filepath.Join(outsideDir, "victim.txt"), "victim")
	err = os.Symlink(outsideDir, filepath.Join(wd, "linkdir"))
	if err != nil {
		t.Fatal(err)
	}

	r := prune(t, wd, itemsFor(t, "keep.txt"))

	if !exists(filepath.Join(wd, "link.txt")) || !exists(filepath.Join(wd, "linkdir")) {
		t.Fatal("symlinks are neither files nor directories to the prune and must survive")
	}
	if !exists(outside) || !exists(filepath.Join(outsideDir, "victim.txt")) {
		t.Fatal("nothing outside the workdir may be touched")
	}
	if len(r.logs) != 0 {
		t.Fatalf("logs: %q", r.joined())
	}
}

func TestEmptyDir(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	write(t, filepath.Join(wd, "expected.txt"), "data")
	for _, name := range []string{"a.tmp", "b.tmp", "c.tmp"} {
		write(t, filepath.Join(wd, tmpRel, name), "stale")
	}
	mkdir(t, wd, tmpRel, "nested", "deeper")

	EmptyDir(wd, tmpRel)

	entries, err := os.ReadDir(filepath.Join(wd, tmpRel))
	if err != nil {
		t.Fatal("the directory itself must stay: it is a mount point on Kubernetes")
	}
	if len(entries) != 0 {
		t.Fatalf("temporary directory not emptied: %d entries", len(entries))
	}
	if !exists(filepath.Join(wd, "expected.txt")) {
		t.Fatal("emptying must not touch anything outside the directory")
	}
}

func TestEmptyDirMissingIsHarmless(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	EmptyDir(wd, tmpRel)
	EmptyDir(filepath.Join(wd, "nope"), tmpRel)
}

func TestEmptyDirLargeContents(t *testing.T) {
	wd := mkdir(t, t.TempDir(), "workdir")
	for i := range 5000 {
		write(t, filepath.Join(wd, tmpRel, fmt.Sprintf("leftover-%d.tmp", i)), "x")
	}
	EmptyDir(wd, tmpRel)
	entries, _ := os.ReadDir(filepath.Join(wd, tmpRel))
	if len(entries) != 0 {
		t.Fatalf("%d entries left", len(entries))
	}
}
