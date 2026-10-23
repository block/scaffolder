package scaffolder

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/block/getit"
)

// ErrNoRevisions is returned by NewSource for a source that cannot return the
// templates of an earlier revision.
var ErrNoRevisions = errors.New("source does not support revisions")

// Source is a template source that can return the templates of each revision
// it has served, so the output of an earlier scaffold can be reproduced.
type Source interface {
	// Open fetches the latest templates and the revision they came from.
	Open(ctx context.Context) (*Revision, error)
	// OpenRevision fetches the templates of a revision returned by Open.
	OpenRevision(ctx context.Context, revision string) (*Revision, error)
}

// Revision is the templates of a Source at one revision.
type Revision struct {
	// ID identifies the revision to Source.OpenRevision.
	ID string
	// Files are the templates to pass to Scaffold.
	Files fs.FS

	cleanup func()
}

// Close removes the fetched templates.
func (r *Revision) Close() { r.cleanup() }

// NewSource returns the Source for a remote source in any form Open accepts.
//
// Only git sources support revisions, identified by commit hash. NewSource
// returns ErrNoRevisions for any other source.
func NewSource(source string) (Source, error) {
	resolver, resolved, err := getit.Resolve(source)
	if err != nil {
		return nil, fmt.Errorf("%s is not a remote source: %w", source, err)
	}
	git, ok := resolver.(*getit.Git)
	if !ok {
		return nil, fmt.Errorf("%s: %w", source, ErrNoRevisions)
	}
	return &gitSource{git: git, source: resolved}, nil
}
