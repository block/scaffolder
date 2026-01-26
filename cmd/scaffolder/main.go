package main

import (
	"encoding/json"
	"html/template"
	"os"
	"reflect"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/iancoleman/strcase"

	"github.com/block/scaffolder"
	"github.com/block/scaffolder/extensions/javascript"
)

var version = "dev"

var cli struct {
	Version  kong.VersionFlag `help:"Show version."`
	JSON     *os.File         `help:"JSON file containing the context to use."`
	Template string           `arg:"" help:"Template directory." type:"existingdir"`
	Dest     string           `arg:"" help:"Destination directory to scaffold."`
}

func main() {
	kctx := kong.Parse(&cli, kong.Vars{"version": version}, kong.Description(scaffolder.About()))
	var context any
	if cli.JSON != nil {
		err := json.NewDecoder(cli.JSON).Decode(&context)
		kctx.FatalIfErrorf(err, "failed to decode JSON")
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
		"title":          strings.Title,
		"typename": func(v any) string {
			return reflect.Indirect(reflect.ValueOf(v)).Type().Name()
		},
	}), scaffolder.Extend(javascript.Extension("template.js")))
	kctx.FatalIfErrorf(err)
}
