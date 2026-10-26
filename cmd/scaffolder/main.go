package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/iancoleman/strcase"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/block/scaffolder"
	"github.com/block/scaffolder/extensions/checksum"
	"github.com/block/scaffolder/extensions/javascript"
)

var version = "dev"

// JSONSource can be either a file path or a direct JSON string.
type JSONSource struct {
	Value any
}

func (j *JSONSource) UnmarshalText(text []byte) error {
	s := string(text)
	trimmed := strings.TrimSpace(s)
	// If it looks like JSON (starts with { or [), parse directly
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return json.Unmarshal(text, &j.Value)
	}
	// Otherwise treat as a file path
	f, err := os.Open(s)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&j.Value)
}

var cli struct {
	Version  kong.VersionFlag `help:"Show version."`
	JSON     *JSONSource      `help:"JSON file path or direct JSON string."`
	Script   string           `help:"Path to a JavaScript file to use for template functions." type:"existingfile" optional:""`
	Checksum bool             `help:"Record checksums in .scaffolder.sum and report local changes that are overwritten or deleted. Use it on every run, or .scaffolder.sum goes stale."`
	Template string           `arg:"" help:"Template directory." type:"existingdir"`
	Dest     string           `arg:"" help:"Destination directory to scaffold."`
}

func main() {
	kctx := kong.Parse(&cli, kong.Vars{"version": version}, kong.Description(scaffolder.About()))
	var context any
	if cli.JSON != nil {
		context = cli.JSON.Value
	}

	err := os.MkdirAll(cli.Dest, 0750)
	kctx.FatalIfErrorf(err)

	options := []scaffolder.Option{scaffolder.Functions(template.FuncMap{
		"snake":          strcase.ToSnake,
		"screamingSnake": strcase.ToScreamingSnake,
		"camel":          strcase.ToCamel,
		"lowerCamel":     strcase.ToLowerCamel,
		"kebab":          strcase.ToKebab,
		"screamingKebab": strcase.ToScreamingKebab,
		"upper":          strings.ToUpper,
		"lower":          strings.ToLower,
		"title":          cases.Title(language.English).String,
		"typename": func(v any) string {
			return reflect.Indirect(reflect.ValueOf(v)).Type().Name()
		},
	}), scaffolder.Extend(javascript.Extension(scriptPath()))}
	if cli.Checksum {
		// Registered last so it records files after other extensions have modified them.
		options = append(options, scaffolder.Extend(checksum.Extension(printChange)))
	}

	err = scaffolder.Scaffold(os.DirFS(cli.Template), cli.Dest, context, options...)
	kctx.FatalIfErrorf(err)
}

func printChange(change checksum.Change) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", change.Kind, change.Path)
}

// scriptPath returns the script relative to the template when it is inside it,
// so it is excluded from the output.
func scriptPath() string {
	if cli.Script == "" {
		return "template.js"
	}
	template, err := filepath.Abs(cli.Template)
	if err != nil {
		return cli.Script
	}
	if rel, err := filepath.Rel(template, cli.Script); err == nil && filepath.IsLocal(rel) {
		return filepath.ToSlash(rel)
	}
	return cli.Script
}
