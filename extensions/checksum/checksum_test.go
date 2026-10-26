package checksum_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/alecthomas/assert/v2"

	"github.com/block/scaffolder"
	"github.com/block/scaffolder/extensions/checksum"
)

func scaffold(t *testing.T, dest string, source fstest.MapFS, options ...scaffolder.Option) ([]checksum.Change, error) {
	t.Helper()
	var changes []checksum.Change
	options = append(options, scaffolder.Extend(checksum.Extension(func(change checksum.Change) {
		changes = append(changes, change)
	})))
	err := scaffolder.Scaffold(source, dest, nil, options...)
	return changes, err
}

func files(nameContent ...string) fstest.MapFS {
	source := fstest.MapFS{}
	for i := 0; i < len(nameContent); i += 2 {
		source[nameContent[i]] = &fstest.MapFile{Data: []byte(nameContent[i+1]), Mode: 0o644}
	}
	return source
}

func sums(pathContent ...string) string {
	out := &strings.Builder{}
	for i := 0; i < len(pathContent); i += 2 {
		sum := sha256.Sum256([]byte(pathContent[i+1]))
		out.WriteString(hex.EncodeToString(sum[:]) + "  " + pathContent[i] + "\n")
	}
	return out.String()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	assert.NoError(t, err)
	return string(content)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestFirstRun(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "new")
	changes, err := scaffold(t, dest, files("b.txt", "b", "dir/a.txt", "a"))
	assert.NoError(t, err)
	assert.Zero(t, changes)
	assert.Equal(t, sums("b.txt", "b", "dir/a.txt", "a"), readFile(t, filepath.Join(dest, checksum.File)))
}

func TestUnchangedRescaffold(t *testing.T) {
	dest := t.TempDir()
	source := files("a.txt", "a", "b.txt", "b")
	_, err := scaffold(t, dest, source)
	assert.NoError(t, err)
	first := readFile(t, filepath.Join(dest, checksum.File))
	changes, err := scaffold(t, dest, source)
	assert.NoError(t, err)
	assert.Zero(t, changes)
	assert.Equal(t, first, readFile(t, filepath.Join(dest, checksum.File)))
}

func TestOverwroteChanges(t *testing.T) {
	dest := t.TempDir()
	_, err := scaffold(t, dest, files("a.txt", "v1"))
	assert.NoError(t, err)
	writeFile(t, filepath.Join(dest, "a.txt"), "edited")
	changes, err := scaffold(t, dest, files("a.txt", "v2"))
	assert.NoError(t, err)
	assert.Equal(t, []checksum.Change{{Path: "a.txt", Kind: checksum.OverwroteChanges}}, changes)
	assert.Equal(t, "v2", readFile(t, filepath.Join(dest, "a.txt")))
	assert.Equal(t, sums("a.txt", "v2"), readFile(t, filepath.Join(dest, checksum.File)))
}

func TestEditMatchingNewOutputIsNotReported(t *testing.T) {
	dest := t.TempDir()
	_, err := scaffold(t, dest, files("a.txt", "v1"))
	assert.NoError(t, err)
	writeFile(t, filepath.Join(dest, "a.txt"), "v2")
	changes, err := scaffold(t, dest, files("a.txt", "v2"))
	assert.NoError(t, err)
	assert.Zero(t, changes)
}

func TestUntrackedFileOverwritten(t *testing.T) {
	dest := t.TempDir()
	_, err := scaffold(t, dest, files("a.txt", "a"))
	assert.NoError(t, err)
	writeFile(t, filepath.Join(dest, "c.txt"), "user")
	changes, err := scaffold(t, dest, files("a.txt", "a", "c.txt", "template"))
	assert.NoError(t, err)
	assert.Equal(t, []checksum.Change{{Path: "c.txt", Kind: checksum.OverwroteChanges}}, changes)
}

func TestNoChecksumFileReportsNothing(t *testing.T) {
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, "a.txt"), "user")
	changes, err := scaffold(t, dest, files("a.txt", "template"))
	assert.NoError(t, err)
	assert.Zero(t, changes)
}

func TestDeleted(t *testing.T) {
	dest := t.TempDir()
	_, err := scaffold(t, dest, files("a.txt", "a", "b.txt", "b", "c.txt", "c"))
	assert.NoError(t, err)
	writeFile(t, filepath.Join(dest, "c.txt"), "edited")
	changes, err := scaffold(t, dest, files("a.txt", "a"))
	assert.NoError(t, err)
	assert.Equal(t, []checksum.Change{
		{Path: "b.txt", Kind: checksum.Deleted},
		{Path: "c.txt", Kind: checksum.DeletedChanges},
	}, changes)
	for _, name := range []string{"b.txt", "c.txt"} {
		_, err = os.Stat(filepath.Join(dest, name))
		assert.IsError(t, err, fs.ErrNotExist)
	}
	assert.Equal(t, sums("a.txt", "a"), readFile(t, filepath.Join(dest, checksum.File)))
}

func TestAlreadyDeletedIsNotReported(t *testing.T) {
	dest := t.TempDir()
	_, err := scaffold(t, dest, files("a.txt", "a", "b.txt", "b"))
	assert.NoError(t, err)
	assert.NoError(t, os.Remove(filepath.Join(dest, "b.txt")))
	changes, err := scaffold(t, dest, files("a.txt", "a"))
	assert.NoError(t, err)
	assert.Zero(t, changes)
}

func TestInvalidChecksumFile(t *testing.T) {
	sum := sums("a.txt", "a")[:64]
	tests := []struct {
		name    string
		content string
	}{
		{"Malformed", "not a checksum\n"},
		{"Duplicate", sum + "  a.txt\n" + sum + "  a.txt\n"},
		{"Escape", sum + "  ../a.txt\n"},
		{"NotCanonical", sum + "  dir/../a.txt\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dest := t.TempDir()
			writeFile(t, filepath.Join(dest, checksum.File), test.content)
			writeFile(t, filepath.Join(dest, "a.txt"), "user")
			_, err := scaffold(t, dest, files("a.txt", "template"))
			assert.Error(t, err)
			assert.Equal(t, "user", readFile(t, filepath.Join(dest, "a.txt")))
		})
	}
}

func TestEntryThroughSymlinkIsNotDeleted(t *testing.T) {
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "victim"), "victim")
	dest := t.TempDir()
	assert.NoError(t, os.Symlink(outside, filepath.Join(dest, "out")))
	writeFile(t, filepath.Join(dest, checksum.File), sums("out/victim", "victim"))
	changes, err := scaffold(t, dest, files("a.txt", "a"))
	assert.NoError(t, err)
	assert.Zero(t, changes)
	assert.Equal(t, "victim", readFile(t, filepath.Join(outside, "victim")))
}

func TestStaleEntryUnderFileParentIsSkipped(t *testing.T) {
	dest := t.TempDir()
	_, err := scaffold(t, dest, files("a.txt", "a", "dir/old.txt", "old"))
	assert.NoError(t, err)
	assert.NoError(t, os.RemoveAll(filepath.Join(dest, "dir")))
	writeFile(t, filepath.Join(dest, "dir"), "user")
	changes, err := scaffold(t, dest, files("a.txt", "a"))
	assert.NoError(t, err)
	assert.Zero(t, changes)
	assert.Equal(t, sums("a.txt", "a"), readFile(t, filepath.Join(dest, checksum.File)))
}

func TestEntryAliasingProducedFileIsNotDeleted(t *testing.T) {
	dest := t.TempDir()
	_, err := scaffold(t, dest, files("old/a.txt", "a"))
	assert.NoError(t, err)
	assert.NoError(t, os.Rename(filepath.Join(dest, "old"), filepath.Join(dest, "new")))
	assert.NoError(t, os.Symlink("new", filepath.Join(dest, "old")))
	changes, err := scaffold(t, dest, files("new/a.txt", "a"))
	assert.NoError(t, err)
	assert.Zero(t, changes)
	assert.Equal(t, "a", readFile(t, filepath.Join(dest, "new", "a.txt")))
	assert.Equal(t, sums("new/a.txt", "a"), readFile(t, filepath.Join(dest, checksum.File)))
}

func TestTemplateProducingChecksumFile(t *testing.T) {
	_, err := scaffold(t, t.TempDir(), files(checksum.File, "template"))
	assert.Error(t, err)
}

func TestTemplateSymlinkAtChecksumFile(t *testing.T) {
	dest := t.TempDir()
	source := files("a.txt", "a")
	source[checksum.File] = &fstest.MapFile{Data: []byte("a.txt"), Mode: fs.ModeSymlink}
	_, err := scaffold(t, dest, source)
	assert.Error(t, err)
	assert.Equal(t, "a", readFile(t, filepath.Join(dest, "a.txt")))
}

func TestChecksumFileWriteDoesNotFollowSymlinks(t *testing.T) {
	dest := t.TempDir()
	source := files("a.txt", "a")
	source[checksum.File+".tmp"] = &fstest.MapFile{Data: []byte("a.txt"), Mode: fs.ModeSymlink}
	_, err := scaffold(t, dest, source)
	assert.NoError(t, err)
	assert.Equal(t, "a", readFile(t, filepath.Join(dest, "a.txt")))
	assert.Equal(t, sums("a.txt", "a"), readFile(t, filepath.Join(dest, checksum.File)))
}

func TestRecordsChangesFromEarlierExtensions(t *testing.T) {
	dest := t.TempDir()
	// Rewrites a.txt after b.txt, which is written after a.txt.
	appendToA := scaffolder.AfterEach(func(path string) error {
		if filepath.Base(path) != "b.txt" {
			return nil
		}
		return os.WriteFile(filepath.Join(dest, "a.txt"), []byte("a!"), 0o600)
	})
	_, err := scaffold(t, dest, files("a.txt", "a", "b.txt", "b"), appendToA)
	assert.NoError(t, err)
	assert.Equal(t, sums("a.txt", "a!", "b.txt", "b"), readFile(t, filepath.Join(dest, checksum.File)))
}

func TestFanOut(t *testing.T) {
	dest := t.TempDir()
	source := fstest.MapFS{
		"{{ range . }}{{ push . . }}{{ end }}": {Data: []byte("{{ . }}"), Mode: 0o644},
	}
	var changes []checksum.Change
	err := scaffolder.Scaffold(source, dest, []string{"x", "y"},
		scaffolder.Extend(checksum.Extension(func(change checksum.Change) { changes = append(changes, change) })))
	assert.NoError(t, err)
	assert.Zero(t, changes)
	assert.Equal(t, sums("x", "x", "y", "y"), readFile(t, filepath.Join(dest, checksum.File)))
}

func TestSymlinksAreNotTracked(t *testing.T) {
	dest := t.TempDir()
	source := files("a.txt", "a")
	source["link"] = &fstest.MapFile{Data: []byte("a.txt"), Mode: fs.ModeSymlink}
	_, err := scaffold(t, dest, source)
	assert.NoError(t, err)
	changes, err := scaffold(t, dest, files("a.txt", "a"))
	assert.NoError(t, err)
	assert.Zero(t, changes)
	assert.Equal(t, sums("a.txt", "a"), readFile(t, filepath.Join(dest, checksum.File)))
	_, err = os.Lstat(filepath.Join(dest, "link"))
	assert.NoError(t, err)
}

func TestFailedRunKeepsChecksumFile(t *testing.T) {
	dest := t.TempDir()
	_, err := scaffold(t, dest, files("a.txt", "a"))
	assert.NoError(t, err)
	writeFile(t, filepath.Join(dest, "a.txt"), "edited")
	changes, err := scaffold(t, dest, files("a.txt", "new", "b.txt", "{{ .Unclosed"))
	assert.Error(t, err)
	assert.Zero(t, changes)
	assert.Equal(t, sums("a.txt", "a"), readFile(t, filepath.Join(dest, checksum.File)))
}

func TestChangesAreSortedByPath(t *testing.T) {
	dest := t.TempDir()
	names := []string{"e", "a", "d", "b", "c"}
	source := fstest.MapFS{}
	for _, name := range names {
		source[name] = &fstest.MapFile{Data: []byte("v1"), Mode: 0o644}
	}
	_, err := scaffold(t, dest, source)
	assert.NoError(t, err)
	for _, name := range names {
		writeFile(t, filepath.Join(dest, name), "edited")
		source[name] = &fstest.MapFile{Data: []byte("v2"), Mode: 0o644}
	}
	changes, err := scaffold(t, dest, source)
	assert.NoError(t, err)
	expected := []checksum.Change{}
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		expected = append(expected, checksum.Change{Path: name, Kind: checksum.OverwroteChanges})
	}
	assert.Equal(t, expected, changes)
}

func TestReusedExtensionStartsFresh(t *testing.T) {
	var changes []checksum.Change
	extension := scaffolder.Extend(checksum.Extension(func(change checksum.Change) { changes = append(changes, change) }))
	tracked := t.TempDir()
	assert.NoError(t, scaffolder.Scaffold(files("a.txt", "a", "b.txt", "b"), tracked, nil, extension))
	assert.NoError(t, scaffolder.Scaffold(files("a.txt", "a", "b.txt", "b"), tracked, nil, extension))

	untracked := t.TempDir()
	writeFile(t, filepath.Join(untracked, "b.txt"), "user")
	assert.NoError(t, scaffolder.Scaffold(files("a.txt", "a"), untracked, nil, extension))
	assert.Zero(t, changes)
	assert.Equal(t, "user", readFile(t, filepath.Join(untracked, "b.txt")))

	assert.NoError(t, scaffolder.Scaffold(files("a.txt", "a"), filepath.Join(t.TempDir(), "new"), nil, extension))
}
