package scaffolder

import (
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/block/getit"
)

func TestGitSourceRevisions(t *testing.T) {
	repo := t.TempDir()
	git(t, repo, "init", "--quiet", "--initial-branch=main")
	first := commit(t, repo, "first")
	second := commit(t, repo, "second")
	// A shallow clone of the ref does not reach the first commit, and git rejects a ref with a commit.
	source := &gitSource{git: getit.NewGit(), source: getit.Source{
		URL:    &url.URL{Scheme: "file", Path: repo, RawQuery: "ref=main&depth=1"},
		SubDir: "app",
	}}

	latest, err := source.Open(t.Context())
	assert.NoError(t, err)
	defer latest.Close()
	assert.Equal(t, second, latest.ID)
	assertFile(t, latest.Files, "second")

	earlier, err := source.OpenRevision(t.Context(), first)
	assert.NoError(t, err)
	assert.Equal(t, first, earlier.ID)
	assertFile(t, earlier.Files, "first")

	earlier.Close()
	_, err = fs.ReadFile(earlier.Files, "file.txt")
	assert.IsError(t, err, fs.ErrNotExist)
}

// commit writes content to app/file.txt in repo, commits it and returns the commit hash.
func commit(t *testing.T, repo, content string) string {
	t.Helper()
	assert.NoError(t, os.MkdirAll(filepath.Join(repo, "app"), 0o750))
	assert.NoError(t, os.WriteFile(filepath.Join(repo, "app", "file.txt"), []byte(content), 0o600))
	git(t, repo, "add", ".")
	git(t, repo, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "--quiet", "-m", content)
	return git(t, repo, "rev-parse", "HEAD")
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(t.Context(), dir, args...)
	assert.NoError(t, err)
	return out
}

func assertFile(t *testing.T, files fs.FS, expected string) {
	t.Helper()
	content, err := fs.ReadFile(files, "file.txt")
	assert.NoError(t, err)
	assert.Equal(t, expected, string(content))
}
