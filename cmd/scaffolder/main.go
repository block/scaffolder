package main

import (
	"encoding/json"
	"html/template"
	"os"
	"reflect"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/iancoleman/strcase"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/block/scaffolder"
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

	err = scaffolder.Scaffold(cli.Template, cli.Dest, context, scaffolder.Functions(template.FuncMap{
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
	}), scaffolder.Extend(javascript.Extension("template.js")))
	kctx.FatalIfErrorf(err)
}
