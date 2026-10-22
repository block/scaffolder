package scaffolder_test

import (
	"context"
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

func TestScaffolderSkip(t *testing.T) {
	tmpDir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(tmpDir, "kept.txt"), []byte("edited"), 0o600))
	source := fstest.MapFS{
		"kept.txt":         {Data: []byte("{{ .Name }}"), Mode: 0o644},
		"skipped/file.txt": {Data: []byte("{{ .Name }}"), Mode: 0o644},
		"written.txt":      {Data: []byte("{{ .Name }}"), Mode: 0o644},
	}
	err := scaffolder.Scaffold(source, tmpDir, map[string]any{"Name": "test"}, scaffolder.Skip(func(path string) bool {
		_, err := os.Stat(path)
		return err == nil || filepath.Base(path) == "skipped"
	}))
	assert.NoError(t, err)
	expect := []scaffoldertest.File{
		{Name: "kept.txt", Mode: 0o600, Content: "edited"},
		{Name: "written.txt", Mode: 0o600, Content: "test"},
	}
	scaffoldertest.AssertFilesEqual(t, tmpDir, expect)
}

func TestScaffolderErrorOnMissingKey(t *testing.T) {
	source := fstest.MapFS{"file.txt": {Data: []byte("{{ .Missing }}"), Mode: 0o644}}
	err := scaffolder.Scaffold(source, t.TempDir(), map[string]any{}, scaffolder.ErrorOnMissingKey())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `map has no entry for key "Missing"`)
}

func TestScaffolderAfterEachSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	source := fstest.MapFS{
		"file.txt": {Data: []byte("file"), Mode: 0o644},
		"link":     {Data: []byte("file.txt"), Mode: fs.ModeSymlink},
	}
	var created []string
	err := scaffolder.Scaffold(source, tmpDir, nil, scaffolder.AfterEach(func(path string) error {
		rel, err := filepath.Rel(tmpDir, path)
		created = append(created, rel)
		return err
	}))
	assert.NoError(t, err)
	assert.Equal(t, []string{"file.txt", "link"}, created)
}
