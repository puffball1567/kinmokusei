package codegen

import (
	goast "go/ast"
	"go/token"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func generateMatch(expr *ast.MatchExpr) (goast.Expr, error) {
	scrutinee, err := generateExpression(expr.Value)
	if err != nil {
		return nil, err
	}
	value := goast.NewIdent("__kinmokuseiMatchValue")
	body := &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: []goast.Expr{value}, Tok: token.DEFINE, Rhs: []goast.Expr{scrutinee}},
		&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN, Rhs: []goast.Expr{value}},
	}}
	unconditional := false
	for i := range expr.Arms {
		arm := &expr.Arms[i]
		var pattern matchPatternCode
		if err := pattern.generate(&arm.Pattern, value); err != nil {
			return nil, err
		}
		result, err := generateExpression(arm.Value)
		if err != nil {
			return nil, err
		}
		returnResult := &goast.ReturnStmt{Results: []goast.Expr{result}}
		if arm.Guard == nil {
			pattern.bindings = append(pattern.bindings, returnResult)
		} else {
			guard, err := generateExpression(arm.Guard)
			if err != nil {
				return nil, err
			}
			pattern.bindings = append(pattern.bindings, &goast.IfStmt{Cond: guard, Body: &goast.BlockStmt{List: []goast.Stmt{returnResult}}})
		}
		armBody := &goast.BlockStmt{List: pattern.bindings}
		if len(pattern.checks) != 0 {
			body.List = append(body.List, &goast.IfStmt{Cond: matchConjunction(pattern.checks), Body: armBody})
		} else {
			body.List = append(body.List, armBody)
			unconditional = arm.Guard == nil
		}
	}
	if !unconditional {
		body.List = append(body.List, &goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{stringLiteral("unreachable match")}}})
	}
	return &goast.CallExpr{Fun: &goast.FuncLit{
		Type: &goast.FuncType{Params: &goast.FieldList{}, Results: &goast.FieldList{List: []*goast.Field{{Type: goType(expr.ResultType)}}}},
		Body: body,
	}}, nil
}

type matchPatternCode struct {
	checks   []goast.Expr
	bindings []goast.Stmt
}

func (c *matchPatternCode) generate(pattern *ast.MatchPattern, value goast.Expr) error {
	switch pattern.Kind {
	case ast.MatchBinding, ast.MatchNonNullBinding:
		if pattern.Kind == ast.MatchNonNullBinding {
			c.addCheck(value, token.NEQ, goast.NewIdent("nil"))
		}
		if pattern.Binding.Name != "_" {
			name := goast.NewIdent(goName(pattern.Binding.Name))
			if pattern.Binding.NonNull {
				value = generateNonNullReference(value, pattern.Binding.ResolvedType, pattern.Binding.Span, "match binding")
			}
			c.bindings = append(c.bindings,
				&goast.AssignStmt{Lhs: []goast.Expr{name}, Tok: token.DEFINE, Rhs: []goast.Expr{value}},
				&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN, Rhs: []goast.Expr{name}},
			)
		}
	case ast.MatchLiteral:
		literal, err := generateExpression(pattern.Literal)
		if err != nil {
			return err
		}
		if _, regex := pattern.Literal.(*ast.RegexExpr); regex {
			c.checks = append(c.checks, &goast.CallExpr{Fun: &goast.SelectorExpr{X: literal, Sel: goast.NewIdent("test")}, Args: []goast.Expr{value}})
		} else {
			c.addCheck(value, token.EQL, literal)
		}
	case ast.MatchRecord:
		for i := range pattern.Fields {
			field := &pattern.Fields[i]
			projection := &goast.SelectorExpr{X: value, Sel: goast.NewIdent(goName(field.ResolvedName))}
			if err := c.generate(&field.Pattern, projection); err != nil {
				return err
			}
		}
	case ast.MatchSequence:
		if !pattern.FixedLength {
			length := &goast.CallExpr{Fun: goast.NewIdent("len"), Args: []goast.Expr{value}}
			c.addCheck(length, token.EQL, matchIndex(len(pattern.Elements)))
		}
		for i := range pattern.Elements {
			projection := &goast.IndexExpr{X: value, Index: matchIndex(i)}
			if err := c.generate(&pattern.Elements[i], projection); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *matchPatternCode) addCheck(left goast.Expr, operator token.Token, right goast.Expr) {
	c.checks = append(c.checks, &goast.BinaryExpr{X: left, Op: operator, Y: right})
}

func matchIndex(index int) goast.Expr {
	return &goast.BasicLit{Kind: token.INT, Value: strconv.Itoa(index)}
}

// Balance large patterns without changing the left-to-right short-circuit order.
func matchConjunction(checks []goast.Expr) goast.Expr {
	if len(checks) == 1 {
		return checks[0]
	}
	middle := len(checks) / 2
	return &goast.BinaryExpr{X: matchConjunction(checks[:middle]), Op: token.LAND, Y: matchConjunction(checks[middle:])}
}
