// Package scaffolder is a general purpose file-system based scaffolding tool
// inspired by cookiecutter.
package scaffolder

import (
	_ "embed"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//
//go:embed README.md
var about string

// About returns text describing how scaffolder works.
func About() string {
	return strings.Join(strings.Split(about, "\n---")[1:], "\n---")
}

const recurseFuncName = "push"

type scaffoldOptions struct {
	Config
	plugins []Extension
}

// Extension's allow the scaffolder to be extended.
type Extension interface {
	Extend(mutableConfig *Config) error
	AfterEach(path string) error
}

// ExtensionFunc is a convenience type for creating an Extension.Extend from a function.
type ExtensionFunc func(mutableConfig *Config) error

func (f ExtensionFunc) Extend(mutableConfig *Config) error { return f(mutableConfig) }
func (f ExtensionFunc) AfterEach(path string) error        { return nil }

// AfterEachExtensionFunc is a convenience type for creating an Extension.AfterEach from a function.
type AfterEachExtensionFunc func(path string) error

func (f AfterEachExtensionFunc) Extend(mutableConfig *Config) error { return nil }
func (f AfterEachExtensionFunc) AfterEach(path string) error        { return f(path) }

// Option is a function that modifies the behaviour of the scaffolder.
type Option func(*scaffoldOptions)

// FuncMap is a map of functions to use in scaffolding templates.
//
// The key is the function name and the value is a function taking a single
// argument and returning either `string` or `(string, error)`.
type FuncMap = template.FuncMap

// Config for the scaffolding.
type Config struct {
	Context any
	Funcs   FuncMap
	Exclude []string

	source string
	target string
}

func (c *Config) Source() string { return c.source }
func (c *Config) Target() string { return c.target }

// Functions adds functions to use in scaffolding templates.
func Functions(funcs FuncMap) Option {
	return func(o *scaffoldOptions) {
		for k, v := range funcs {
			o.Funcs[k] = v
		}
	}
}

// Extend adds an Extension to the scaffolder.
//
// An extension can be used to add functions to the template context, to
// modify the template context, and so on.
func Extend(plugin Extension) Option {
	return func(o *scaffoldOptions) {
		o.plugins = append(o.plugins, plugin)
	}
}

// Exclude the given regex paths from scaffolding.
//
// Matching occurs before template evaluation and .tmpl suffix removal.
func Exclude(paths ...string) Option {
	return func(so *scaffoldOptions) {
		so.Exclude = append(so.Exclude, paths...)
	}
}

// AfterEach configures Scaffolder to call "after" for each file or directory
// created.
//
// Useful for setting file permissions, etc.
//
// Each AfterEach function is called in order.
func AfterEach(after func(path string) error) Option {
	return func(so *scaffoldOptions) {
		so.plugins = append(so.plugins, AfterEachExtensionFunc(after))
	}
}

// Scaffold evaluates the scaffolding files at the given source using ctx, while
// copying them into destination.
func Scaffold(source, destination string, ctx any, options ...Option) error {
	opts := scaffoldOptions{
		Config: Config{
			source:  source,
			target:  destination,
			Context: ctx,
			Funcs: FuncMap{
				recurseFuncName: func(name string, ctx any) (string, error) { panic("not implemented") },
			},
		},
	}
	for _, option := range options {
		option(&opts)
	}

	for _, plugin := range opts.plugins {
		if err := plugin.Extend(&opts.Config); err != nil {
			return fmt.Errorf("failed to extend scaffolder: %w", err)
		}
	}

	s := &state{
		scaffoldOptions:  opts,
		deferredSymlinks: map[string]string{},
	}

	if err := s.scaffold(source, destination, ctx); err != nil {
		return fmt.Errorf("failed to scaffold: %w", err)
	}

	for dstPath := range s.deferredSymlinks {
		if err := s.applySymlinks(dstPath); err != nil {
			return fmt.Errorf("failed to apply symlink: %w", err)
		}
	}
	return nil
}

type state struct {
	scaffoldOptions
	deferredSymlinks map[string]string
}

func (s *state) scaffold(srcDir, dstDir string, ctx any) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	if err := os.Mkdir(dstDir, 0700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create directory: %w", err)
	}
nextEntry:
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		relPath, _ := filepath.Rel(s.source, srcPath) // Can't fail.
		for _, exclude := range s.Exclude {
			if matched, err := regexp.MatchString(exclude, relPath); err != nil {
				return fmt.Errorf("invalid exclude pattern %q: %w", exclude, err)
			} else if matched {
				continue nextEntry
			}
		}
		funcs := maps.Clone(s.Funcs)

		// Add a recursive function that can be used to recurse into subcontexts for files and directories.
		recursiveContext := map[string]any{}
		funcs[recurseFuncName] = func(name string, ctx any) string {
			recursiveContext[name] = ctx
			return name + "\000"
		}
		dstName, err := evaluate(srcPath, entry.Name(), ctx, funcs)
		if err != nil {
			return fmt.Errorf("failed to evaluate path name %q: %w", filepath.Join(dstDir, entry.Name()), err)
		}
		if dstName == "" {
			continue
		}

		dstPath := filepath.Join(dstDir, dstName)
		dstPath = strings.TrimSuffix(dstPath, ".tmpl")

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}

		if len(recursiveContext) == 0 {
			if err := s.scaffoldEntry(info, srcPath, dstPath, ctx, funcs); err != nil {
				return err
			}
		}
		for subEntry, subCtx := range recursiveContext {
			if err := s.scaffoldEntry(info, srcPath, filepath.Join(dstDir, subEntry), subCtx, funcs); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *state) scaffoldEntry(info fs.FileInfo, srcPath, dstPath string, ctx any, funcs template.FuncMap) error {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read symlink: %w", err)
		}

		target, err = evaluate(srcPath, target, ctx, funcs)
		if err != nil {
			return fmt.Errorf("failed to evaluate symlink target: %w", err)
		}

		// Ensure symlink is relative.
		if filepath.IsAbs(target) {
			rel, err := filepath.Rel(filepath.Dir(dstPath), target)
			if err != nil {
				return fmt.Errorf("failed to make symlink relative: %w", err)
			}
			target = rel
		}

		s.deferredSymlinks[dstPath] = target

	case info.Mode().IsDir():
		if err := os.MkdirAll(dstPath, 0700); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		for _, plugin := range s.plugins {
			if err := plugin.AfterEach(dstPath); err != nil {
				return fmt.Errorf("failed to run after: %w", err)
			}
		}
		return s.scaffold(srcPath, dstPath, ctx)

	case info.Mode().IsRegular():
		template, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		content, err := evaluate(srcPath, string(template), ctx, funcs)
		if err != nil {
			return fmt.Errorf("%s: failed to evaluate template: %w", srcPath, err)
		}
		err = os.WriteFile(dstPath, []byte(content), info.Mode())
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		for _, plugin := range s.plugins {
			if err := plugin.AfterEach(dstPath); err != nil {
				return fmt.Errorf("failed to run after: %w", err)
			}
		}

	default:
		return fmt.Errorf("%s: unsupported file type %s", srcPath, info.Mode())
	}
	return nil
}

// Recursively apply symlinks.
func (s *state) applySymlinks(path string) error {
	target, ok := s.deferredSymlinks[path]
	if !ok {
		return nil
	}
	targetPath := filepath.Clean(filepath.Join(filepath.Dir(path), target))
	if err := s.applySymlinks(targetPath); err != nil {
		return fmt.Errorf("failed to apply symlink: %w", err)
	}
	delete(s.deferredSymlinks, path)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove symlink target: %w", err)
	}
	return os.Symlink(target, path)
}

func evaluate(path, tmpl string, ctx any, funcs template.FuncMap) (string, error) {
	t, err := template.New(path).Funcs(funcs).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}
	newName := &strings.Builder{}
	err = t.Execute(newName, ctx)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return newName.String(), nil
}
