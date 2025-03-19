# A general-purpose project scaffolding library and tool inspired by [cookiecutter](https://github.com/cookiecutter/cookiecutter)

[![stability-experimental](https://img.shields.io/badge/stability-experimental-orange.svg)](https://github.com/mkenney/software-guides/blob/master/STABILITY-BADGES.md#experimental) [![Go Reference](https://pkg.go.dev/badge/github.com/block/scaffolder.svg)](https://pkg.go.dev/github.com/block/scaffolder) [![CI](https://github.com/block/scaffolder/actions/workflows/ci.yml/badge.svg)](https://github.com/block/scaffolder/actions/workflows/ci.yml)

<!-- Note that everything after in this README is used to generate the --help documentation for the command line tool. -->

---

Scaffolder evaluates the scaffolding files at the given desScaffolder evaluates a template of directories and files into a destination using JSON context using the following
rules:

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


For example, given the following files and directories as the template:

	template/
	  {{ range .modules }}{{ push .name  . }}{{ end }}/
	    file.txt

And the context "context.json":

    {
      "modules": [
        {"name": "module1", "path": "path1"},
        {"name": "module2", "path": "path2"}
      ]
    }

Running scaffolder with:

    scaffolder --json context.json --template template --dest dest

The output "dest" directory will contain the following files and directories:

    module1/
      file.txt
    module2/
      file.txt
