# A general-purpose project scaffolding library and tool inspired by [cookiecutter](https://github.com/cookiecutter/cookiecutter)

[![stability-experimental](https://img.shields.io/badge/stability-experimental-orange.svg)](https://github.com/mkenney/software-guides/blob/master/STABILITY-BADGES.md#experimental) [![Go Reference](https://pkg.go.dev/badge/github.com/block/scaffolder.svg)](https://pkg.go.dev/github.com/block/scaffolder) [![CI](https://github.com/block/scaffolder/actions/workflows/ci.yml/badge.svg)](https://github.com/block/scaffolder/actions/workflows/ci.yml)

<!-- Note that everything after in this README is used to generate the --help documentation for the command line tool. -->

---

Scaffolder recursively evaluates a directory hierarchy of templates into a destination using JSON as the template context. It follows these rules:

- Templates are evaluated using the Go template engine.
- Both path names and file contents are evaluated.
- If a file name ends with ".tmpl", the ".tmpl" suffix will be removed.
- If a file or directory name evalutes to the empty string it will be excluded.
- If a file named "template.js" exists in the root of the template directory,
  all functions defined in this file will be available as Go template functions.
- Directory and file names in templates can be expanded multiple times
  using the "push" function. This function takes two arguments, the
  file/directory name and the context to use when evaluating templates within
  the file/directory.
- The following functions are available as Go template functions: "snake", "screamingSnake", "camel", "lowerCamel",
  "kebab", "screamingKebab", "upper", "lower", "title",

## Usage

```
scaffolder --json '{...}' <source> <dest>
scaffolder --json file.json <source> <dest>
```

Where `<source>` can be any URL supported by [getit](https://github.com/block/getit).

eg.

```
scaffolder --json '{"name": "Bob"}' git+https://github.com/foo/bar ./dest
```

## Detecting overwritten changes

Re-scaffolding overwrites files, including any changes made to them since
they were scaffolded. With `--checksum`, scaffolder records a checksum of every
file it writes in `.scaffolder.sum` in the destination. Commit this file, and
pass `--checksum` on every run.

On later runs, scaffolder still overwrites files, but reports each one that had
local changes. It also deletes files listed in `.scaffolder.sum` that the
template no longer produces, and reports them:

```
main.go: overwrote local changes
old.go: deleted
legacy.go: deleted with local changes
```

`.scaffolder.sum` uses the `sha256sum` format, so `sha256sum -c .scaffolder.sum`
checks it by hand.

## Example

For example, given the following files and directories as the template:

```
template/
  {{ range .modules }}{{ push .name  . }}{{ end }}/
	file.txt
```

And the context "context.json":

```json
{
  "modules": [
	{"name": "module1", "path": "path1"},
	{"name": "module2", "path": "path2"}
  ]
}
```

Running scaffolder with:

```
scaffolder --json context.json ./template ./dest
```

The output "dest" directory will contain the following files and directories:

```
module1/
  file.txt
module2/
  file.txt
```
