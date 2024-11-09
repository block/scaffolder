package scaffoldertest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/block/scaffolder"
)

type File struct {
	Name    string
	Mode    os.FileMode // Mode to expect - only the user permissions and symlink bits are used.
	Content string
}

func (f File) String() string {
	return fmt.Sprintf("%-32s %s %q", f.Name, f.Mode, f.Content)
}

func AssertFilesEqual(t *testing.T, dir string, expect []File) {
	actual := []File{}
	err := scaffolder.WalkDir(dir, func(path string, d os.DirEntry) error {
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		var content []byte
		if !d.IsDir() {
			content, err = os.ReadFile(path)
			if err != nil {
				return err
			}
			actual = append(actual, File{Name: rel, Mode: info.Mode() & (os.ModeSymlink | 0o700), Content: string(content)})
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actual) != len(expect) {
		expectNames := make([]string, len(expect))
		for i, file := range expect {
			expectNames[i] = file.Name
		}
		sort.Strings(expectNames)
		actualNames := make([]string, len(actual))
		for i, file := range actual {
			actualNames[i] = file.Name
		}
		sort.Strings(actualNames)
		t.Fatalf("\nExpected: %s\n  Actual: %s", strings.Join(expectNames, ", "), strings.Join(actualNames, ", "))
	}
	sort.Slice(expect, func(i, j int) bool { return expect[i].Name < expect[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name < actual[j].Name })
	for i, file := range expect {
		file.Mode &= os.ModeSymlink | 0o700
		if file != actual[i] {
			t.Errorf("\nExpected: %s\n  Actual: %s", file, actual[i])
		}
	}
}
