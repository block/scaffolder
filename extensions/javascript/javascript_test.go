package javascript

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/block/scaffolder"
	"github.com/block/scaffolder/scaffoldertest"
)

type Context struct {
	Name string
}

func TestExtension(t *testing.T) {
	dest := t.TempDir()
	err := scaffolder.Scaffold("testdata", dest, Context{
		Name: "Alice",
	},
		scaffolder.Exclude("^go.mod$"),
		scaffolder.Extend(Extension("template.js")),
		scaffolder.Functions(scaffolder.FuncMap{
			"goHello": func(c Context) string {
				return "Hello " + c.Name
			},
		}),
	)
	assert.NoError(t, err)
	scaffoldertest.AssertFilesEqual(t, dest, []scaffoldertest.File{
		{Name: "sdrawkcab", Mode: 0600},
		{Name: "hello.txt", Mode: 0600, Content: "Hello Alice"},
	})
}
