package scaffolder_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/alecthomas/assert/v2"

	"github.com/block/scaffolder"
	"github.com/block/scaffolder/scaffoldertest"
)

func TestScaffolder(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "new")
	err := scaffolder.Scaffold(os.DirFS("testdata/template"), tmpDir, map[string]any{
		"List":    []string{"first", "second"},
		"Name":    "test",
		"Include": true,
	}, scaffolder.Exclude("excluded"))
	assert.NoError(t, err)
	expect := []scaffoldertest.File{
		{Name: "first.txt", Mode: 0o600, Content: "first"},
		{Name: "first/first", Mode: 0o600},
		{Name: "include", Mode: 0o600, Content: "included"},
		{Name: "included-dir/included", Mode: 0o600, Content: "included"},
		{Name: "intermediate", Mode: 0o700 | os.ModeSymlink, Content: "Hello, test!\n"},
		{Name: "regular-test", Mode: 0o600, Content: "Hello, test!\n"},
		{Name: "second.txt", Mode: 0o600, Content: "second"},
		{Name: "second/second", Mode: 0o600},
		{Name: "symlink-test", Mode: 0o700 | os.ModeSymlink, Content: "Hello, test!\n"},
	}
	scaffoldertest.AssertFilesEqual(t, tmpDir, expect)
}

func TestScaffolderRemoteSource(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping remote source test in short mode")
	}
	tmpDir := filepath.Join(t.TempDir(), "new")
	// Use this repo's testdata as a remote source
	source, cleanup, err := scaffolder.Open(context.Background(), "git+https://github.com/block/scaffolder//testdata/template?ref=main&depth=1")
	assert.NoError(t, err)
	defer cleanup()
	err = scaffolder.Scaffold(source, tmpDir, map[string]any{
		"List":    []string{"first", "second"},
		"Name":    "test",
		"Include": true,
	}, scaffolder.Exclude("excluded"))
	assert.NoError(t, err)
	expect := []scaffoldertest.File{
		{Name: "first.txt", Mode: 0o600, Content: "first"},
		{Name: "first/first", Mode: 0o600},
		{Name: "include", Mode: 0o600, Content: "included"},
		{Name: "included-dir/included", Mode: 0o600, Content: "included"},
		{Name: "intermediate", Mode: 0o700 | os.ModeSymlink, Content: "Hello, test!\n"},
		{Name: "regular-test", Mode: 0o600, Content: "Hello, test!\n"},
		{Name: "second.txt", Mode: 0o600, Content: "second"},
		{Name: "second/second", Mode: 0o600},
		{Name: "symlink-test", Mode: 0o700 | os.ModeSymlink, Content: "Hello, test!\n"},
	}
	scaffoldertest.AssertFilesEqual(t, tmpDir, expect)
}

func TestScaffolderMapFS(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "new")
	source := fstest.MapFS{
		"{{ .Name }}.txt.tmpl": {Data: []byte("Hello, {{ .Name }}!\n"), Mode: 0o644},
		"bin/run":              {Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		"link-{{ .Name }}":     {Data: []byte("{{ .Name }}.txt"), Mode: fs.ModeSymlink},
	}
	err := scaffolder.Scaffold(source, tmpDir, map[string]any{"Name": "test"})
	assert.NoError(t, err)
	expect := []scaffoldertest.File{
		{Name: "bin/run", Mode: 0o700, Content: "#!/bin/sh\n"},
		{Name: "link-test", Mode: 0o700 | os.ModeSymlink, Content: "Hello, test!\n"},
		{Name: "test.txt", Mode: 0o600, Content: "Hello, test!\n"},
	}
	scaffoldertest.AssertFilesEqual(t, tmpDir, expect)
}

func TestScaffolderWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := scaffolder.Scaffold(os.DirFS("testdata/template"), filepath.Join(t.TempDir(), "new"), nil, scaffolder.WithContext(ctx))
	assert.IsError(t, err, context.Canceled)
}

type recorder struct {
	name       string
	target     string
	events     *[]string
	extend     func(config *scaffolder.Config) error
	before     func(config *scaffolder.Config) error
	beforeEach func(path string) error
	afterEach  func(path string) error
	after      func() error
}

func (r *recorder) Extend(config *scaffolder.Config) error {
	r.target = config.Target()
	r.record("Extend", "")
	if r.extend != nil {
		return r.extend(config)
	}
	return nil
}

func (r *recorder) Before(config *scaffolder.Config) error {
	r.record("Before", "")
	if r.before != nil {
		return r.before(config)
	}
	return nil
}

func (r *recorder) BeforeEach(path string) error {
	r.record("BeforeEach", path)
	if r.beforeEach != nil {
		return r.beforeEach(path)
	}
	return nil
}

func (r *recorder) AfterEach(path string) error {
	r.record("AfterEach", path)
	if r.afterEach != nil {
		return r.afterEach(path)
	}
	return nil
}

func (r *recorder) After() error {
	r.record("After", "")
	if r.after != nil {
		return r.after()
	}
	return nil
}

func (r *recorder) record(hook, path string) {
	event := r.name + "." + hook
	if path != "" {
		rel, err := filepath.Rel(r.target, path)
		if err != nil {
			panic(err)
		}
		event += " " + filepath.ToSlash(rel)
	}
	*r.events = append(*r.events, event)
}

func TestExtensionHookOrder(t *testing.T) {
	var events []string
	source := fstest.MapFS{
		"dir/file": {Data: []byte("content"), Mode: 0o644},
		"link":     {Data: []byte("dir/file"), Mode: fs.ModeSymlink},
	}
	err := scaffolder.Scaffold(source, filepath.Join(t.TempDir(), "new"), nil,
		scaffolder.Extend(&recorder{name: "A", events: &events}),
		scaffolder.Extend(&recorder{name: "B", events: &events}))
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"A.Extend", "B.Extend",
		"A.Before", "B.Before",
		"A.BeforeEach dir", "B.BeforeEach dir", "A.AfterEach dir", "B.AfterEach dir",
		"A.BeforeEach dir/file", "B.BeforeEach dir/file", "A.AfterEach dir/file", "B.AfterEach dir/file",
		"A.After", "B.After",
	}, events)
}

func TestBeforeSeesFinalConfig(t *testing.T) {
	var events []string
	var exclude []string
	err := scaffolder.Scaffold(fstest.MapFS{}, filepath.Join(t.TempDir(), "new"), nil,
		scaffolder.Extend(&recorder{name: "A", events: &events, before: func(config *scaffolder.Config) error {
			exclude = config.Exclude
			return nil
		}}),
		scaffolder.Extend(&recorder{name: "B", events: &events, extend: func(config *scaffolder.Config) error {
			config.Exclude = append(config.Exclude, "excluded")
			return nil
		}}))
	assert.NoError(t, err)
	assert.Equal(t, []string{"excluded"}, exclude)
}

func TestBeforeEachSeesOldContent(t *testing.T) {
	var events []string
	dest := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dest, "file"), []byte("old"), 0o600))
	var contents []string
	readFile := func(path string) error {
		content, err := os.ReadFile(path)
		contents = append(contents, string(content))
		return err
	}
	err := scaffolder.Scaffold(fstest.MapFS{"file": {Data: []byte("new"), Mode: 0o644}}, dest, nil,
		scaffolder.Extend(&recorder{name: "A", events: &events, beforeEach: readFile, afterEach: readFile}))
	assert.NoError(t, err)
	assert.Equal(t, []string{"old", "new"}, contents)
}

func TestAfterRunsAfterSymlinks(t *testing.T) {
	var events []string
	dest := filepath.Join(t.TempDir(), "new")
	var mode fs.FileMode
	source := fstest.MapFS{
		"file": {Data: []byte("content"), Mode: 0o644},
		"link": {Data: []byte("file"), Mode: fs.ModeSymlink},
	}
	err := scaffolder.Scaffold(source, dest, nil,
		scaffolder.Extend(&recorder{name: "A", events: &events, after: func() error {
			info, err := os.Lstat(filepath.Join(dest, "link"))
			if err != nil {
				return err
			}
			mode = info.Mode()
			return nil
		}}))
	assert.NoError(t, err)
	assert.Equal(t, fs.ModeSymlink, mode.Type())
}

func TestAfterNotCalledOnFailure(t *testing.T) {
	var events []string
	source := fstest.MapFS{"file": {Data: []byte("{{ .Unclosed"), Mode: 0o644}}
	err := scaffolder.Scaffold(source, filepath.Join(t.TempDir(), "new"), nil,
		scaffolder.Extend(&recorder{name: "A", events: &events}))
	assert.Error(t, err)
	assert.Equal(t, []string{"A.Extend", "A.Before"}, events)
}

func TestBeforeEachErrorStopsRun(t *testing.T) {
	var events []string
	dest := filepath.Join(t.TempDir(), "new")
	errBeforeEach := errors.New("refused")
	err := scaffolder.Scaffold(fstest.MapFS{"file": {Data: []byte("content"), Mode: 0o644}}, dest, nil,
		scaffolder.Extend(&recorder{name: "A", events: &events, beforeEach: func(string) error { return errBeforeEach }}))
	assert.IsError(t, err, errBeforeEach)
	_, err = os.Stat(filepath.Join(dest, "file"))
	assert.IsError(t, err, fs.ErrNotExist)
}
