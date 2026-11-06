package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Selection receives first, then evaluates every LHS before performing writes.
// Inferred temporaries retain private Go types. Only the selected case runs
// these statements; property setters cannot change a later target's address.
func generateSelectReceiveAssignment(clause *ast.SelectCase, receive goast.Expr) (goast.Stmt, []goast.Stmt, error) {
	// Plain variable targets need no conversion or operand snapshots. Retain
	// Go's native communication assignment for this common case.
	direct := true
	for i, target := range clause.Targets {
		if _, ok := target.(*ast.IdentifierExpr); !ok {
			property, isMember := target.(*ast.MemberExpr)
			if len(clause.Targets) != 1 || isMember && property.Property {
				direct = false
			}
		}
		if i < len(clause.Upcasts) && clause.Upcasts[i] != nil {
			direct = false
		}
	}
	if direct {
		targets := make([]goast.Expr, len(clause.Targets))
		for i, target := range clause.Targets {
			generated, err := generateExpression(target)
			if err != nil {
				return nil, nil, err
			}
			targets[i] = generated
		}
		return &goast.AssignStmt{Lhs: targets, Tok: token.ASSIGN, Rhs: []goast.Expr{receive}}, nil, nil
	}
	bindings := make([]goast.Expr, len(clause.Targets))
	var prepare, writes []goast.Stmt
	used := false
	for i, source := range clause.Targets {
		if identifier, ok := source.(*ast.IdentifierExpr); ok && identifier.Name == "_" {
			bindings[i] = goast.NewIdent("_")
			continue
		}
		used = true
		name := fmt.Sprintf("__kinmokuseiSelectValue%d", i)
		bindings[i] = goast.NewIdent(name)
		value := goast.Expr(bindings[i])
		if i < len(clause.Upcasts) && clause.Upcasts[i] != nil {
			conversion := *clause.Upcasts[i]
			conversion.Value = &ast.IdentifierExpr{Name: name, Span: source.GetSpan()}
			var err error
			value, err = generateExpression(&conversion)
			if err != nil {
				return nil, nil, err
			}
		}
		if property, ok := source.(*ast.MemberExpr); ok && property.Property {
			receiver, err := generatePropertyReceiver(property)
			if err != nil {
				return nil, nil, err
			}
			if receiver != nil {
				stored := goast.NewIdent(fmt.Sprintf("__kinmokuseiSelectReceiver%d", i))
				prepare = append(prepare, &goast.AssignStmt{Lhs: []goast.Expr{stored}, Tok: token.DEFINE, Rhs: []goast.Expr{receiver}})
				receiver = stored
			}
			writes = append(writes, &goast.ExprStmt{X: propertyCall(receiver, propertyAccessorName(property, true), value)})
			continue
		}
		target, err := generateExpression(source)
		if err != nil {
			return nil, nil, err
		}
		if index, ok := source.(*ast.IndexExpr); ok && index.MapIndex {
			indexed := target.(*goast.IndexExpr)
			m := goast.NewIdent(fmt.Sprintf("__kinmokuseiSelectMap%d", i))
			key := goast.NewIdent(fmt.Sprintf("__kinmokuseiSelectKey%d", i))
			prepare = append(prepare, &goast.AssignStmt{Lhs: []goast.Expr{m}, Tok: token.DEFINE, Rhs: []goast.Expr{evaluatedValue(indexed.X)}}, &goast.AssignStmt{Lhs: []goast.Expr{key}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.CallExpr{Fun: goast.NewIdent("__kinmokuseiEvalMapKey"), Args: []goast.Expr{m, indexed.Index}}}})
			target = &goast.IndexExpr{X: m, Index: key}
		} else {
			pointer := goast.NewIdent(fmt.Sprintf("__kinmokuseiSelectTarget%d", i))
			prepare = append(prepare, &goast.AssignStmt{Lhs: []goast.Expr{pointer}, Tok: token.DEFINE, Rhs: []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: target}}})
			target = &goast.StarExpr{X: pointer}
		}
		writes = append(writes, &goast.AssignStmt{Lhs: []goast.Expr{target}, Tok: token.ASSIGN, Rhs: []goast.Expr{value}})
	}
	if !used {
		return &goast.ExprStmt{X: receive}, nil, nil
	}
	return &goast.AssignStmt{Lhs: bindings, Tok: token.DEFINE, Rhs: []goast.Expr{receive}}, append(prepare, writes...), nil
}
