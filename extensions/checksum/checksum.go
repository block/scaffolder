// Package checksum records checksums of scaffolded files, and reports local
// changes that re-scaffolding overwrote or deleted.
package checksum

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/block/scaffolder"
)

// File is the name of the checksum file in the destination root, in sha256sum format.
const File = ".scaffolder.sum"

const tempFile = File + ".tmp"

var sumLine = regexp.MustCompile(`^([0-9a-f]{64})  (.+)\n?$`)

// ChangeKind describes what happened to a file with local changes.
type ChangeKind int

const (
	// OverwroteChanges means a file that had changed since it was last
	// scaffolded, or that was not in File, was overwritten.
	OverwroteChanges ChangeKind = iota
	// Deleted means a file that is no longer produced was deleted.
	Deleted
	// DeletedChanges means a file that is no longer produced, and that had
	// changed since it was last scaffolded, was deleted.
	DeletedChanges
)

func (k ChangeKind) String() string {
	switch k {
	case OverwroteChanges:
		return "overwrote local changes"
	case Deleted:
		return "deleted"
	case DeletedChanges:
		return "deleted with local changes"
	}
	return fmt.Sprintf("ChangeKind(%d)", int(k))
}

// Change is a file whose local changes were overwritten, or that was deleted.
type Change struct {
	// Path is slash-separated and relative to the destination.
	Path string
	Kind ChangeKind
}

// Extension records written files in File and reports local changes that were overwritten or
// deleted. It must be registered after any extension that modifies written files.
func Extension(report func(Change)) scaffolder.Extension {
	return &extension{report: report}
}

type extension struct {
	report func(Change)
	target string
	// root confines every file access, including deletion of paths read
	// from File, to the destination. It is nil until the destination exists.
	root *os.Root
	// previous is nil when the destination had no File, which disables
	// reporting and deletion.
	previous map[string]string
	produced map[string]bool
	// before holds checksums of files as they were on disk just before this
	// run first wrote them.
	before map[string]string
}

func (e *extension) Extend(*scaffolder.Config) error { return nil }

func (e *extension) Before(config *scaffolder.Config) error {
	*e = extension{
		report:   e.report,
		target:   config.Target(),
		produced: map[string]bool{},
		before:   map[string]string{},
	}
	root, err := os.OpenRoot(e.target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to open destination: %w", err)
	}
	e.root = root
	data, err := root.ReadFile(File)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", File, err)
	}
	e.previous, err = parseSums(data)
	return err
}

func (e *extension) BeforeEach(path string) error {
	rel, err := e.relPath(path)
	if err != nil {
		return err
	}
	if rel == File || rel == tempFile {
		return fmt.Errorf("%s: this name is reserved for checksum tracking, rename it in the template", rel)
	}
	if strings.ContainsAny(rel, "\\\n") {
		return fmt.Errorf("%q: checksum tracking does not support backslashes or newlines in names, rename it in the template", rel)
	}
	// Once this run has written a path, what is on disk is this run's
	// output, not the user's, so only the first write is compared.
	if e.produced[rel] {
		return nil
	}
	e.produced[rel] = true
	if e.previous == nil {
		return nil
	}
	sum, ok, err := e.hashRegular(rel)
	if err != nil || !ok {
		return err
	}
	e.before[rel] = sum
	return nil
}

func (e *extension) AfterEach(string) error { return nil }

func (e *extension) After() error {
	if e.root == nil {
		root, err := os.OpenRoot(e.target)
		if err != nil {
			return fmt.Errorf("failed to open destination: %w", err)
		}
		e.root = root
	}
	defer e.root.Close()

	// Hashing here rather than in AfterEach picks up changes that other
	// extensions make after their own AfterEach.
	sums := map[string]string{}
	for rel := range e.produced {
		sum, ok, err := e.hashRegular(rel)
		if err != nil {
			return err
		}
		if ok {
			sums[rel] = sum
		}
	}

	changes := e.overwritten(sums)
	deleted, err := e.deleteUnproduced()
	if err != nil {
		return err
	}
	changes = append(changes, deleted...)

	if err := e.writeSums(sums); err != nil {
		return fmt.Errorf("failed to write %s: %w", File, err)
	}
	if err := e.root.Rename(tempFile, File); err != nil {
		return fmt.Errorf("failed to write %s: %w", File, err)
	}

	slices.SortFunc(changes, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	for _, change := range changes {
		e.report(change)
	}
	return nil
}

// overwritten returns the files whose local changes were overwritten.
func (e *extension) overwritten(sums map[string]string) []Change {
	var changes []Change
	for rel, old := range e.before {
		previous, tracked := e.previous[rel]
		// A file that matched its checksum had no local changes, and a local
		// change that already matches the new output lost nothing.
		if (tracked && old == previous) || old == sums[rel] {
			continue
		}
		changes = append(changes, Change{Path: rel, Kind: OverwroteChanges})
	}
	return changes
}

// deleteUnproduced deletes files listed in the previous File that this run
// did not produce.
func (e *extension) deleteUnproduced() ([]Change, error) {
	var changes []Change
	for rel, previous := range e.previous {
		if e.produced[rel] {
			continue
		}
		sum, ok, err := e.hashRegular(rel)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if err := e.root.Remove(filepath.FromSlash(rel)); err != nil {
			return nil, fmt.Errorf("failed to delete %s: %w", rel, err)
		}
		kind := Deleted
		if sum != previous {
			kind = DeletedChanges
		}
		changes = append(changes, Change{Path: rel, Kind: kind})
	}
	return changes, nil
}

func (e *extension) relPath(path string) (string, error) {
	rel, err := filepath.Rel(e.target, path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%s: the template writes outside the destination %s", path, e.target)
	}
	return filepath.ToSlash(rel), nil
}

// hashRegular returns the checksum of rel, or false if it is not a regular
// file.
func (e *extension) hashRegular(rel string) (string, bool, error) {
	name := filepath.FromSlash(rel)
	info, err := e.root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("%s: %w", rel, err)
	}
	if !info.Mode().IsRegular() {
		return "", false, nil
	}
	f, err := e.root.Open(name)
	if err != nil {
		return "", false, fmt.Errorf("%s: %w", rel, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false, fmt.Errorf("%s: %w", rel, err)
	}
	return hex.EncodeToString(h.Sum(nil)), true, nil
}

func parseSums(data []byte) (map[string]string, error) {
	sums := map[string]string{}
	lineNo := 0
	for line := range strings.Lines(string(data)) {
		lineNo++
		match := sumLine.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("%s:%d: invalid line, expected \"<sha256>  <path>\"", File, lineNo)
		}
		sum, rel := match[1], match[2]
		// Paths must be canonical, so that each one matches exactly one
		// produced path and cannot reach outside the destination.
		if strings.Contains(rel, "\\") || path.Clean(rel) != rel || !filepath.IsLocal(filepath.FromSlash(rel)) {
			return nil, fmt.Errorf("%s:%d: invalid path %q, expected a clean relative path inside the destination", File, lineNo, rel)
		}
		if _, ok := sums[rel]; ok {
			return nil, fmt.Errorf("%s:%d: duplicate path %q", File, lineNo, rel)
		}
		sums[rel] = sum
	}
	return sums, nil
}

func (e *extension) writeSums(sums map[string]string) error {
	f, err := e.root.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, rel := range slices.Sorted(maps.Keys(sums)) {
		fmt.Fprintf(w, "%s  %s\n", sums[rel], rel)
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
