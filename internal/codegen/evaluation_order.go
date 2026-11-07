package codegen

import (
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

const evaluationValueName = "__kinmokuseiEvalValue"

func evaluatedValue(value goast.Expr) goast.Expr {
	return &goast.CallExpr{Fun: goast.NewIdent(evaluationValueName), Args: []goast.Expr{value}}
}

// A Go call boundary fixes an earlier value before subsequent calls/receives.
// Generic inference retains private/anonymous Go types without naming them.
func snapshotOrderedValue(source ast.Expression, value goast.Expr) goast.Expr {
	if source == nil || isGoConstant(source) {
		return value
	}
	if literal, ok := source.(*ast.LiteralExpr); ok && (literal.Kind == ast.NilLiteral || literal.Kind == ast.NullLiteral) {
		return value
	}
	// Real calls already finish before later calls in Go. Avoid an extra
	// boundary around their completed, typed results.
	if call, ok := source.(*ast.CallExpr); ok && call.Builtin == ast.NotBuiltinCall && !call.Conversion {
		return value
	}
	if _, ok := source.(*ast.NewExpr); ok {
		return value
	}
	if hasContextualShift(source) {
		switch source := source.(type) {
		case *ast.BinaryExpr:
			binary := value.(*goast.BinaryExpr)
			binary.X = snapshotOrderedValue(source.Left, binary.X)
			binary.Y = snapshotOrderedValue(source.Right, binary.Y)
			if (source.Operator == "<<" || source.Operator == ">>") && !isGoConstant(source.Right) {
				binary.Y = &goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiEvalShiftCount"), Args: []goast.Expr{binary.Y}}
			}
		case *ast.UnaryExpr:
			unary := value.(*goast.UnaryExpr)
			unary.X = snapshotOrderedValue(source.Operand, unary.X)
		case *ast.CallExpr:
			call := value.(*goast.CallExpr)
			for i, argument := range source.Arguments {
				call.Args[i] = snapshotOrderedValue(argument, call.Args[i])
			}
		}
		return value
	}
	return evaluatedValue(value)
}

// Runtime shifts of constants inherit their type from the containing call or
// operator. Snapshot their mutable operands rather than defaulting the result
// to int in an unconstrained generic call.
func hasContextualShift(expression ast.Expression) bool {
	if isGoConstant(expression) {
		return false
	}
	switch expr := expression.(type) {
	case *ast.BinaryExpr:
		if expr.Operator == "??" {
			return false
		}
		return (expr.Operator == "<<" || expr.Operator == ">>") && isGoConstant(expr.Left) || hasContextualShift(expr.Left) || hasContextualShift(expr.Right)
	case *ast.UnaryExpr:
		return expr.Operator != "&" && expr.Operator != "*" && expr.Operator != ast.NonNullOperator && hasContextualShift(expr.Operand)
	case *ast.CallExpr:
		if expr.Builtin == ast.MinCall || expr.Builtin == ast.MaxCall || expr.Builtin == ast.ComplexCall || expr.Builtin == ast.RealCall || expr.Builtin == ast.ImagCall {
			for _, argument := range expr.Arguments {
				if hasContextualShift(argument) {
					return true
				}
			}
		}
	}
	return false
}

func evaluationMayHaveEffects(expression ast.Expression) bool {
	switch expr := expression.(type) {
	case *ast.ArrowExpr, *ast.IdentifierExpr, *ast.LiteralExpr, nil:
		return false
	case *ast.CallExpr:
		if expr.NonNullConversion {
			return true
		}
		if isGoConstant(expr) {
			return false
		}
		if expr.Conversion || expr.Builtin == ast.CopyArrayCall || expr.Builtin == ast.ViewArrayCall {
			return expressionListMayHaveEffects(expr.Arguments)
		}
		if expr.Builtin == ast.LenCall || expr.Builtin == ast.CapCall {
			return expressionListMayHaveEffects(expr.Arguments)
		}
		return true
	case *ast.NewExpr, *ast.TaskStartExpr, *ast.AwaitExpr, *ast.PropagateExpr, *ast.GoTypeAssertionExpr:
		return true
	case *ast.UnaryExpr:
		return expr.Operator == "<-" || expr.Operator == ast.NonNullOperator || evaluationMayHaveEffects(expr.Operand)
	case *ast.BinaryExpr:
		if expr.Operator == "??" {
			return true
		}
		return evaluationMayHaveEffects(expr.Left) || evaluationMayHaveEffects(expr.Right)
	case *ast.MemberExpr:
		return expr.Property || evaluationMayHaveEffects(expr.Object)
	case *ast.IndexExpr:
		return evaluationMayHaveEffects(expr.Object) || evaluationMayHaveEffects(expr.Index)
	case *ast.SliceExpr:
		return evaluationMayHaveEffects(expr.Object) || evaluationMayHaveEffects(expr.Low) || evaluationMayHaveEffects(expr.High) || evaluationMayHaveEffects(expr.Max)
	case *ast.ArrayLiteralExpr:
		for _, value := range expr.Elements {
			if evaluationMayHaveEffects(value) {
				return true
			}
		}
	case *ast.ObjectLiteralExpr:
		for _, field := range expr.Fields {
			if evaluationMayHaveEffects(field.Value) {
				return true
			}
		}
	case *ast.GoCompositeLiteralExpr:
		for _, field := range expr.Fields {
			if evaluationMayHaveEffects(field.Value) {
				return true
			}
		}
	case *ast.ClassUpcastExpr:
		return evaluationMayHaveEffects(expr.Value)
	}
	return false
}

func orderExpressionList(source []ast.Expression, values []goast.Expr) {
	later := false
	for i := len(source) - 1; i >= 0; i-- {
		if later {
			values[i] = snapshotOrderedValue(source[i], values[i])
		}
		later = later || evaluationMayHaveEffects(source[i])
	}
}

func expressionListMayHaveEffects(expressions []ast.Expression) bool {
	for _, expression := range expressions {
		if evaluationMayHaveEffects(expression) {
			return true
		}
	}
	return false
}

func orderFieldValues(source []ast.ObjectField, values []goast.Expr) {
	later := false
	for i := len(source) - 1; i >= 0; i-- {
		field := values[i].(*goast.KeyValueExpr)
		if later {
			field.Value = snapshotOrderedValue(source[i].Value, field.Value)
		}
		later = later || evaluationMayHaveEffects(source[i].Value)
	}
}

func evaluationRuntimeDeclarations(file *goast.File) ([]goast.Decl, error) {
	needed := false
	goast.Inspect(file, func(node goast.Node) bool {
		if name, ok := node.(*goast.Ident); ok && (name.Name == evaluationValueName || name.Name == "__kinmokuseiEvalShiftCount" || name.Name == "__kinmokuseiEvalMapKey") {
			needed = true
		}
		return true
	})
	if !needed {
		return nil, nil
	}
	return parseDeferSupport(`package p
func __kinmokuseiEvalValue[T any](value T)T{return value}
func __kinmokuseiEvalMapKey[K comparable,V any](m map[K]V,key K)K{return key}
func __kinmokuseiEvalShiftCount[T interface{~int|~int8|~int16|~int32|~int64|~uint|~uint8|~uint16|~uint32|~uint64|~uintptr}](value T)T{_ = uint(1)<<value;return value}
`)
}

func generateOrderedAssignment(source *ast.AssignmentStmt, target, value goast.Expr) (goast.Stmt, error) {
	if !evaluationMayHaveEffects(source.Value) {
		return &goast.AssignStmt{Lhs: []goast.Expr{target}, Tok: goAssignmentToken(source.Operator), Rhs: []goast.Expr{value}}, nil
	}
	if _, simple := source.Target.(*ast.IdentifierExpr); simple && (source.Operator == "" || source.Operator == "=") {
		return &goast.AssignStmt{Lhs: []goast.Expr{target}, Tok: token.ASSIGN, Rhs: []goast.Expr{value}}, nil
	}
	if member, ok := source.Target.(*ast.MemberExpr); ok && (source.Operator == "" || source.Operator == "=") {
		if object, ok := member.Object.(*ast.IdentifierExpr); ok && (object.Name == "this" || member.Go && !member.GoField) {
			return &goast.AssignStmt{Lhs: []goast.Expr{target}, Tok: token.ASSIGN, Rhs: []goast.Expr{value}}, nil
		}
	}
	body := &goast.BlockStmt{}
	if indexed, ok := source.Target.(*ast.IndexExpr); ok && indexed.MapIndex {
		index := target.(*goast.IndexExpr)
		m, key := goast.NewIdent("__kinmokuseiAssignmentMap"), goast.NewIdent("__kinmokuseiAssignmentKey")
		body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{m}, Tok: token.DEFINE, Rhs: []goast.Expr{evaluatedValue(index.X)}},
			&goast.AssignStmt{Lhs: []goast.Expr{key}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiEvalMapKey"), Args: []goast.Expr{m, index.Index}}}})
		target = &goast.IndexExpr{X: m, Index: key}
	} else {
		pointer := goast.NewIdent("__kinmokuseiAssignmentTarget")
		body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{pointer}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: target}}})
		target = &goast.StarExpr{X: pointer}
	}
	if source.Operator != "" && source.Operator != "=" {
		old := goast.NewIdent("__kinmokuseiAssignmentValue")
		body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{old}, Tok: token.DEFINE, Rhs: []goast.Expr{target}})
		value = &goast.BinaryExpr{X: old, Op: goToken(source.Operator[:len(source.Operator)-1]), Y: value}
	}
	body.List = append(body.List, &goast.AssignStmt{Lhs: []goast.Expr{target}, Tok: token.ASSIGN, Rhs: []goast.Expr{value}})
	// A zero-result IIFE is also a valid for-post statement.
	return &goast.ExprStmt{X: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: body}}}, nil
}
