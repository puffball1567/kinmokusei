package codegen

import (
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// A typed cell preserves constants, interface storage, and typed nil values
// before crossing any. The separate contract retains source distinctions that
// Go erases (including nullability inside collections and callable signatures).
func decoratorPayloadType(ref ast.TypeRef) *goast.StructType {
	return &goast.StructType{Fields: &goast.FieldList{List: []*goast.Field{
		{Names: []*goast.Ident{goast.NewIdent("value")}, Type: goType(ref)},
	}}}
}

func decoratorBox(value goast.Expr, ref ast.TypeRef, identity, contract string) goast.Expr {
	return &goast.CompositeLit{Type: goast.NewIdent("__kinmokuseiDecoratorValue"), Elts: []goast.Expr{
		&goast.KeyValueExpr{Key: goast.NewIdent("TypeIdentity"), Value: stringLiteral(identity)},
		&goast.KeyValueExpr{Key: goast.NewIdent("contract"), Value: stringLiteral(contract)},
		&goast.KeyValueExpr{Key: goast.NewIdent("value"), Value: &goast.CompositeLit{Type: decoratorPayloadType(ref), Elts: []goast.Expr{value}}},
	}}
}

// The input expression is evaluated once, including when it is a function call.
// A failed assertion reads only the zero-value cell and returns false.
func decoratorDecode(value goast.Expr, ref ast.TypeRef, contract string) goast.Expr {
	boxed, cell, ok := goast.NewIdent("boxed"), goast.NewIdent("cell"), goast.NewIdent("ok")
	return &goast.CallExpr{Fun: &goast.FuncLit{
		Type: &goast.FuncType{
			Params:  &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{boxed}, Type: goast.NewIdent("__kinmokuseiDecoratorValue")}}},
			Results: &goast.FieldList{List: []*goast.Field{{Type: goType(ref)}, {Type: goast.NewIdent("bool")}}},
		},
		Body: &goast.BlockStmt{List: []goast.Stmt{
			&goast.AssignStmt{Lhs: []goast.Expr{cell, ok}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.TypeAssertExpr{
				X: &goast.SelectorExpr{X: boxed, Sel: goast.NewIdent("value")}, Type: decoratorPayloadType(ref),
			}}},
			&goast.ReturnStmt{Results: []goast.Expr{
				&goast.SelectorExpr{X: cell, Sel: goast.NewIdent("value")},
				&goast.BinaryExpr{X: ok, Op: token.LAND, Y: &goast.BinaryExpr{
					X: &goast.SelectorExpr{X: boxed, Sel: goast.NewIdent("contract")}, Op: token.EQL, Y: stringLiteral(contract),
				}},
			}},
		}},
	}, Args: []goast.Expr{value}}
}
