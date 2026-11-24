package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"path/filepath"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func generateMatch(expr *ast.MatchExpr) (goast.Expr, error) {
	body, err := generateMatchBody(expr, func(arm *ast.MatchArm) (goast.Stmt, error) {
		result, err := generateExpression(arm.Value)
		if err != nil {
			return nil, err
		}
		return &goast.ReturnStmt{Results: []goast.Expr{result}}, nil
	})
	if err != nil {
		return nil, err
	}
	return &goast.CallExpr{Fun: &goast.FuncLit{
		Type: &goast.FuncType{Params: &goast.FieldList{}, Results: &goast.FieldList{List: []*goast.Field{{Type: goType(expr.ResultType)}}}},
		Body: body,
	}}, nil
}

func generateMatchResultReturn(stmt *ast.ReturnStmt, resultZero func(ast.TypeRef) goast.Expr) (goast.Stmt, error) {
	expr, ok := stmt.Value.(*ast.MatchExpr)
	if !ok {
		return nil, fmt.Errorf("Result match return requires a match expression")
	}
	return generateMatchBody(expr, func(arm *ast.MatchArm) (goast.Stmt, error) {
		if arm.ResultReturn == nil {
			return nil, fmt.Errorf("Result match arm is missing a checked return")
		}
		returned := *arm.ResultReturn
		returned.CrossesTry = stmt.CrossesTry
		return generateResultReturn(&returned, resultZero)
	})
}

func generateMatchBody(expr *ast.MatchExpr, generateArmReturn func(*ast.MatchArm) (goast.Stmt, error)) (*goast.BlockStmt, error) {
	scrutinee, err := generateExpression(expr.Value)
	if err != nil {
		return nil, err
	}
	value := goast.NewIdent("__kinmokuseiMatchValue")
	failure := goast.NewIdent("__kinmokuseiMatchError")
	captures := []goast.Expr{value}
	switch expr.SubjectKind {
	case ast.MatchResultSubject:
		captures = append(captures, failure)
	case ast.MatchVoidResultSubject:
		captures = []goast.Expr{failure}
	}
	body := &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: captures, Tok: token.DEFINE, Rhs: []goast.Expr{scrutinee}},
	}}
	if expr.SubjectKind != ast.MatchVoidResultSubject {
		body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN, Rhs: []goast.Expr{value}})
	}
	unconditional := false
	for i := range expr.Arms {
		arm := &expr.Arms[i]
		var pattern matchPatternCode
		var err error
		if expr.SubjectKind == ast.MatchValueSubject {
			err = pattern.generate(&arm.Pattern, value)
		} else {
			err = pattern.generateResult(&arm.Pattern, value, failure)
		}
		if err != nil {
			return nil, err
		}
		returnResult, err := generateArmReturn(arm)
		if err != nil {
			return nil, err
		}
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
			body.List = append(body.List, wrapMatchChecks(pattern.checks, armBody))
		} else {
			body.List = append(body.List, armBody)
			unconditional = arm.Guard == nil
		}
	}
	if !unconditional {
		message := "unreachable match"
		if expr.EnumExhaustive {
			message = fmt.Sprintf("match received an undeclared enum value at %s:%d:%d",
				filepath.Base(expr.Span.Path), expr.Span.Start.Line, expr.Span.Start.Column)
		}
		body.List = append(body.List, &goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{stringLiteral(message)}}})
	}
	return body, nil
}

type matchPatternCode struct {
	checks   []matchPatternCheck
	bindings []goast.Stmt
}

type matchPatternCheck struct {
	init      goast.Stmt
	condition goast.Expr
}

func (c *matchPatternCode) generate(pattern *ast.MatchPattern, value goast.Expr) error {
	switch pattern.Kind {
	case ast.MatchTypeBinding:
		c.addTypeCheck(pattern, value)
	case ast.MatchBinding, ast.MatchNonNullBinding, ast.MatchSequenceRest:
		if pattern.Kind == ast.MatchNonNullBinding {
			c.addCheck(value, token.NEQ, goast.NewIdent("nil"))
		}
		c.addBinding(pattern.Binding, value)
	case ast.MatchLiteral:
		literal, err := generateExpression(pattern.Literal)
		if err != nil {
			return err
		}
		if _, regex := pattern.Literal.(*ast.RegexExpr); regex {
			c.checks = append(c.checks, matchPatternCheck{condition: &goast.CallExpr{Fun: &goast.SelectorExpr{X: literal, Sel: goast.NewIdent("test")}, Args: []goast.Expr{value}}})
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
		return c.generateSequence(pattern, value)
	}
	return nil
}

func (c *matchPatternCode) addBinding(binding ast.Binding, value goast.Expr) {
	if binding.Name == "_" {
		return
	}
	name := goast.NewIdent(goName(binding.Name))
	c.bindings = append(c.bindings,
		&goast.AssignStmt{Lhs: []goast.Expr{name}, Tok: token.DEFINE, Rhs: []goast.Expr{value}},
		&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN, Rhs: []goast.Expr{name}},
	)
	if binding.NonNull {
		c.bindings = append(c.bindings, generateTypedNonNullCheck(name, binding.ResolvedType, binding.Span, "match binding", nil))
	}
}

func (c *matchPatternCode) addTypeCheck(pattern *ast.MatchPattern, operand goast.Expr) {
	assertion := *pattern.Assertion
	if assertion.ClassDowncast {
		// Use the canonical class name for its downcast helper, including
		// when the source pattern spells the target through an alias.
		assertion.Type = pattern.Binding.ResolvedType
	}
	value := goast.NewIdent(fmt.Sprintf("__kinmokuseiMatchTyped%d", len(c.checks)))
	if pattern.Binding.Name == "_" {
		value = goast.NewIdent("_")
	}
	ok := goast.NewIdent(fmt.Sprintf("__kinmokuseiMatchOK%d", len(c.checks)))
	c.checks = append(c.checks, matchPatternCheck{
		init:      &goast.AssignStmt{Lhs: []goast.Expr{value, ok}, Tok: token.DEFINE, Rhs: []goast.Expr{generateTypeAssertion(&assertion, operand)}},
		condition: ok,
	})
	c.addBinding(pattern.Binding, value)
}

func (c *matchPatternCode) generateResult(pattern *ast.MatchPattern, value, failure *goast.Ident) error {
	operator := token.EQL
	if pattern.Kind == ast.MatchResultFail {
		operator = token.NEQ
		value = failure
	}
	// Check the outcome before inspecting the payload, which may be a zero
	// value that violates the source contract on the failure edge.
	c.addCheck(failure, operator, goast.NewIdent("nil"))
	if pattern.Payload != nil {
		return c.generate(pattern.Payload, value)
	}
	return nil
}

func (c *matchPatternCode) addCheck(left goast.Expr, operator token.Token, right goast.Expr) {
	c.checks = append(c.checks, matchPatternCheck{condition: &goast.BinaryExpr{X: left, Op: operator, Y: right}})
}

// Each assertion introduces a scope for its captured value, after any
// earlier checks that make its operand safe to evaluate.
func wrapMatchChecks(checks []matchPatternCheck, body *goast.BlockStmt) goast.Stmt {
	for end := len(checks); end > 0; {
		start := end - 1
		for start > 0 && checks[start].init == nil {
			start--
		}
		statement := &goast.IfStmt{Init: checks[start].init, Cond: matchConjunction(checks[start:end]), Body: body}
		body = &goast.BlockStmt{List: []goast.Stmt{statement}}
		end = start
	}
	return body.List[0]
}

// Balance large patterns without changing the left-to-right short-circuit order.
func matchConjunction(checks []matchPatternCheck) goast.Expr {
	if len(checks) == 1 {
		return checks[0].condition
	}
	middle := len(checks) / 2
	return &goast.BinaryExpr{X: matchConjunction(checks[:middle]), Op: token.LAND, Y: matchConjunction(checks[middle:])}
}
