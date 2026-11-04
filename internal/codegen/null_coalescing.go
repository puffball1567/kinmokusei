package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

const coalescePrefix = "__kinmokuseiCoalesce"

func generateCoalesce(source *ast.BinaryExpr) (goast.Expr, error) {
	if source.CoalesceType == nil {
		return nil, fmt.Errorf("?? has no checked result type")
	}
	left, err := generateExpression(source.Left)
	if err != nil {
		return nil, err
	}
	if source.CoalesceHelper != "" {
		fallback := source.Right.(*ast.CallExpr)
		callee, err := generateExpression(fallback.Callee)
		if err != nil {
			return nil, err
		}
		return &goast.CallExpr{Fun: goast.NewIdent(coalescePrefix + source.CoalesceHelper), Args: []goast.Expr{left, callee}}, nil
	}
	right, err := generateExpression(source.Right)
	if err != nil {
		return nil, err
	}
	body := &goast.BlockStmt{}
	if literal, ok := source.Left.(*ast.LiteralExpr); !ok || (literal.Kind != ast.NullLiteral && literal.Kind != ast.NilLiteral) {
		value := goast.NewIdent("__kinmokuseiCoalesceValue")
		present := goast.Expr(value)
		if source.CoalesceLeftValue != nil {
			present, err = generateExpression(source.CoalesceLeftValue)
			if err != nil {
				return nil, err
			}
		}
		body.List = append(body.List,
			&goast.AssignStmt{Lhs: []goast.Expr{value}, Tok: token.DEFINE, Rhs: []goast.Expr{left}},
			&goast.IfStmt{Cond: &goast.BinaryExpr{X: value, Op: token.NEQ, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{present}}}}},
		)
	}
	body.List = append(body.List, &goast.ReturnStmt{Results: []goast.Expr{right}})
	return &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}, Results: &goast.FieldList{List: []*goast.Field{{Type: goType(*source.CoalesceType)}}}}, Body: body}}, nil
}

func coalesceDeclarations(file *goast.File) ([]goast.Decl, error) {
	needed := map[string]bool{}
	goast.Inspect(file, func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok {
			switch name.Name {
			case coalescePrefix + "Pointer", coalescePrefix + "Slice", coalescePrefix + "Map", coalescePrefix + "Interface", coalescePrefix + "Channel", coalescePrefix + "Send", coalescePrefix + "Receive":
				needed[name.Name] = true
			}
		}
		return true
	})
	if len(needed) == 0 {
		return nil, nil
	}
	declarations, err := parseDeferSupport(`package p
func __kinmokuseiCoalescePointer[P ~*E,E any](value P,fallback func()P)P{if value!=nil{return value};return fallback()}
func __kinmokuseiCoalesceSlice[S ~[]E,E any](value S,fallback func()S)S{if value!=nil{return value};return fallback()}
func __kinmokuseiCoalesceMap[M ~map[K]V,K comparable,V any](value M,fallback func()M)M{if value!=nil{return value};return fallback()}
func __kinmokuseiCoalesceInterface[I any](value I,fallback func()I)I{if interface{}(value)!=nil{return value};return fallback()}
func __kinmokuseiCoalesceChannel[C ~chan E,E any](value C,fallback func()C)C{if value!=nil{return value};return fallback()}
func __kinmokuseiCoalesceSend[C ~chan<- E,E any](value C,fallback func()C)C{if value!=nil{return value};return fallback()}
func __kinmokuseiCoalesceReceive[C ~<-chan E,E any](value C,fallback func()C)C{if value!=nil{return value};return fallback()}
`)
	if err != nil {
		return nil, err
	}
	var used []goast.Decl
	for _, declaration := range declarations {
		if function, ok := declaration.(*goast.FuncDecl); ok && needed[function.Name.Name] {
			used = append(used, declaration)
		}
	}
	return used, nil
}
