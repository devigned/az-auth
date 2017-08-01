package cgo

import (
	"go/ast"
	"go/token"
	"go/types"
)

// ArrayWrapper is a wrapper for the
type Slice struct {
	elem types.Type
}

// NewSliceWrapper wraps types.Slice to provide a consistent comparison
func NewSlice(elem types.Type) Slice {
	return Slice{
		elem: elem,
	}
}

// Underlying returns the underlying type of the Slice (types.Type)
func (s Slice) Underlying() types.Type {
	return s
}

// Underlying returns the string representation of the type (types.Type)
func (s Slice) String() string {
	return types.TypeString(types.NewSlice(s.elem), nil)
}

// ToAst returns the go/ast representation of the CGo wrapper of the Slice type
func (s Slice) ToAst() []ast.Decl {
	return []ast.Decl{
		s.NewAst(),
		s.StringAst(),
		s.ItemAst(),
		s.ItemSetAst(),
		s.ItemAppendAst(),
		s.DestroyAst(),
	}
}

func (s Slice) GoName() string {
	return "[]" + s.elem.String()
}

func (s Slice) CGoName() string {
	return "slice_of_" + s.elem.String()
}

// NewAst produces the []ast.Decl to construct a slice type and increment it's reference count
func (s Slice) NewAst() ast.Decl {
	functionName := s.CGoName() + "_new"
	goType := &ast.ArrayType{
		Elt: NewIdent(s.elem.String()),
	}
	return NewAst(functionName, goType)
}

// StringAst produces the []ast.Decl to provide a string representation of the slice
func (s Slice) StringAst() ast.Decl {
	functionName := s.CGoName() + "_str"
	goTypeIdent := NewIdent(s.GoName())
	return StringAst(functionName, goTypeIdent)
}

// DestroyAst produces the []ast.Decl to destruct a slice type and decrement it's reference count
func (s Slice) DestroyAst() ast.Decl {
	return DestroyAst(s.CGoName() + "_destroy")
}

func (s Slice) ItemAst() ast.Decl {
	functionName := s.CGoName() + "_item"
	selfIdent := NewIdent("self")
	indexIdent := NewIdent("i")
	indexTypeIdent := NewIdent("int")
	goTypeIdent := NewIdent(s.GoName())
	elementTypeIdent := NewIdent(s.elem.String())
	itemsIdent := NewIdent("items")

	castExpression := CastUnsafePtr(DeRef(goTypeIdent), selfIdent)

	funcDecl := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: ExportComments(functionName),
		},
		Name: NewIdent(functionName),
		Type: &ast.FuncType{
			Params: InstanceMethodParams(
				[]*ast.Field{
					{
						Names: []*ast.Ident{indexIdent},
						Type:  indexTypeIdent,
					},
				}...),
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: elementTypeIdent},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						itemsIdent,
					},
					Rhs: []ast.Expr{
						castExpression,
					},
					Tok: token.DEFINE,
				},
				Return(&ast.IndexExpr{
					X: &ast.ParenExpr{
						X: &ast.StarExpr{
							X: itemsIdent,
						},
					},
					Index: indexIdent,
				}),
			},
		},
	}

	return funcDecl
}

func (s Slice) ItemSetAst() ast.Decl {
	functionName := s.CGoName() + "_item_set"
	selfIdent := NewIdent("self")
	indexIdent := NewIdent("i")
	indexTypeIdent := NewIdent("int")
	goTypeIdent := NewIdent(s.GoName())
	elementTypeIdent := NewIdent(s.elem.String())
	itemsIdent := NewIdent("items")
	itemIdent := NewIdent("item")

	castExpression := CastUnsafePtr(DeRef(goTypeIdent), selfIdent)

	funcDecl := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: ExportComments(functionName),
		},
		Name: NewIdent(functionName),
		Type: &ast.FuncType{
			Params: InstanceMethodParams(
				[]*ast.Field{
					{
						Names: []*ast.Ident{indexIdent},
						Type:  indexTypeIdent,
					},
					{
						Names: []*ast.Ident{itemIdent},
						Type:  elementTypeIdent,
					},
				}...),
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						itemsIdent,
					},
					Rhs: []ast.Expr{
						castExpression,
					},
					Tok: token.DEFINE,
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.IndexExpr{
							X: &ast.ParenExpr{
								X: &ast.StarExpr{
									X: itemsIdent,
								},
							},
							Index: indexIdent,
						},
					},
					Rhs: []ast.Expr{
						itemIdent,
					},
					Tok: token.ASSIGN,
				},
			},
		},
	}

	return funcDecl
}

// ItemAppendAst returns a function declaration which appends an item to the slice
func (s Slice) ItemAppendAst() ast.Decl {
	functionName := s.CGoName() + "_item_append"
	selfIdent := NewIdent("self")
	goTypeIdent := NewIdent(s.GoName())
	elementTypeIdent := NewIdent(s.elem.String())
	itemsIdent := NewIdent("items")
	itemIdent := NewIdent("item")

	castExpression := CastUnsafePtr(DeRef(goTypeIdent), selfIdent)

	funcDecl := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: ExportComments(functionName),
		},
		Name: NewIdent(functionName),
		Type: &ast.FuncType{
			Params: InstanceMethodParams(
				[]*ast.Field{
					{
						Names: []*ast.Ident{itemIdent},
						Type:  elementTypeIdent,
					},
				}...),
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						itemsIdent,
					},
					Rhs: []ast.Expr{
						castExpression,
					},
					Tok: token.DEFINE,
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						DeRef(itemsIdent),
					},
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: NewIdent("append"),
							Args: []ast.Expr{
								DeRef(itemsIdent),
								itemIdent,
							},
						},
					},
					Tok: token.ASSIGN,
				},
			},
		},
	}

	return funcDecl
}
