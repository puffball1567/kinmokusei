package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// A switch/select cannot prove a function returns if a break can leave it.
// Inspect statement bodies only: a callback has an independent control flow.
func hasSwitchExit(target ast.Statement) bool {
	span := target.GetSpan()
	var visit func(ast.Statement, int) bool
	visit = func(statement ast.Statement, depth int) bool {
		switch statement := statement.(type) {
		case *ast.BranchStmt:
			if statement.Kind != ast.BreakBranch {
				return false
			}
			if statement.Label == "" {
				return depth == 0
			}
			// A label declared inside the target belongs to a nested construct.
			position := statement.ResolvedDeclaration.Start.Offset
			return position < span.Start.Offset || position >= span.End.Offset
		case *ast.BlockStmt:
			for _, child := range statement.Statements {
				if visit(child, depth) {
					return true
				}
			}
		case *ast.LabeledStmt:
			return visit(statement.Statement, depth)
		case *ast.IfStmt:
			return visit(statement.Then, depth) || visit(statement.Else, depth)
		case *ast.WhileStmt:
			return visit(statement.Body, depth+1)
		case *ast.ForStmt:
			return visit(statement.Body, depth+1)
		case *ast.ForRangeStmt:
			return visit(statement.Body, depth+1)
		case *ast.ValueSwitchStmt:
			for _, clause := range statement.Cases {
				if visit(clause.Body, depth+1) {
					return true
				}
			}
		case *ast.TypeSwitchStmt:
			for _, clause := range statement.Cases {
				if visit(clause.Body, depth+1) {
					return true
				}
			}
		case *ast.SelectStmt:
			for _, clause := range statement.Cases {
				if visit(clause.Body, depth+1) {
					return true
				}
			}
		case *ast.TryStmt:
			if visit(statement.Body, depth) {
				return true
			}
			for _, clause := range statement.Catches {
				if visit(clause.Body, depth) {
					return true
				}
			}
			if statement.FinallyBody != nil {
				return visit(statement.FinallyBody, depth)
			}
		}
		return false
	}
	return visit(target, -1)
}
