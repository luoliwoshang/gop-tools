package misc

import (
	"testing"

	. "golang.org/x/tools/gopls/internal/lsp/regtest"
)

const anonyOverload = `
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
println add(100, 7)
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
	fmt.Println(add__0(100, 7))
}
`

func TestAnonyOverload(t *testing.T) {
	testCases := []match{
		{`test.gop`, `println (add)\(100, 7\)`, "def.gop", `func\(a, b int\) int`},
	}
	runGoToDefinitionTest(t, anonyOverload, testCases)
}

const overloadMixAnonyAndNamed = `
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
}
`

func TestOverloadMixAnonyAndNamed(t *testing.T) {
	testCases := []match{
		{`test.gop`, `println (mul)\(100, 7\)`, "def.gop", `func (mulInt)\(a, b int\) int`},
	}
	runGoToDefinitionTest(t, overloadMixAnonyAndNamed, testCases)
}

const overloadMethod = `
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
var c = a.mul(100)
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
var c = a.mulInt(100)

func main() {
}
`

func TestOverloadMethod(t *testing.T) {
	testCases := []match{
		{`test.gop`, `var c = a.(mul)\(100\)`, "def.gop", `func \(a \*foo\) (mulInt)\(b int\) \*foo`},
	}
	runGoToDefinitionTest(t, overloadMethod, testCases)
}

const overloadFromGo = `
-- go.mod --
module mod.com

go 1.19
-- def.go --
package main

const GopPackage = true

type N struct {
}

func (m *N) OnKey__0(a string, fn func()) {
	fn()
}

func (m *N) OnKey__1(a string, fn func(key string)) {
	fn(a)
}

func (m *N) OnKey__2(a []string, fn func()) {
	fn()
}
-- test.gop --
n := &N{}

n.onKey("hello", func() {
	println("hello world")
})
-- gop_autogen.go --
package main

import "fmt"

const _ = true
func main() {
	n := &N{}
	n.OnKey__0("hello", func() {
		fmt.Println("hello world")
	})
}
`

func TestOverloadFromGo(t *testing.T) {
	testCases := []match{
		{`test.gop`, `onKey`, "def.go", `OnKey__0`},
	}
	runGoToDefinitionTest(t, overloadFromGo, testCases)
}

const overloadCrossPkg = `
-- go.mod --
module mod.com

go 1.19
-- lib/lib.gop --
package lib

func Add = (
	func(a, b int) int {
		return a + b
	}
	func(a, b string) string {
		return a + b
	}
)

-- lib/gop_autogen.go --
package lib

const GopPackage = true
const _ = true
func Add__0(a int, b int) int {
	return a + b
}
func Add__1(a string, b string) string {
	return a + b
}

-- main.gop --
import (
	"mod.com/lib"
)

println lib.Add(100, 7)
println lib.Add("Hello", "World")

-- gop_autogen.go --
package main

import (
	"fmt"
	"mod.com/lib"
)

const _ = true
func main() {
	fmt.Println(lib.Add__0(100, 7))
	fmt.Println(lib.Add__1("Hello", "World"))
}
`

// Test cross package 's overload definition
func TestOverloadCrossPkg(t *testing.T) {
	testCases := []match{
		{`main.gop`, `println lib.(Add)\(100, 7\)`, "lib/lib.gop", `func\(a, b int\) int`},
		{`main.gop`, `println lib.(Add)\("Hello", "World"\)`, "lib/lib.gop", `func\(a, b string\) string`},
	}
	runGoToDefinitionTest(t, overloadCrossPkg, testCases)
}

type match struct {
	findLocFile string
	findLocReg  string
	wantFile    string
	wantLocReg  string
}

func runGoToDefinitionTest(t *testing.T, files string, testCases []match) {
	Run(t, files, func(t *testing.T, env *Env) {
		for _, test := range testCases {
			env.OpenFile(test.findLocFile)
			loc := env.GoToDefinition(env.RegexpSearch(test.findLocFile, test.findLocReg))
			name := env.Sandbox.Workdir.URIToPath(loc.URI)
			if name != test.wantFile {
				t.Errorf("GoToDefinition: got file %q, want %q", name, test.wantFile)
			}
			if want := env.RegexpSearch(test.wantFile, test.wantLocReg); loc != want {
				t.Errorf("GoToDefinition: got location %v, want %v", loc, want)
			}
		}
	})
}
