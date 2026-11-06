package codegen

import (
	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
	goast "go/ast"
	"go/token"
)

func generateSelectCase(clause *kinmokuseiAST.SelectCase) (*goast.CommClause, error) {
	generated := &goast.CommClause{}
	var prefix []goast.Stmt
	if clause.Kind == kinmokuseiAST.SelectSend {
		channel, err := generateExpression(clause.Channel)
		if err != nil {
			return nil, err
		}
		value, err := generateExpression(clause.Value)
		if err != nil {
			return nil, err
		}
		generated.Comm = &goast.SendStmt{Chan: channel, Value: value}
	} else if clause.Kind == kinmokuseiAST.SelectReceive {
		channel, err := generateExpression(clause.Channel)
		if err != nil {
			return nil, err
		}
		receive := &goast.UnaryExpr{Op: token.ARROW, X: channel}
		if clause.Declare {
			targets := make([]goast.Expr, len(clause.Bindings))
			used := false
			for i, binding := range clause.Bindings {
				name := "_"
				if binding.Name != "_" && binding.Used {
					name = goName(binding.Name)
					used = true
				}
				targets[i] = goast.NewIdent(name)
			}
			if used {
				generated.Comm = &goast.AssignStmt{Lhs: targets, Tok: token.DEFINE, Rhs: []goast.Expr{receive}}
			} else {
				generated.Comm = &goast.ExprStmt{X: receive}
			}
		} else if len(clause.Targets) != 0 {
			var err error
			generated.Comm, prefix, err = generateSelectReceiveAssignment(clause, receive)
			if err != nil {
				return nil, err
			}
		} else {
			generated.Comm = &goast.ExprStmt{X: receive}
		}
	}
	body, err := generateBlock(clause.Body)
	if err != nil {
		return nil, err
	}
	generated.Body = append(prefix, body.List...)
	return generated, nil
}
