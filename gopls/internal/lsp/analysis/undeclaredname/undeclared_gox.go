// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package undeclaredname

import (
	"bytes"
	"fmt"
	"go/types"
	"strings"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/format"
	"github.com/goplus/gop/token"
	"github.com/goplus/gop/x/typesutil"
	"golang.org/x/tools/gop/analysis"
	"golang.org/x/tools/gop/ast/astutil"
	"golang.org/x/tools/gopls/internal/lsp/safetoken"
	"golang.org/x/tools/internal/gop/analysisinternal"
)

var GopAnalyzer = &analysis.Analyzer{
	Name:             "gopUndeclaredname",
	Doc:              Doc,
	Requires:         []analysis.IAnalyzer{Analyzer},
	Run:              gopRun,
	RunDespiteErrors: true,
}

func gopRun(pass *analysis.Pass) (interface{}, error) {
	if len(pass.GopFiles) == 0 {
		return nil, nil
	}
	for _, err := range pass.TypeErrors {
		gopRunForError(pass, err)
	}
	return nil, nil
}

func gopRunForError(pass *analysis.Pass, err types.Error) {
	var name string
	for _, prefix := range undeclaredNamePrefixes {
		if !strings.HasPrefix(err.Msg, prefix) {
			continue
		}
		name = strings.TrimPrefix(err.Msg, prefix)
	}
	if name == "" {
		return
	}
	var file *ast.File
	for _, f := range pass.GopFiles {
		if f.Pos() <= err.Pos && err.Pos < f.End() {
			file = f
			break
		}
	}
	if file == nil {
		return
	}

	// Get the path for the relevant range.
	path, _ := astutil.PathEnclosingInterval(file, err.Pos, err.Pos)
	if len(path) < 2 {
		return
	}
	ident, ok := path[0].(*ast.Ident)
	if !ok || ident.Name != name {
		return
	}

	// Undeclared quick fixes only work in function bodies.
	inFunc := false
	for i := range path {
		if _, inFunc = path[i].(*ast.FuncDecl); inFunc {
			if i == 0 {
				return
			}
			if _, isBody := path[i-1].(*ast.BlockStmt); !isBody {
				return
			}
			break
		}
	}
	if !inFunc {
		return
	}
	// Skip selector expressions because it might be too complex
	// to try and provide a suggested fix for fields and methods.
	if _, ok := path[1].(*ast.SelectorExpr); ok {
		return
	}
	tok := pass.Fset.File(file.Pos())
	if tok == nil {
		return
	}
	offset := safetoken.StartPosition(pass.Fset, err.Pos).Offset
	end := tok.Pos(offset + len(name)) // TODO(adonovan): dubious! err.Pos + len(name)??
	pass.Report(analysis.Diagnostic{
		Pos:     err.Pos,
		End:     end,
		Message: err.Msg,
	})
}

func GopSuggestedFix(fset *token.FileSet, start, end token.Pos, content []byte, file *ast.File, pkg *types.Package, info *typesutil.Info) (*analysis.SuggestedFix, error) {
	pos := start // don't use the end
	path, _ := astutil.PathEnclosingInterval(file, pos, pos)
	if len(path) < 2 {
		return nil, fmt.Errorf("no expression found")
	}
	ident, ok := path[0].(*ast.Ident)
	if !ok {
		return nil, fmt.Errorf("no identifier found")
	}

	// Check for a possible call expression, in which case we should add a
	// new function declaration.
	if len(path) > 1 {
		if _, ok := path[1].(*ast.CallExpr); ok {
			return gopNewFunctionDeclaration(path, file, pkg, info, fset)
		}
	}

	// Get the place to insert the new statement.
	insertBeforeStmt := analysisinternal.StmtToInsertVarBefore(path)
	if insertBeforeStmt == nil {
		return nil, fmt.Errorf("could not locate insertion point")
	}

	insertBefore := safetoken.StartPosition(fset, insertBeforeStmt.Pos()).Offset

	// Get the indent to add on the line after the new statement.
	// Since this will have a parse error, we can not use format.Source().
	contentBeforeStmt, indent := content[:insertBefore], "\n"
	if nl := bytes.LastIndex(contentBeforeStmt, []byte("\n")); nl != -1 {
		indent = string(contentBeforeStmt[nl:])
	}

	// Create the new local variable statement.
	newStmt := fmt.Sprintf("%s := %s", ident.Name, indent)
	return &analysis.SuggestedFix{
		Message: fmt.Sprintf("Create variable \"%s\"", ident.Name),
		TextEdits: []analysis.TextEdit{{
			Pos:     insertBeforeStmt.Pos(),
			End:     insertBeforeStmt.Pos(),
			NewText: []byte(newStmt),
		}},
	}, nil
}

func gopNewFunctionDeclaration(path []ast.Node, file *ast.File, pkg *types.Package, info *typesutil.Info, fset *token.FileSet) (*analysis.SuggestedFix, error) {
	if len(path) < 3 {
		return nil, fmt.Errorf("unexpected set of enclosing nodes: %v", path)
	}
	ident, ok := path[0].(*ast.Ident)
	if !ok {
		return nil, fmt.Errorf("no name for function declaration %v (%T)", path[0], path[0])
	}
	call, ok := path[1].(*ast.CallExpr)
	if !ok {
		return nil, fmt.Errorf("no call expression found %v (%T)", path[1], path[1])
	}

	// Find the enclosing function, so that we can add the new declaration
	// below.
	var enclosing *ast.FuncDecl
	for _, n := range path {
		if n, ok := n.(*ast.FuncDecl); ok {
			enclosing = n
			break
		}
	}
	// TODO(rstambler): Support the situation when there is no enclosing
	// function.
	if enclosing == nil {
		return nil, fmt.Errorf("no enclosing function found: %v", path)
	}

	pos := enclosing.End()

	var paramNames []string
	var paramTypes []types.Type
	// keep track of all param names to later ensure uniqueness
	nameCounts := map[string]int{}
	for _, arg := range call.Args {
		typ := info.TypeOf(arg)
		if typ == nil {
			return nil, fmt.Errorf("unable to determine type for %s", arg)
		}

		switch t := typ.(type) {
		// this is the case where another function call returning multiple
		// results is used as an argument
		case *types.Tuple:
			n := t.Len()
			for i := 0; i < n; i++ {
				name := typeToArgName(t.At(i).Type())
				nameCounts[name]++

				paramNames = append(paramNames, name)
				paramTypes = append(paramTypes, types.Default(t.At(i).Type()))
			}

		default:
			// does the argument have a name we can reuse?
			// only happens in case of a *ast.Ident
			var name string
			if ident, ok := arg.(*ast.Ident); ok {
				name = ident.Name
			}

			if name == "" {
				name = typeToArgName(typ)
			}

			nameCounts[name]++

			paramNames = append(paramNames, name)
			paramTypes = append(paramTypes, types.Default(typ))
		}
	}

	for n, c := range nameCounts {
		// Any names we saw more than once will need a unique suffix added
		// on. Reset the count to 1 to act as the suffix for the first
		// occurrence of that name.
		if c >= 2 {
			nameCounts[n] = 1
		} else {
			delete(nameCounts, n)
		}
	}

	params := &ast.FieldList{}

	for i, name := range paramNames {
		if suffix, repeats := nameCounts[name]; repeats {
			nameCounts[name]++
			name = fmt.Sprintf("%s%d", name, suffix)
		}

		// only worth checking after previous param in the list
		if i > 0 {
			// if type of parameter at hand is the same as the previous one,
			// add it to the previous param list of identifiers so to have:
			//  (s1, s2 string)
			// and not
			//  (s1 string, s2 string)
			if paramTypes[i] == paramTypes[i-1] {
				params.List[len(params.List)-1].Names = append(params.List[len(params.List)-1].Names, ast.NewIdent(name))
				continue
			}
		}

		params.List = append(params.List, &ast.Field{
			Names: []*ast.Ident{
				ast.NewIdent(name),
			},
			Type: analysisinternal.TypeExpr(file, pkg, paramTypes[i]),
		})
	}

	decl := &ast.FuncDecl{
		Name: ast.NewIdent(ident.Name),
		Type: &ast.FuncType{
			Params: params,
			// TODO(rstambler): Also handle result parameters here.
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: ast.NewIdent("panic"),
						Args: []ast.Expr{
							&ast.BasicLit{
								Value: `"unimplemented"`,
							},
						},
					},
				},
			},
		},
	}

	b := bytes.NewBufferString("\n\n")
	if err := format.Node(b, fset, decl); err != nil {
		return nil, err
	}
	return &analysis.SuggestedFix{
		Message: fmt.Sprintf("Create function \"%s\"", ident.Name),
		TextEdits: []analysis.TextEdit{{
			Pos:     pos,
			End:     pos,
			NewText: b.Bytes(),
		}},
	}, nil
}
