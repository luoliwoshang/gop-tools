package misc

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "golang.org/x/tools/gopls/internal/lsp/regtest"
)

func TestReferencesOnOverloadDecl1(t *testing.T) {
	const files = `
-- go.mod --
module mod.com

go 1.12
-- def.gop --
func add = (
	func(a, b int) int {
		return a + b
	}
	func(a, b string) string {
		return a + b
	}
)
-- test.gop --
println add(1,2)
println add("Hello", "World")
`
	Run(t, files, func(t *testing.T, env *Env) {
		env.OpenFile("def.gop")
		loc := env.GoToDefinition(env.RegexpSearch("def.gop", `add`))
		refs, err := env.Editor.References(env.Ctx, loc)
		if err != nil {
			t.Fatalf("references on (*s).Error failed: %v", err)
		}
		var buf strings.Builder
		for _, ref := range refs {
			fmt.Fprintf(&buf, "%s %s\n", env.Sandbox.Workdir.URIToPath(ref.URI), ref.Range)
		}
		got := buf.String()
		want := "def.gop 0:5-0:8\n" + // overload decl
			"test.gop 0:8-0:11\n" + // overload int call
			"test.gop 1:8-1:11\n" // overload string call
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected references on (*s).Error (-want +got):\n%s", diff)
		}
	})
}
func TestReferencesOnOverloadDecl2(t *testing.T) {
	const files = `
-- go.mod --
module mod.com

go 1.12
-- def.gop --
func mulInt(a, b int) int {
    return a * b
}

func mulFloat(a, b float64) float64 {
    return a * b
}

func mul = (
    mulInt
    func(a, b string) string {
        return a + b
    }
    mulFloat
)
-- test.gop --
println mul(100, 7)
println mul("Hello", "World")
println mul(1.2, 3.14)
`
	Run(t, files, func(t *testing.T, env *Env) {
		env.OpenFile("def.gop")
		loc := env.GoToDefinition(env.RegexpSearch("def.gop", `func (mul) = \(`))
		refs, err := env.Editor.References(env.Ctx, loc)
		if err != nil {
			t.Fatalf("references on (*s).Error failed: %v", err)
		}
		var buf strings.Builder
		for _, ref := range refs {
			fmt.Fprintf(&buf, "%s %s\n", env.Sandbox.Workdir.URIToPath(ref.URI), ref.Range)
		}
		got := buf.String()
		want := "def.gop 8:5-8:8\n" + // overload defintion
			"def.gop 9:4-9:10\n" + // mutInt
			"def.gop 13:4-13:12\n" + // mutFloat
			"test.gop 0:8-0:11\n" + // overload int call
			"test.gop 1:8-1:11\n" + // overload string call
			"test.gop 2:8-2:11\n" // overload float call
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected references on (*s).Error (-want +got):\n%s", diff)
		}
	})
}

func TestReferencesOnOverloadDecl3(t *testing.T) {
    const files = `
-- go.mod --
module mod.com

go 1.12
-- def.gop --
type foo struct {
}
func (a *foo) mulInt(b int) *foo {
    return a
}
func (a *foo) mulFoo(b *foo) *foo {
    return a
}
func (foo).mul = (
    (foo).mulInt
    (foo).mulFoo
)
-- test.gop --
var a *foo
var c = a.mul(100)
`
    Run(t, files, func(t *testing.T, env *Env) {
        env.OpenFile("def.gop")
        loc := env.GoToDefinition(env.RegexpSearch("def.gop", `func \(foo\)\.(mul) = \(`))
        refs, err := env.Editor.References(env.Ctx, loc)
        if err != nil {
            t.Fatalf("references on (*s).Error failed: %v", err)
        }
        var buf strings.Builder
        for _, ref := range refs {
            fmt.Fprintf(&buf, "%s %s\n", env.Sandbox.Workdir.URIToPath(ref.URI), ref.Range)
        }
        got := buf.String()
        want := "def.gop 8:11-8:14\n" +
            "test.gop 1:10-1:13\n"
        if diff := cmp.Diff(want, got); diff != "" {
            t.Errorf("unexpected references on (*s).Error (-want +got):\n%s", diff)
        }
    })
}