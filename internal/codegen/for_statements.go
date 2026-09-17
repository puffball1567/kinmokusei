package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func generateForStatement(stmt *ast.ForStmt) (goast.Stmt, error) {
	result := &goast.ForStmt{}
	var err error
	variable, _ := stmt.Initializer.(*ast.VariableDecl)
	recursive := variable != nil && variable.RecursiveBinding
	if stmt.Initializer != nil && !recursive {
		result.Init, err = generateForClause(stmt.Initializer, true)
		if err != nil {
			return nil, err
		}
	}
	if stmt.Condition != nil {
		result.Cond, err = generateExpression(stmt.Condition)
		if err != nil {
			return nil, err
		}
	}
	if stmt.Post != nil {
		result.Post, err = generateForClause(stmt.Post, false)
		if err != nil {
			return nil, err
		}
	}
	result.Body, err = generateBlock(stmt.Body)
	if err != nil {
		return nil, err
	}
	if recursive {
		return initializeRecursiveForArrow(result, variable)
	}
	return result, nil
}

func initializeRecursiveForArrow(loop *goast.ForStmt, variable *ast.VariableDecl) (goast.Stmt, error) {
	// Validate the same checked metadata as ordinary recursive local storage.
	if _, err := generateLocalArrowStorage(variable); err != nil {
		return nil, err
	}
	assignment, err := generateStatement(variable)
	if err != nil {
		return nil, err
	}
	first := fmt.Sprintf("__kinmokusei_loop_first_%d", variable.Span.Start.Offset)
	// Keep a short declaration in the actual Go for header: Go then creates
	// fresh iteration variables before each post statement. Construct the arrow
	// once, after the first iteration's storage exists but before its condition.
	loop.Init = &goast.AssignStmt{
		Lhs: []goast.Expr{goast.NewIdent(goName(variable.Name))}, Tok: token.DEFINE,
		Rhs: []goast.Expr{&goast.CallExpr{
			Fun: &goast.ParenExpr{X: goType(variable.ResolvedType)}, Args: []goast.Expr{goast.NewIdent("nil")},
		}},
	}
	prefix := []goast.Stmt{&goast.IfStmt{
		Cond: goast.NewIdent(first),
		Body: &goast.BlockStmt{List: []goast.Stmt{
			assignment,
			&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(first)}, Tok: token.ASSIGN, Rhs: []goast.Expr{goast.NewIdent("false")}},
		}},
	}}
	if loop.Cond != nil {
		prefix = append(prefix, &goast.IfStmt{
			Cond: &goast.UnaryExpr{Op: token.NOT, X: loop.Cond},
			Body: &goast.BlockStmt{List: []goast.Stmt{&goast.BranchStmt{Tok: token.BREAK}}},
		})
		loop.Cond = nil
	}
	// Retain the source body's lexical scope, including shadowing and labels.
	loop.Body = &goast.BlockStmt{List: append(prefix, loop.Body)}
	// A single-variable header also avoids a Go 1.23 compiler failure when
	// rewriting captured function variables in multi-variable declarations.
	return &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(first)}, Tok: token.DEFINE, Rhs: []goast.Expr{goast.NewIdent("true")}},
		loop,
	}}, nil
}

func generateLabeledStatement(stmt *ast.LabeledStmt) (goast.Stmt, error) {
	generated, err := generateStatement(stmt.Statement)
	if err != nil {
		return nil, err
	}
	if sourceLoop, ok := stmt.Statement.(*ast.ForStmt); ok {
		if variable, ok := sourceLoop.Initializer.(*ast.VariableDecl); ok && variable.RecursiveBinding {
			block := generated.(*goast.BlockStmt)
			loop := block.List[len(block.List)-1]
			if stmt.LoopBranchLabel == "" {
				block.List[len(block.List)-1] = &goast.LabeledStmt{Label: goast.NewIdent(stmt.Label), Stmt: loop}
				return block, nil
			}
			// A goto restarts initialization, whereas break/continue target the
			// actual loop. Source labels are unique within each callable.
			loopLabel := stmt.LoopBranchLabel
			used := false
			goast.Inspect(loop, func(node goast.Node) bool {
				if _, nestedCallable := node.(*goast.FuncLit); nestedCallable {
					return false
				}
				if branch, ok := node.(*goast.BranchStmt); ok && branch.Label != nil && branch.Label.Name == stmt.Label && (branch.Tok == token.BREAK || branch.Tok == token.CONTINUE) {
					branch.Label = goast.NewIdent(loopLabel)
					used = true
				}
				return true
			})
			if used {
				block.List[len(block.List)-1] = &goast.LabeledStmt{Label: goast.NewIdent(loopLabel), Stmt: loop}
			}
		}
	}
	return &goast.LabeledStmt{Label: goast.NewIdent(stmt.Label), Stmt: generated}, nil
}
