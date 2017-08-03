package cgo

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

const (
	INCREMENT_REF_FUNC_NAME  = "cgo_incref"
	DECREMENT_REF_FUNC_NAME  = "cgo_decref"
	COBJECT_STRUCT_TYPE_NAME = "cobject"
	REFS_VAR_NAME            = "refs"
	REFS_STRUCT_FIELD_NAME   = "refs"
)

var (
	unsafePointer = &ast.SelectorExpr{
		X:   NewIdent("unsafe"),
		Sel: NewIdent("Pointer"),
	}

	uintptr = NewIdent("uintptr")
)

// CObjectStruct produces an AST struct which will represent a C exposed Object
func CObjectStruct() ast.Decl {
	return &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: NewIdent(COBJECT_STRUCT_TYPE_NAME),
				Type: &ast.StructType{
					Fields: &ast.FieldList{
						List: []*ast.Field{
							{
								Names: []*ast.Ident{NewIdent("ptr")},
								Type: &ast.SelectorExpr{
									X:   NewIdent("unsafe"),
									Sel: NewIdent("Pointer"),
								},
							},
							{
								Names: []*ast.Ident{NewIdent("cnt")},
								Type:  NewIdent("int32"),
							},
						},
					},
				},
			},
		},
	}
}

// RefsStruct produces an AST struct which will keep track of references to pointers used by the host CFFI
func RefsStruct() ast.Decl {
	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: NewIdent(REFS_VAR_NAME),
				Type: &ast.StructType{
					Fields: &ast.FieldList{
						List: []*ast.Field{
							{
								Type: &ast.SelectorExpr{
									X:   NewIdent("sync"),
									Sel: NewIdent("Mutex"),
								},
							},
							{
								Names: []*ast.Ident{NewIdent("next")},
								Type:  NewIdent("int32"),
							},
							{
								Names: []*ast.Ident{NewIdent(REFS_STRUCT_FIELD_NAME)},
								Type: &ast.MapType{
									Key: &ast.SelectorExpr{
										X:   NewIdent("unsafe"),
										Sel: NewIdent("Pointer"),
									},
									Value: NewIdent("int32"),
								},
							},
							{
								Names: []*ast.Ident{NewIdent("ptrs")},
								Type: &ast.MapType{
									Key:   NewIdent("int32"),
									Value: NewIdent(COBJECT_STRUCT_TYPE_NAME),
								},
							},
						},
					},
				},
			},
		},
	}
}

// NewAst produces the []ast.Decl to construct a slice type and increment it's reference count
func NewAst(functionName string, goType ast.Expr) ast.Decl {
	localVarIdent := NewIdent("o")
	target := &ast.UnaryExpr{
		Op: token.AND,
		X:  localVarIdent,
	}

	funcDecl := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: ExportComments(functionName),
		},
		Name: NewIdent(functionName),
		Type: &ast.FuncType{
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: uintptr},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				DeclareVar(localVarIdent, goType),
				IncrementRefCall(target),
				Return(UintPtr(UnsafePointerToTarget(target))),
			},
		},
	}

	return funcDecl
}

// StringAst produces the []ast.Decl to provide a string representation of the slice
func StringAst(functionName string, goType ast.Expr) ast.Decl {
	selfIdent := NewIdent("self")
	stringIdent := NewIdent("string")

	castExpression := CastUnsafePtr(DeRef(goType), selfIdent)
	deRef := DeRef(castExpression)
	sprintf := FormatSprintf("%#v", deRef)

	funcDecl := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: ExportComments(functionName),
		},
		Name: NewIdent(functionName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{selfIdent},
						Type:  unsafePointer,
					},
				},
			},
			Results: &ast.FieldList{
				List: []*ast.Field{
					{Type: stringIdent},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				Return(sprintf),
			},
		},
	}

	return funcDecl
}

// DestroyAst produces the []ast.Decl to destruct a slice type and decrement it's reference count
func DestroyAst(functionName string) ast.Decl {
	funcDecl := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: ExportComments(functionName),
		},
		Name: NewIdent(functionName),
		Type: &ast.FuncType{
			Params: InstanceMethodParams(),
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				DecrementRefCall(NewIdent("self")),
			},
		},
	}

	return funcDecl
}

func Init() ast.Decl {
	refsVar := NewIdent(REFS_VAR_NAME)
	refsField := NewIdent(REFS_STRUCT_FIELD_NAME)
	return &ast.FuncDecl{
		Name: NewIdent("init"),
		Type: &ast.FuncType{},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// refs.Lock()
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   refsVar,
							Sel: NewIdent("Lock"),
						},
					},
				},
				// refs.next = -1
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   refsVar,
							Sel: NewIdent("next"),
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.BasicLit{
							Value: "-1",
							Kind:  token.INT,
						},
					},
				},
				// refs.refs = make(map[unsafe.Pointer]int32
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   refsVar,
							Sel: refsField,
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: NewIdent("make"),
							Args: []ast.Expr{
								&ast.MapType{
									Key:   unsafePointer,
									Value: NewIdent("int32"),
								},
							},
						},
					},
				},
				// refs.ptrs = make(map[int32]cobject)
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   refsVar,
							Sel: NewIdent("ptrs"),
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: NewIdent("make"),
							Args: []ast.Expr{
								&ast.MapType{
									Key:   NewIdent("int32"),
									Value: NewIdent(COBJECT_STRUCT_TYPE_NAME),
								},
							},
						},
					},
				},
				// refs.Unlock()
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   refsVar,
							Sel: NewIdent("Unlock"),
						},
					},
				},
			},
		},
	}
}

func DecrementRef() ast.Decl {
	ptr := NewIdent("ptr")
	refsType := NewIdent(REFS_VAR_NAME)
	refsField := NewIdent(REFS_STRUCT_FIELD_NAME)
	num := NewIdent("num")
	ok := NewIdent("ok")
	s := NewIdent("s")
	ptrs := NewIdent("ptrs")
	cnt := NewIdent("cnt")
	del := NewIdent("delete")
	unlock := NewIdent("Unlock")

	return &ast.FuncDecl{
		Doc:  &ast.CommentGroup{List: ExportComments(DECREMENT_REF_FUNC_NAME)},
		Name: NewIdent(DECREMENT_REF_FUNC_NAME),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ptr},
						Type: &ast.SelectorExpr{
							X:   NewIdent("unsafe"),
							Sel: NewIdent("Pointer"),
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// refs.Lock()
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   refsType,
							Sel: NewIdent("Lock"),
						},
					},
				},
				// num, ok := refs.refs[ptr]
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						num,
						ok,
					},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.IndexExpr{
							X: &ast.SelectorExpr{
								X:   refsType,
								Sel: refsField,
							},
							Index: ptr,
						},
					},
				},
				// if !ok {
				&ast.IfStmt{
					Cond: &ast.UnaryExpr{
						Op: token.NOT,
						X:  ok,
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							// panic("decref untracted object")
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: NewIdent("panic"),
									Args: []ast.Expr{
										&ast.BasicLit{Value: "\"decref untracked object!\"", Kind: token.STRING},
									},
								},
							},
						},
					},
				},
				// }
				// s := refs.ptrs[num]
				&ast.AssignStmt{
					Lhs: []ast.Expr{s},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.IndexExpr{
							X: &ast.SelectorExpr{
								X:   refsType,
								Sel: ptrs,
							},
							Index: num,
						},
					},
				},
				// if s.cnt -1 <= 0 {
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.BinaryExpr{
							X: &ast.SelectorExpr{
								X:   s,
								Sel: cnt,
							},
							Op: token.SUB,
							Y:  &ast.BasicLit{Value: "1", Kind: token.INT},
						},
						Op: token.LEQ,
						Y:  &ast.BasicLit{Value: "0", Kind: token.INT},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							// delete(refs.ptrs, num)
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: del,
									Args: []ast.Expr{
										&ast.SelectorExpr{
											X:   refsType,
											Sel: ptrs,
										},
										num,
									},
								},
							},
							// delete(refs.refs, ptr)
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: del,
									Args: []ast.Expr{
										&ast.SelectorExpr{
											X:   refsType,
											Sel: refsField,
										},
										ptr,
									},
								},
							},
							// refs.Unlock()
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X:   refsType,
										Sel: unlock,
									},
								},
							},
							&ast.ReturnStmt{},
						},
					},
				},
				// }
				// refs.ptrs[num] = __cobjects{s.ptr, s.cnt - 1}
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.IndexExpr{
							X: &ast.SelectorExpr{
								X:   refsType,
								Sel: ptrs,
							},
							Index: num,
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.CompositeLit{
							Type: NewIdent(COBJECT_STRUCT_TYPE_NAME),
							Elts: []ast.Expr{
								&ast.SelectorExpr{
									X:   s,
									Sel: ptr,
								},
								&ast.BinaryExpr{
									X: &ast.SelectorExpr{
										X:   s,
										Sel: NewIdent("cnt"),
									},
									Op: token.SUB,
									Y:  &ast.BasicLit{Value: "1", Kind: token.INT},
								},
							},
						},
					},
				},
				// refs.Unlock()
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   refsType,
							Sel: unlock,
						},
					},
				},
			},
		},
	}
}

func IncrementRef() ast.Decl {
	ptr := NewIdent("ptr")
	refsType := NewIdent(REFS_VAR_NAME)
	refsField := NewIdent(REFS_STRUCT_FIELD_NAME)
	num := NewIdent("num")
	ok := NewIdent("ok")
	s := NewIdent("s")
	ptrs := NewIdent("ptrs")
	next := NewIdent("next")
	unlock := NewIdent("Unlock")

	return &ast.FuncDecl{
		Doc:  &ast.CommentGroup{List: ExportComments(INCREMENT_REF_FUNC_NAME)},
		Name: NewIdent(INCREMENT_REF_FUNC_NAME),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ptr},
						Type: &ast.SelectorExpr{
							X:   NewIdent("unsafe"),
							Sel: NewIdent("Pointer"),
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// refs.Lock()
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   refsType,
							Sel: NewIdent("Lock"),
						},
					},
				},
				// num, ok := refs.refs[ptr]
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						num,
						ok,
					},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.IndexExpr{
							X: &ast.SelectorExpr{
								X:   refsType,
								Sel: refsField,
							},
							Index: ptr,
						},
					},
				},
				// if ok {
				&ast.IfStmt{
					Cond: ok,
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							// s := refs.ptrs[num]
							&ast.AssignStmt{
								Lhs: []ast.Expr{s},
								Tok: token.DEFINE,
								Rhs: []ast.Expr{
									&ast.IndexExpr{
										X: &ast.SelectorExpr{
											X:   refsType,
											Sel: ptrs,
										},
										Index: num,
									},
								},
							},
							// refs.ptrs[num] = cobjects{s.ptr, s.cnt + 1}
							&ast.AssignStmt{
								Lhs: []ast.Expr{
									&ast.IndexExpr{
										X: &ast.SelectorExpr{
											X:   refsType,
											Sel: ptrs,
										},
										Index: num,
									},
								},
								Tok: token.ASSIGN,
								Rhs: []ast.Expr{
									&ast.CompositeLit{
										Type: NewIdent(COBJECT_STRUCT_TYPE_NAME),
										Elts: []ast.Expr{
											&ast.SelectorExpr{
												X:   s,
												Sel: ptr,
											},
											&ast.BinaryExpr{
												X: &ast.SelectorExpr{
													X:   s,
													Sel: NewIdent("cnt"),
												},
												Op: token.ADD,
												Y:  &ast.BasicLit{Value: "1", Kind: token.INT},
											},
										},
									},
								},
							},
						},
					},
					// } else {
					Else: &ast.BlockStmt{
						List: []ast.Stmt{
							// num = refs.next
							&ast.AssignStmt{
								Lhs: []ast.Expr{
									num,
								},
								Tok: token.ASSIGN,
								Rhs: []ast.Expr{
									&ast.SelectorExpr{
										X:   refsType,
										Sel: next,
									},
								},
							},
							// refs.next--
							&ast.IncDecStmt{
								X: &ast.SelectorExpr{
									X:   refsType,
									Sel: next,
								},
								Tok: token.DEC,
							},
							// if refs.next > 0 {
							&ast.IfStmt{
								Cond: &ast.BinaryExpr{
									X: &ast.SelectorExpr{
										X:   refsType,
										Sel: next,
									},
									Op: token.GTR,
									Y:  &ast.BasicLit{Value: "0", Kind: token.INT},
								},
								Body: &ast.BlockStmt{
									List: []ast.Stmt{
										// panic("refs.next underflow")
										&ast.ExprStmt{
											X: &ast.CallExpr{
												Fun: NewIdent("panic"),
												Args: []ast.Expr{
													&ast.BasicLit{Value: "\"refs.next underflow\"", Kind: token.STRING},
												},
											},
										},
									},
								},
							},
							// }
							// refs.refs[ptr] = num
							&ast.AssignStmt{
								Lhs: []ast.Expr{
									&ast.IndexExpr{
										X: &ast.SelectorExpr{
											X:   refsType,
											Sel: refsField,
										},
										Index: ptr,
									},
								},
								Tok: token.ASSIGN,
								Rhs: []ast.Expr{
									num,
								},
							},
							// refs.ptrs[num] = __cobject{ptr, 1}
							&ast.AssignStmt{
								Lhs: []ast.Expr{
									&ast.IndexExpr{
										X: &ast.SelectorExpr{
											X:   refsType,
											Sel: ptrs,
										},
										Index: num,
									},
								},
								Tok: token.ASSIGN,
								Rhs: []ast.Expr{
									&ast.CompositeLit{
										Type: NewIdent(COBJECT_STRUCT_TYPE_NAME),
										Elts: []ast.Expr{
											ptr,
											&ast.BasicLit{Value: "1", Kind: token.INT},
										},
									},
								},
							},
						},
					},
				},
				// refs.Unlock()
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   refsType,
							Sel: unlock,
						},
					},
				},
			},
		},
	}
}

func UnsafePointerToTarget(targets ...ast.Expr) ast.Expr {
	return &ast.CallExpr{
		Fun:  unsafePointer,
		Args: targets,
	}
}

// IncrementRefCall takes a target expression to increment it's cgo pointer ref and returns the expression
func IncrementRefCall(target ast.Expr) *ast.ExprStmt {
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun:  NewIdent(INCREMENT_REF_FUNC_NAME),
			Args: []ast.Expr{UnsafePointerToTarget(target)},
		},
	}
}

// DecrementRefCall takes a target expression to decrement it's cgo pointer ref and returns the expression
func DecrementRefCall(target ast.Expr) *ast.ExprStmt {
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun:  NewIdent(DECREMENT_REF_FUNC_NAME),
			Args: []ast.Expr{target},
		},
	}
}

// NewIdent takes a name as string and returns an *ast.Ident in that name
func NewIdent(name string) *ast.Ident {
	return &ast.Ident{
		Name: name,
	}
}

// DeclareVar declares a local variable
func DeclareVar(name *ast.Ident, t ast.Expr) *ast.DeclStmt {
	return &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{name},
					Type:  t,
				},
			},
		},
	}
}

// CastUnsafePtr take a cast type and target expression and returns a cast expression
func CastUnsafePtr(castType, target ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{
		Fun: &ast.ParenExpr{
			X: castType,
		},
		Args: []ast.Expr{target},
	}
}

// DeRef takes an expression and prefaces the expression with a *
func DeRef(expr ast.Expr) *ast.StarExpr {
	return &ast.StarExpr{X: expr}
}

// Ref takes an expression and prefaces the expression with a &
func Ref(expr ast.Expr) ast.Expr {
	return &ast.UnaryExpr{
		X:  expr,
		Op: token.AND,
	}
}

// Return takes an expression and returns a return statement containing the expression
func Return(expressions ...ast.Expr) *ast.ReturnStmt {
	return &ast.ReturnStmt{
		Results: expressions,
	}
}

// FormatSprintf takes a format and a target expression and returns a fmt.Sprintf expression
func FormatSprintf(format string, target ast.Expr) *ast.CallExpr {
	fmtSprintf := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   NewIdent("fmt"),
			Sel: NewIdent("Sprintf"),
		},
		Args: []ast.Expr{
			&ast.BasicLit{
				Kind:  token.STRING,
				Value: "\"" + format + "\"",
			},
			target,
		},
	}

	return fmtSprintf
}

// ExportComments takes a name to export as string and returns a comment group
func ExportComments(exportName string) []*ast.Comment {
	return []*ast.Comment{
		{
			Text:  "//export " + exportName,
			Slash: token.Pos(1),
		},
	}
}

// InstanceMethodParams returns a constructed field list for an instance method
func InstanceMethodParams(fields ...*ast.Field) *ast.FieldList {
	tmpFields := []*ast.Field{
		{
			Names: []*ast.Ident{NewIdent("self")},
			Type:  unsafePointer,
		},
	}
	tmpFields = append(tmpFields, fields...)
	params := &ast.FieldList{
		List: tmpFields,
	}
	return params
}

// UintPtr wraps an argument in a uintptr
func UintPtr(arg ast.Expr) ast.Expr {
	return &ast.CallExpr{
		Fun:  NewIdent("uintptr"),
		Args: []ast.Expr{arg},
	}
}

// FuncAst returns an FuncDecl which wraps the func
func FuncAst(f *Func) *ast.FuncDecl {
	fun := f.Func
	functionName := f.CGoName()
	sig := fun.Type().(*types.Signature)

	functionCall := &ast.CallExpr{
		Fun:  f.AliasedGoName(),
		Args: ParamIdents(sig.Params()),
	}

	funcDecl := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: ExportComments(functionName),
		},
		Name: NewIdent(functionName),
		Body: &ast.BlockStmt{List: []ast.Stmt{}},
	}

	params := Fields(sig.Params())

	if sig.Results().Len() > 0 {
		resultNames := make([]ast.Expr, sig.Results().Len())
		for i := 0; i < sig.Results().Len(); i++ {
			resultNames[i] = NewIdent(fmt.Sprintf("r%d", i))
		}

		assign := &ast.AssignStmt{
			Lhs: resultNames,
			Tok: token.DEFINE,
			Rhs: []ast.Expr{functionCall},
		}

		funcDecl.Body.List = append(funcDecl.Body.List, assign)
		resultExprs := make([]ast.Expr, sig.Results().Len())
		for i := 0; i < sig.Results().Len(); i++ {
			result := sig.Results().At(i)
			switch result.Type().(type) {
			case *types.Basic:
				resultExprs[i] = resultNames[i]
			case *types.Pointer:
				// already have a pointer, so just count the reference
				funcDecl.Body.List = append(funcDecl.Body.List, IncrementRefCall(resultNames[i]))
				resultExprs[i] = UintPtr(UnsafePointerToTarget(resultNames[i]))
			case *types.Named:
				// grab a reference to the named type
				funcDecl.Body.List = append(funcDecl.Body.List, IncrementRefCall(Ref(resultNames[i])))
				resultExprs[i] = UintPtr(UnsafePointerToTarget(Ref(resultNames[i])))
			}
		}

		funcDecl.Body.List = append(funcDecl.Body.List, Return(resultExprs...))

		funcDecl.Type = &ast.FuncType{
			Params:  params,
			Results: Fields(sig.Results()),
		}
	} else {
		funcDecl.Body.List = append(funcDecl.Body.List, &ast.ExprStmt{
			X: functionCall,
		})

		funcDecl.Type = &ast.FuncType{
			Params: params,
		}
	}

	return funcDecl
}

type result struct {
	Name *ast.Ident
	Stmt ast.Stmt
}

// ParamIdents transforms parameter tuples into a slice of AST expressions
func ParamIdents(funcParams *types.Tuple) []ast.Expr {
	if funcParams == nil || funcParams.Len() <= 0 {
		return []ast.Expr{}
	}

	args := make([]ast.Expr, funcParams.Len())
	for i := 0; i < funcParams.Len(); i++ {
		param := funcParams.At(i)
		args[i] = ParamExpr(param, param.Type())
	}
	return args
}

func ParamExpr(param *types.Var, t types.Type) ast.Expr {
	return CastExpr(t, NewIdent(param.Name()))
}

// UintptrToUnsafePointer wraps the argument in an UnsafePointer
func UintptrToUnsafePointer(arg ast.Expr) ast.Expr {
	return &ast.CallExpr{
		Fun:  unsafePointer,
		Args: []ast.Expr{arg},
	}
}

func CastExpr(t types.Type, ident ast.Expr) ast.Expr {

	switch t := t.(type) {
	case *types.Pointer:
		return DeRef(CastExpr(t.Elem(), UintptrToUnsafePointer(ident)))
	case *types.Named:
		pkg := t.Obj().Pkg()
		typeName := t.Obj().Name()
		castExpr := DeRef(CastUnsafePtr(DeRef(&ast.SelectorExpr{
			X:   NewIdent(PkgPathAliasFromString(pkg.Path())),
			Sel: NewIdent(typeName),
		}), UintptrToUnsafePointer(ident)))
		return castExpr
	case *types.Slice:
		return DeRef(CastUnsafePtr(DeRef(NewIdent("[]"+t.Elem().String())), UintptrToUnsafePointer(ident)))
	default:
		return ident
	}
}

// Fields transforms parameters into a list of AST fields
func Fields(funcParams *types.Tuple) *ast.FieldList {
	if funcParams == nil || funcParams.Len() <= 0 {
		return &ast.FieldList{}
	}

	fields := make([]*ast.Field, funcParams.Len())
	for i := 0; i < funcParams.Len(); i++ {
		p := funcParams.At(i)
		switch t := p.Type().(type) {
		case *types.Pointer:
			fields[i] = UintPtrOrBasic(p, t.Elem())
		default:
			fields[i] = UintPtrOrBasic(p, t)
		}
	}
	return &ast.FieldList{List: fields}
}

// UintPtrOrBasic returns a Basic typed field or an unsafe pointer if not a Basic type
func UintPtrOrBasic(p *types.Var, t types.Type) *ast.Field {
	returnDefault := func() *ast.Field {
		return &ast.Field{
			Type:  uintptr,
			Names: []*ast.Ident{NewIdent(p.Name())},
		}
	}
	switch t.(type) {
	case *types.Basic:
		return VarToField(p, t)
	//case *types.Named, *types.Interface:
	//	if ImplementsError(t) {
	//		return &ast.Field{
	//			Type: NewIdent("error"),
	//		}
	//	} else {
	//		return returnDefault()
	//	}
	default:
		return returnDefault()
	}
}

type hasMethods interface {
	NumMethods() int
	Method(int) *types.Func
}

// ImplementsError returns true if a type has an Error() string function signature
func ImplementsError(t types.Type) bool {
	isError := func(fun *types.Func) bool {
		if sig, ok := fun.Type().(*types.Signature); ok {
			if fun.Name() != "Error" {
				return false
			}

			if sig.Params().Len() != 0 {
				return false
			}

			if sig.Results().Len() == 1 {
				result := sig.Results().At(0)
				if obj, ok := result.Type().(*types.Basic); ok {
					return obj.Kind() == types.String
				}
			}
		}

		return false
	}

	hasErrorMethod := func(obj hasMethods) bool {
		numMethods := obj.NumMethods()
		for i := 0; i < numMethods; i++ {
			if isError(obj.Method(i)) {
				return true
			}
		}
		return false
	}

	if obj, ok := t.Underlying().(hasMethods); ok {
		return hasErrorMethod(obj)
	}

	return false
}

// VarToField transforms a Var into an AST field
func VarToField(p *types.Var, t types.Type) *ast.Field {
	switch typ := t.(type) {
	case *types.Named:
		return NamedToField(p, typ)
	default:
		name := p.Name()
		typeName := p.Type().String()
		return &ast.Field{
			Type:  NewIdent(typeName),
			Names: []*ast.Ident{NewIdent(name)},
		}

	}
}

// NameToField transforms a Var that's a Named type into an AST Field
func NamedToField(p *types.Var, named *types.Named) *ast.Field {
	pkg := p.Pkg()
	if pkg == nil {
		pkg = named.Obj().Pkg()
	}

	if pkg != nil {
		path := pkg.Path()
		pkgAlias := PkgPathAliasFromString(path)
		typeName := pkgAlias + "." + named.Obj().Name()
		nameIdent := NewIdent(p.Name())
		return &ast.Field{
			Type:  NewIdent(typeName),
			Names: []*ast.Ident{nameIdent},
		}
	} else {
		typeIdnet := NewIdent(p.Type().String())
		nameIdent := NewIdent(p.Name())
		return &ast.Field{
			Type:  typeIdnet,
			Names: []*ast.Ident{nameIdent},
		}
	}
}

// PkgPathAliasFromString takes a golang path as a string and returns an import alias for that path
func PkgPathAliasFromString(path string) string {
	splits := strings.FieldsFunc(path, splitPkgPath)
	return strings.Join(splits, "_")
}

func splitPkgPath(r rune) bool {
	return r == '.' || r == '/'
}
