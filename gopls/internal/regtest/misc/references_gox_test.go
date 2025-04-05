package misc

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "golang.org/x/tools/gopls/internal/lsp/regtest"
)

func TestRefOverloadDeclAnony(t *testing.T) {
	const files = `
-- go.mod --
module mod.com

go 1.19
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
-- gop_autogen.go --
package main

import "fmt"

const _ = true
func add__0(a int, b int) int {
	return a + b
}
func add__1(a string, b string) string {
	return a + b
}
func main() {
	fmt.Println(add__0(1, 2))
	fmt.Println(add__1("Hello", "World"))
}
`
	testCases := []refTest{
		{
			"def.gop", "add", []string{
				"def.gop 0:5-0:8",   // overload decl
				"test.gop 0:8-0:11", // overload int call
				"test.gop 1:8-1:11", // overload string call
			},
		},
	}
	runFindRefTest(t, files, testCases)
}

func TestRefOverloadDeclNamedAndAnony(t *testing.T) {
	const files = `
-- go.mod --
module mod.com

go 1.19
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
-- gop_autogen.go --
package main

import "fmt"

const _ = true
const Gopo_mul = "mulInt,,mulFloat"
func mulInt(a int, b int) int {
	return a * b
}
func mul__1(a string, b string) string {
	return a + b
}
func mulFloat(a float64, b float64) float64 {
	return a * b
}
func main() {
	fmt.Println(mulInt(100, 7))
	fmt.Println(mul__1("Hello", "World"))
	fmt.Println(mulFloat(1.2, 3.14))
}
`

	testCases := []refTest{
		// goxls: overload reference
		{
			"def.gop", `func (mul) = \(`,
			[]string{
				"def.gop 8:5-8:8",   // overload defintion
				"test.gop 0:8-0:11", // overload int call
				"test.gop 1:8-1:11", // overload string call
				"test.gop 2:8-2:11", // overload float call
			},
		},
		// goxls: overload member reference
		{
			"def.gop", `func mul = \(\n\s+(mulInt)`,
			[]string{
				"def.gop 0:5-0:11",  // mulInt
				"def.gop 9:4-9:10",  // overload mulInt
				"test.gop 0:8-0:11", // use overload mulInt
			},
		},
	}
	runFindRefTest(t, files, testCases)
}

func TestRefOverloadDeclMethod(t *testing.T) {
	const files = `
-- go.mod --
module mod.com

go 1.19
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
var b = a.mul(100)
var c = a.mul(a)
-- gop_autogen.go --
package main

const _ = true

type foo struct {
}

const Gopo_foo_mul = ".mulInt,.mulFoo"
func (a *foo) mulInt(b int) *foo {
	return a
}
func (a *foo) mulFoo(b *foo) *foo {
	return a
}

var a *foo
var b = a.mulInt(100)
var c = a.mulFoo(a)

func main() {
}
`
	testCases := []refTest{
		{
			"def.gop", `func \(foo\)\.(mul) = \(`,
			[]string{
				"def.gop 8:11-8:14",
				"test.gop 1:10-1:13",
				"test.gop 2:10-2:13",
			},
		},
		{
			"def.gop", `\(foo\)\.(mulInt)`,
			[]string{
				"def.gop 2:14-2:20",
				"def.gop 9:10-9:16",
				"test.gop 1:10-1:13",
			},
		},
	}
	runFindRefTest(t, files, testCases)
}

type refTest struct {
	defineFile   string
	defineLocReg string
	refLocs      []string
}

func runFindRefTest(t *testing.T, files string, testCase []refTest) {
	Run(t, files, func(t *testing.T, env *Env) {
		for _, test := range testCase {
			env.OpenFile(test.defineFile)
			loc := env.GoToDefinition(env.RegexpSearch(test.defineFile, test.defineLocReg))
			refs, err := env.Editor.References(env.Ctx, loc)
			if err != nil {
				t.Fatalf("references on (*s).Error failed: %v", err)
			}
			var buf strings.Builder
			for _, ref := range refs {
				fmt.Fprintf(&buf, "%s %s\n", env.Sandbox.Workdir.URIToPath(ref.URI), ref.Range)
			}
			got := buf.String()
			want := strings.Join(test.refLocs, "\n") + "\n"
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("unexpected references on (*s).Error (-want +got):\n%s", diff)
			}
		}
	})
}
