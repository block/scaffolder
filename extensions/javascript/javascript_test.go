package javascript_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/block/scaffolder"
	"github.com/block/scaffolder/extensions/javascript"
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
		scaffolder.Extend(javascript.Extension("template.js")),
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

func TestExtensionAbsoluteScriptPath(t *testing.T) {
	scriptDir := t.TempDir()
	scriptFile := filepath.Join(scriptDir, "custom.js")
	err := os.WriteFile(scriptFile, []byte(`
function reverse(s) {
  return s.split("").reverse().join("");
}
function hello(c) {
  return goHello(c);
}
`), 0600)
	assert.NoError(t, err)

	dest := t.TempDir()
	err = scaffolder.Scaffold("testdata", dest, Context{
		Name: "Alice",
	},
		scaffolder.Exclude("^go.mod$", "^template\\.js$"),
		scaffolder.Extend(javascript.Extension(scriptFile)),
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
