package bind

import (
	"fmt"
	"go/ast"

	"bufio"
	"github.com/devigned/veil/cgo"
	"github.com/devigned/veil/core"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
)

var (
	registry = map[string]func(*cgo.Package) Bindable{"py3": NewPy3Binder}
)

// Bindable is the interface for any object that will create a binding for a golang.Package
type Bindable interface {
	Bind(outDir string) error
}

// NewBinder is a factory method for creating a new binder for a given target
func NewBinder(pkg *cgo.Package, target string) (Bindable, error) {
	bindable, ok := registry[target]

	if !ok {
		return nil, core.NewSystemError(fmt.Sprintf("I don't know how to create a binder for %s", target))
	}

	cgoAst(pkg)
	return bindable(pkg), nil
}

// cgoAst generates a map of file names and io.Writers which are the cgo substrate for targets to bind.
// The cgo layer is intended to normalize types from Go into more standard C types and provide a standard
// layer to build FFI language bindings.
func cgoAst(pkg *cgo.Package) *ast.File {

	//printPracticeAst()
	declarations := []ast.Decl{
		cgo.Imports("C"),
		cgo.Imports("fmt", "sync", "unsafe", "strconv", "strings", "os"),
	}

	for _, item := range pkg.ToCgoAst() {
		declarations = append(declarations, item)
	}

	declarations = append(declarations, cgo.MainFunc())

	mainFile := &ast.File{
		Name: &ast.Ident{
			Name: "main",
		},
		Decls: declarations,
	}

	// Print the AST.
	// ast.Print(&token.FileSet{}, mainFile)
	writer := bufio.NewWriter(os.Stdout)
	printer.Fprint(writer, &token.FileSet{}, mainFile)
	defer writer.Flush()
	return mainFile
}

func printPracticeAst() {
	src := `

	package main

	import "C"

func cgo_func_0x3129483107_ass_item(self cgo_type_0x3129483107, i int, v string) {
	arr := (*[]string)(unsafe.Pointer(self))
	(*arr)[i] = v
}
}
		`

	fset := token.NewFileSet() // positions are relative to fset
	f, _ := parser.ParseFile(fset, "src.go", src, parser.ParseComments)
	ast.Print(fset, f)
}
