package codegen

import (
	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
	goast "go/ast"
	"go/token"
)

func generateSelectCase(clause *kinmokuseiAST.SelectCase) (*goast.CommClause, error) {
	generated := &goast.CommClause{}
	var prefix []goast.Stmt
	var hiddenFlag *goast.Ident
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
			checkPayload := len(clause.Bindings) == 2 && clause.Bindings[0].NonNull
			used := false
			for i, binding := range clause.Bindings {
				name := goast.NewIdent("_")
				if binding.Name != "_" && (binding.Used || binding.NonNull) {
					name.Name = goName(binding.Name)
					used = true
				} else if checkPayload && i == 1 {
					name.Name = "__kinmokuseiSelectPresent"
					hiddenFlag = name
					used = true
				}
				targets[i] = name
			}
			if used {
				generated.Comm = &goast.AssignStmt{Lhs: targets, Tok: token.DEFINE, Rhs: []goast.Expr{receive}}
			} else {
				generated.Comm = &goast.ExprStmt{X: receive}
			}
			if checkPayload {
				binding := clause.Bindings[0]
				success := referenceSuccess(clause.NonNullCondition, targets[1])
				prefix = append(prefix, generateTypedNonNullCheck(targets[0], binding.ResolvedType, binding.Span, "non-null storage boundary", success))
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
	if hiddenFlag != nil {
		// Go namespace imports can use the compiler's temporary prefix.
		// The communication binding must not capture references in its body.
		used := map[string]bool{}
		goast.Inspect(body, func(node goast.Node) bool {
			if identifier, ok := node.(*goast.Ident); ok {
				used[identifier.Name] = true
			}
			return true
		})
		for used[hiddenFlag.Name] {
			hiddenFlag.Name += "_"
		}
	}
	generated.Body = append(prefix, body.List...)
	return generated, nil
}
