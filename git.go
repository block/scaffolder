package scaffolder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/block/getit"
)

// commitHash matches a full SHA-1 or SHA-256 commit hash.
var commitHash = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// gitSource is a Source for a git repository, whose revisions are commit hashes.
type gitSource struct {
	git    *getit.Git
	source getit.Source
}

// Open clones the ref the source names, or the default branch.
func (g *gitSource) Open(ctx context.Context) (*Revision, error) {
	return g.fetch(ctx, g.source, "")
}

// OpenRevision clones the whole repository, since the commit can be on any
// branch and older than a shallow clone reaches.
func (g *gitSource) OpenRevision(ctx context.Context, revision string) (*Revision, error) {
	if !commitHash.MatchString(revision) {
		return nil, fmt.Errorf("invalid revision %q: not a full commit hash", revision)
	}
	u := *g.source.URL
	u.RawQuery = ""
	return g.fetch(ctx, getit.Source{URL: &u, SubDir: g.source.SubDir}, revision)
}

// fetch clones source and checks out revision, or keeps the cloned HEAD if
// revision is empty.
func (g *gitSource) fetch(ctx context.Context, source getit.Source, revision string) (*Revision, error) {
	dir, err := os.MkdirTemp("", "scaffolder-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	rev := &Revision{cleanup: func() { _ = os.RemoveAll(dir) }}
	id, err := g.checkout(ctx, source, revision, dir)
	if err != nil {
		rev.Close()
		return nil, err
	}
	rev.ID = id
	rev.Files = os.DirFS(filepath.Join(dir, source.SubDir))
	return rev, nil
}

// checkout clones source into dir at revision and returns its commit hash.
func (g *gitSource) checkout(ctx context.Context, source getit.Source, revision, dir string) (string, error) {
	if err := g.git.Fetch(ctx, source, dir); err != nil {
		return "", fmt.Errorf("failed to fetch source: %w", err)
	}
	if revision != "" {
		if _, err := runGit(ctx, dir, "checkout", "--quiet", "--detach", revision); err != nil {
			return "", fmt.Errorf("failed to check out revision %s: %w", revision, err)
		}
	}
	id, err := runGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to resolve revision: %w", err)
	}
	_ = os.RemoveAll(filepath.Join(dir, ".git"))
	return id, nil
}

// runGit runs git in dir and returns its trimmed output.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...) //nolint:gosec // arguments come from this package and revisions are checked to be commit hashes
	cmd.Stderr = stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}
