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

func statementDefinitelyReturns(statement ast.Statement) bool {
	if statement == nil {
		return false
	}
	if block, ok := statement.(*ast.BlockStmt); ok {
		return definitelyReturns(block)
	}
	return definitelyReturns(&ast.BlockStmt{Statements: []ast.Statement{statement}})
}

func statementDefinitelyStopsBlock(statement ast.Statement) bool {
	switch statement := statement.(type) {
	case *ast.ReturnStmt, *ast.ThrowStmt, *ast.BranchStmt:
		return true
	case *ast.LabeledStmt:
		return statementDefinitelyStopsBlock(statement.Statement)
	case *ast.BlockStmt:
		for _, nested := range statement.Statements {
			if statementDefinitelyStopsBlock(nested) {
				return true
			}
		}
	case *ast.IfStmt:
		if statement.Else == nil || !statementDefinitelyStopsBlock(statement.Then) {
			return false
		}
		return statementDefinitelyStopsBlock(statement.Else)
	case *ast.TryStmt:
		if statement.FinallyBody != nil && statementDefinitelyStopsBlock(statement.FinallyBody) {
			return true
		}
		if !statementDefinitelyStopsBlock(statement.Body) {
			return false
		}
		for _, clause := range statement.Catches {
			if !statementDefinitelyStopsBlock(clause.Body) {
				return false
			}
		}
		return true
	case *ast.SelectStmt, *ast.ValueSwitchStmt, *ast.TypeSwitchStmt:
		// A break stops its case, not the surrounding block. Reuse the existing
		// all-path return proof for compound breakable statements.
		return statementDefinitelyReturns(statement)
	}
	return false
}

func valueSwitchCaseFallthroughReachable(clause *ast.ValueSwitchCase) bool {
	if clause == nil || !clause.FallsThrough || clause.Body == nil || len(clause.Body.Statements) == 0 {
		return false
	}
	for _, statement := range clause.Body.Statements[:len(clause.Body.Statements)-1] {
		if statementDefinitelyStopsBlock(statement) {
			return false
		}
	}
	return true
}

func definitelyReturns(block *ast.BlockStmt) bool {
	if block == nil || len(block.Statements) == 0 {
		return false
	}
	// Go requires a syntactically terminating final statement, even when an
	// earlier return makes a trailing statement unreachable.
	for _, stmt := range block.Statements[len(block.Statements)-1:] {
		switch stmt := stmt.(type) {
		case *ast.BlockStmt:
			if definitelyReturns(stmt) {
				return true
			}
		case *ast.ReturnStmt:
			return true
		case *ast.ThrowStmt:
			return true
		case *ast.LabeledStmt:
			if statementDefinitelyReturns(stmt.Statement) {
				return true
			}
		case *ast.TryStmt:
			if stmt.Terminal {
				return true
			}
		case *ast.IfStmt:
			if stmt.Else == nil {
				continue
			}
			thenReturns := definitelyReturns(stmt.Then)
			elseReturns := false
			switch branch := stmt.Else.(type) {
			case *ast.BlockStmt:
				elseReturns = definitelyReturns(branch)
			case *ast.IfStmt:
				elseReturns = definitelyReturns(&ast.BlockStmt{Statements: []ast.Statement{branch}})
			}
			if thenReturns && elseReturns {
				return true
			}
		case *ast.SelectStmt:
			if hasSwitchExit(stmt) {
				return false
			}
			if len(stmt.Cases) == 0 {
				return true
			}
			allReturn := true
			for i := range stmt.Cases {
				if !definitelyReturns(stmt.Cases[i].Body) {
					allReturn = false
					break
				}
			}
			if allReturn {
				return true
			}
		case *ast.ValueSwitchStmt:
			if valueSwitchDefinitelyReturns(stmt) {
				return true
			}
		case *ast.TypeSwitchStmt:
			if hasSwitchExit(stmt) {
				return false
			}
			hasDefault := false
			allReturn := len(stmt.Cases) != 0
			for i := range stmt.Cases {
				hasDefault = hasDefault || stmt.Cases[i].Default
				if !definitelyReturns(stmt.Cases[i].Body) {
					allReturn = false
				}
			}
			if hasDefault && allReturn {
				return true
			}
		}
	}
	return false
}

func valueSwitchDefinitelyReturns(statement *ast.ValueSwitchStmt) bool {
	if statement == nil || len(statement.Cases) == 0 || hasSwitchExit(statement) {
		return false
	}
	caseReturns := make([]bool, len(statement.Cases))
	hasDefault := false
	for index := len(statement.Cases) - 1; index >= 0; index-- {
		clause := &statement.Cases[index]
		hasDefault = hasDefault || clause.Default
		caseReturns[index] = definitelyReturns(clause.Body)
		if !caseReturns[index] && index+1 < len(statement.Cases) && valueSwitchCaseFallthroughReachable(clause) {
			caseReturns[index] = caseReturns[index+1]
		}
	}
	if !hasDefault {
		return false
	}
	for _, returns := range caseReturns {
		if !returns {
			return false
		}
	}
	return true
}
