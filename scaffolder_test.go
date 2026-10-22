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
	err := scaffolder.Scaffold("testdata/template", tmpDir, map[string]any{
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
	err := scaffolder.Scaffold("git+https://github.com/block/scaffolder//testdata/template?ref=main&depth=1", tmpDir, map[string]any{
		"List":    []string{"first", "second"},
		"Name":    "test",
		"Include": true,
	}, scaffolder.WithContext(context.Background()), scaffolder.Exclude("excluded"))
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

func TestScaffoldFS(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "new")
	err := scaffolder.ScaffoldFS(os.DirFS("testdata/template"), tmpDir, map[string]any{
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

func TestScaffoldFSMapFS(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "new")
	source := fstest.MapFS{
		"{{ .Name }}.txt.tmpl": {Data: []byte("Hello, {{ .Name }}!\n"), Mode: 0o644},
		"bin/run":              {Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		"link-{{ .Name }}":     {Data: []byte("{{ .Name }}.txt"), Mode: fs.ModeSymlink},
	}
	err := scaffolder.ScaffoldFS(source, tmpDir, map[string]any{"Name": "test"})
	assert.NoError(t, err)
	expect := []scaffoldertest.File{
		{Name: "bin/run", Mode: 0o700, Content: "#!/bin/sh\n"},
		{Name: "link-test", Mode: 0o700 | os.ModeSymlink, Content: "Hello, test!\n"},
		{Name: "test.txt", Mode: 0o600, Content: "Hello, test!\n"},
	}
	scaffoldertest.AssertFilesEqual(t, tmpDir, expect)
}
