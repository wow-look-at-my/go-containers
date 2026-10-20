package blocking

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
)

// forbiddenWaits stand in for a wakeup; a goroutine that waits parks instead.
var forbiddenWaits = set.Of("runtime.Gosched", "time.Sleep", "time.After", "time.Tick", "time.NewTicker")

// No library file in this module waits by yielding, sleeping or polling a clock.
func TestNoLibraryCodeSleepsOrYields(t *testing.T) {
	root := filepath.Join("..", "..")
	var found []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".claude" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		found = append(found, forbiddenCalls(t, path)...)
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, found)
}

func forbiddenCalls(t *testing.T, path string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	require.NoError(t, err)
	imported := map[string]string{}
	for _, spec := range file.Imports {
		importPath := strings.Trim(spec.Path.Value, `"`)
		name := filepath.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imported[name] = importPath
	}
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		call := imported[pkg.Name] + "." + selector.Sel.Name
		if forbiddenWaits.Contains(call) {
			found = append(found, fset.Position(selector.Pos()).String()+": "+call)
		}
		return true
	})
	return found
}

// The scan reports a yield it is shown, so an empty result above means something.
func TestForbiddenCallsFindsAYield(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spin.go")
	source := "package spin\n\nimport \"runtime\"\n\nfunc spin(ready func() bool) {\n\tfor !ready() {\n\t\truntime.Gosched()\n\t}\n}\n"
	require.NoError(t, os.WriteFile(path, []byte(source), 0o644))
	found := forbiddenCalls(t, path)
	require.Len(t, found, 1)
	assert.Contains(t, found[0], "runtime.Gosched")
}
