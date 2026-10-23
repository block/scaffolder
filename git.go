package scaffolder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/block/getit"
)

// gitSource is a Source for a git repository, whose revisions are commit hashes.
type gitSource struct {
	git    *getit.Git
	source getit.Source
}

// Open clones the ref the source names, or the default branch.
func (g *gitSource) Open(ctx context.Context) (*Revision, error) {
	return g.fetch(ctx, g.source)
}

// OpenRevision fetches only the given commit, which needs git 2.49 or later.
func (g *gitSource) OpenRevision(ctx context.Context, revision string) (*Revision, error) {
	u := *g.source.URL
	query := u.Query()
	query.Del("ref")
	query.Set("commit", revision)
	query.Set("depth", "1")
	u.RawQuery = query.Encode()
	return g.fetch(ctx, getit.Source{URL: &u, SubDir: g.source.SubDir})
}

// fetch clones source and returns its templates at the cloned commit.
func (g *gitSource) fetch(ctx context.Context, source getit.Source) (*Revision, error) {
	dir, err := os.MkdirTemp("", "scaffolder-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	rev := &Revision{cleanup: func() { _ = os.RemoveAll(dir) }}
	if err := g.git.Fetch(ctx, source, dir); err != nil {
		rev.Close()
		return nil, fmt.Errorf("failed to fetch source: %w", err)
	}
	id, err := runGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		rev.Close()
		return nil, fmt.Errorf("failed to resolve revision: %w", err)
	}
	_ = os.RemoveAll(filepath.Join(dir, ".git"))
	rev.ID = id
	rev.Files = os.DirFS(filepath.Join(dir, source.SubDir))
	return rev, nil
}

// runGit runs git in dir and returns its trimmed output.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...) //nolint:gosec // arguments come from this package
	cmd.Stderr = stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}
