package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func generateBlock(block *kinmokuseiAST.BlockStmt) (*goast.BlockStmt, error) {
	result := &goast.BlockStmt{}
	groupEnd := 0
	for index, stmt := range block.Statements {
		if index >= groupEnd {
			group := kinmokuseiAST.LocalArrowGroup(block.Statements[index:])
			groupEnd = index + len(group)
			for _, variable := range group {
				if !variable.RecursiveBinding {
					continue
				}
				declaration, err := generateLocalArrowStorage(variable)
				if err != nil {
					return nil, err
				}
				result.List = append(result.List, declaration)
			}
		}
		if variable, ok := stmt.(*kinmokuseiAST.VariableDecl); ok {
			if propagated, ok := variable.Value.(*kinmokuseiAST.PropagateExpr); ok {
				generated, err := generatePropagationStatements(propagated, variable)
				if err != nil {
					return nil, err
				}
				result.List = append(result.List, generated...)
				if variable.Name != "_" && !variable.Used {
					result.List = append(result.List, &goast.AssignStmt{
						Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN,
						Rhs: []goast.Expr{goast.NewIdent(goName(variable.Name))},
					})
				}
				continue
			}
		}
		if expression, ok := stmt.(*kinmokuseiAST.ExpressionStmt); ok {
			if propagated, ok := expression.Value.(*kinmokuseiAST.PropagateExpr); ok {
				generated, err := generatePropagationStatements(propagated, nil)
				if err != nil {
					return nil, err
				}
				result.List = append(result.List, generated...)
				continue
			}
		}
		generated, err := generateStatement(stmt)
		if err != nil {
			return nil, err
		}
		result.List = append(result.List, generated)
		if variable, ok := stmt.(*kinmokuseiAST.VariableDecl); ok && variable.Name != "_" && !variable.Used {
			result.List = append(result.List, &goast.AssignStmt{
				Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN,
				Rhs: []goast.Expr{goast.NewIdent(goName(variable.Name))},
			})
		}
		if declaration, ok := stmt.(*kinmokuseiAST.MultiVariableDecl); ok {
			for _, binding := range declaration.Bindings {
				if binding.Name == "_" || binding.Used {
					continue
				}
				result.List = append(result.List, &goast.AssignStmt{
					Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN,
					Rhs: []goast.Expr{goast.NewIdent(goName(binding.Name))},
				})
			}
		}
	}
	return result, nil
}

func generateStatement(stmt kinmokuseiAST.Statement) (goast.Stmt, error) {
	switch stmt := stmt.(type) {
	case *kinmokuseiAST.VariableDecl:
		if stmt.DiscardArity > 0 {
			return generateDiscard(stmt.Value, stmt.DiscardArity, stmt.Type)
		}
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		if stmt.RecursiveBinding {
			return &goast.AssignStmt{
				Lhs: []goast.Expr{goast.NewIdent(goName(stmt.Name))}, Tok: token.ASSIGN,
				Rhs: []goast.Expr{value},
			}, nil
		}
		tok := token.VAR
		if stmt.Constant && isGoConstant(stmt.Value) {
			tok = token.CONST
		}
		spec := &goast.ValueSpec{Names: []*goast.Ident{goast.NewIdent(goName(stmt.Name))}, Values: []goast.Expr{value}}
		if stmt.Type.Name != "" || stmt.Type.IsFunction() || stmt.Type.IsPointer() || stmt.Type.IsArray() {
			spec.Type = goType(stmt.Type)
		}
		return &goast.DeclStmt{Decl: &goast.GenDecl{Tok: tok, Specs: []goast.Spec{spec}}}, nil
	case *kinmokuseiAST.MultiVariableDecl:
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		names := make([]*goast.Ident, len(stmt.Bindings))
		for i, binding := range stmt.Bindings {
			names[i] = goast.NewIdent(goName(binding.Name))
		}
		spec := &goast.ValueSpec{Names: names, Values: []goast.Expr{value}}
		return &goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{spec}}}, nil
	case *kinmokuseiAST.ReturnStmt:
		if stmt.CrossesTry {
			return generateExceptionReturn(stmt)
		}
		if stmt.ResultKind != kinmokuseiAST.NormalReturn {
			return generateResultReturn(stmt)
		}
		result := &goast.ReturnStmt{}
		if stmt.Value != nil {
			value, err := generateExpression(stmt.Value)
			if err != nil {
				return nil, err
			}
			result.Results = []goast.Expr{value}
		}
		for _, expression := range stmt.AdditionalValues {
			value, err := generateExpression(expression)
			if err != nil {
				return nil, err
			}
			result.Results = append(result.Results, value)
		}
		return result, nil
	case *kinmokuseiAST.ThrowStmt:
		if stmt.Bare {
			return &goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{
				goast.NewIdent(fmt.Sprintf("__kinmokusei_thrown_%d", stmt.RethrowOffset)),
			}}}, nil
		}
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		thrown := &goast.CompositeLit{Type: goast.NewIdent("__kinmokuseiThrown"), Elts: []goast.Expr{
			&goast.KeyValueExpr{Key: goast.NewIdent("err"), Value: value},
		}}
		return &goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{thrown}}}, nil
	case *kinmokuseiAST.TryStmt:
		return generateTryStatement(stmt)
	case *kinmokuseiAST.IfStmt:
		condition, err := generateExpression(stmt.Condition)
		if err != nil {
			return nil, err
		}
		then, err := generateBlock(stmt.Then)
		if err != nil {
			return nil, err
		}
		result := &goast.IfStmt{Cond: condition, Body: then}
		if stmt.Else != nil {
			result.Else, err = generateStatement(stmt.Else)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	case *kinmokuseiAST.BlockStmt:
		return generateBlock(stmt)
	case *kinmokuseiAST.ExpressionStmt:
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		return &goast.ExprStmt{X: value}, nil
	case *kinmokuseiAST.AssignmentStmt:
		if member, ok := stmt.Target.(*kinmokuseiAST.MemberExpr); ok && member.Property {
			return generatePropertyAssignment(member, stmt.Operator, stmt.Value)
		}
		if stmt.DiscardArity > 0 {
			return generateDiscard(stmt.Value, stmt.DiscardArity, kinmokuseiAST.TypeRef{})
		}
		target, err := generateExpression(stmt.Target)
		if err != nil {
			return nil, err
		}
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		return &goast.AssignStmt{Lhs: []goast.Expr{target}, Tok: goAssignmentToken(stmt.Operator), Rhs: []goast.Expr{value}}, nil
	case *kinmokuseiAST.IncDecStmt:
		if member, ok := stmt.Target.(*kinmokuseiAST.MemberExpr); ok && member.Property {
			return generatePropertyAssignment(member, stmt.Operator, nil)
		}
		target, err := generateExpression(stmt.Target)
		if err != nil {
			return nil, err
		}
		operator := token.INC
		if stmt.Operator == "--" {
			operator = token.DEC
		}
		return &goast.IncDecStmt{X: target, Tok: operator}, nil
	case *kinmokuseiAST.MultiAssignmentStmt:
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		targets := make([]goast.Expr, len(stmt.Bindings))
		for i, binding := range stmt.Bindings {
			targets[i] = goast.NewIdent(goName(binding.Name))
			if binding.GoMember != nil {
				targets[i], err = generateExpression(binding.GoMember)
				if err != nil {
					return nil, err
				}
			}
		}
		return &goast.AssignStmt{Lhs: targets, Tok: token.ASSIGN, Rhs: []goast.Expr{value}}, nil
	case *kinmokuseiAST.WhileStmt:
		condition, err := generateExpression(stmt.Condition)
		if err != nil {
			return nil, err
		}
		body, err := generateBlock(stmt.Body)
		if err != nil {
			return nil, err
		}
		return &goast.ForStmt{Cond: condition, Body: body}, nil
	case *kinmokuseiAST.ForStmt:
		return generateForStatement(stmt)
	case *kinmokuseiAST.ForRangeStmt:
		source, err := generateExpression(stmt.Source)
		if err != nil {
			return nil, err
		}
		body, err := generateBlock(stmt.Body)
		if err != nil {
			return nil, err
		}
		result := &goast.RangeStmt{X: source, Body: body}
		for index := len(stmt.Bindings) - 1; index >= 0; index-- {
			binding := stmt.Bindings[index]
			if binding.Name == "_" || !binding.Assigned || binding.Used {
				continue
			}
			body.List = append([]goast.Stmt{&goast.AssignStmt{
				Lhs: []goast.Expr{goast.NewIdent("_")}, Tok: token.ASSIGN,
				Rhs: []goast.Expr{goast.NewIdent(goName(binding.Name))},
			}}, body.List...)
		}
		if stmt.Kind == kinmokuseiAST.ChannelRange || stmt.Kind == kinmokuseiAST.IntegerRange || stmt.Kind == kinmokuseiAST.IteratorRange {
			if len(stmt.Bindings) == 1 && rangeBindingNeeded(stmt.Bindings[0]) {
				result.Key = goast.NewIdent(goName(stmt.Bindings[0].Name))
				result.Tok = token.DEFINE
			}
			return result, nil
		}
		if len(stmt.Bindings) == 1 {
			binding := stmt.Bindings[0]
			if rangeBindingNeeded(binding) {
				result.Key = goast.NewIdent("_")
				result.Value = goast.NewIdent(goName(binding.Name))
				result.Tok = token.DEFINE
			}
			return result, nil
		}
		if len(stmt.Bindings) == 2 {
			key, value := stmt.Bindings[0], stmt.Bindings[1]
			if rangeBindingNeeded(value) {
				result.Key = goast.NewIdent("_")
				if rangeBindingNeeded(key) {
					result.Key = goast.NewIdent(goName(key.Name))
				}
				result.Value = goast.NewIdent(goName(value.Name))
				result.Tok = token.DEFINE
			} else if rangeBindingNeeded(key) {
				result.Key = goast.NewIdent(goName(key.Name))
				result.Tok = token.DEFINE
			}
		}
		return result, nil
	case *kinmokuseiAST.SelectStmt:
		body := &goast.BlockStmt{}
		for i := range stmt.Cases {
			clause, err := generateSelectCase(&stmt.Cases[i])
			if err != nil {
				return nil, err
			}
			body.List = append(body.List, clause)
		}
		return &goast.SelectStmt{Body: body}, nil
	case *kinmokuseiAST.ValueSwitchStmt:
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		body := &goast.BlockStmt{}
		for i := range stmt.Cases {
			clause, err := generateValueSwitchCase(&stmt.Cases[i])
			if err != nil {
				return nil, err
			}
			body.List = append(body.List, clause)
		}
		return &goast.SwitchStmt{Tag: value, Body: body}, nil
	case *kinmokuseiAST.TypeSwitchStmt:
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		guardName := fmt.Sprintf("__kinmokusei_type_switch_%d", stmt.Span.Start.Offset)
		guardUsed := false
		body := &goast.BlockStmt{}
		for i := range stmt.Cases {
			guardUsed = guardUsed || stmt.Cases[i].Used
			clause, err := generateTypeSwitchCase(&stmt.Cases[i], guardName)
			if err != nil {
				return nil, err
			}
			body.List = append(body.List, clause)
		}
		assertion := &goast.TypeAssertExpr{X: value}
		var guard goast.Stmt = &goast.ExprStmt{X: assertion}
		if guardUsed {
			guard = &goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(guardName)}, Tok: token.DEFINE, Rhs: []goast.Expr{assertion}}
		}
		return &goast.TypeSwitchStmt{Assign: guard, Body: body}, nil
	case *kinmokuseiAST.BranchStmt:
		branch := token.BREAK
		if stmt.Kind == kinmokuseiAST.ContinueBranch {
			branch = token.CONTINUE
		} else if stmt.Kind == kinmokuseiAST.GotoBranch {
			branch = token.GOTO
		} else if stmt.Kind == kinmokuseiAST.FallthroughBranch {
			branch = token.FALLTHROUGH
		}
		generated := &goast.BranchStmt{Tok: branch}
		if stmt.Label != "" {
			generated.Label = goast.NewIdent(stmt.Label)
		}
		return generated, nil
	case *kinmokuseiAST.LabeledStmt:
		return generateLabeledStatement(stmt)
	case *kinmokuseiAST.CallControlStmt:
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		call, ok := value.(*goast.CallExpr)
		if !ok {
			return nil, fmt.Errorf("call-control statement at %v does not contain a call", stmt.Span.Start)
		}
		if stmt.Kind == kinmokuseiAST.GoCall {
			return &goast.GoStmt{Call: call}, nil
		}
		return &goast.DeferStmt{Call: call}, nil
	case *kinmokuseiAST.DetachStmt:
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		taskType := taskTypeFromAnnotation(stmt.ValueType, stmt.ResultTask, stmt.Void)
		body := &goast.BlockStmt{List: []goast.Stmt{
			&goast.ExprStmt{X: &goast.UnaryExpr{Op: token.ARROW, X: &goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("done")}}},
			&goast.IfStmt{Cond: &goast.BinaryExpr{X: &goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("panicValue")}, Op: token.NEQ, Y: goast.NewIdent("nil")}, Body: &goast.BlockStmt{List: []goast.Stmt{
				&goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{&goast.SelectorExpr{X: goast.NewIdent("task"), Sel: goast.NewIdent("panicValue")}}}},
			}}},
		}}
		return &goast.GoStmt{Call: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{List: []*goast.Field{{Names: []*goast.Ident{goast.NewIdent("task")}, Type: taskType}}}}, Body: body}, Args: []goast.Expr{value}}}, nil
	case *kinmokuseiAST.ChannelSendStmt:
		channel, err := generateExpression(stmt.Channel)
		if err != nil {
			return nil, err
		}
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		return &goast.SendStmt{Chan: channel, Value: value}, nil
	default:
		return nil, fmt.Errorf("unsupported statement %T", stmt)
	}
}

func generateValueSwitchCase(clause *kinmokuseiAST.ValueSwitchCase) (*goast.CaseClause, error) {
	generated := &goast.CaseClause{}
	for _, value := range clause.Values {
		expression, err := generateExpression(value)
		if err != nil {
			return nil, err
		}
		generated.List = append(generated.List, expression)
	}
	body, err := generateBlock(clause.Body)
	if err != nil {
		return nil, err
	}
	generated.Body = body.List
	return generated, nil
}

func generateSelectCase(clause *kinmokuseiAST.SelectCase) (*goast.CommClause, error) {
	generated := &goast.CommClause{}
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
			targets := make([]goast.Expr, len(clause.Targets))
			for i, target := range clause.Targets {
				targets[i], err = generateExpression(target)
				if err != nil {
					return nil, err
				}
			}
			generated.Comm = &goast.AssignStmt{Lhs: targets, Tok: token.ASSIGN, Rhs: []goast.Expr{receive}}
		} else {
			generated.Comm = &goast.ExprStmt{X: receive}
		}
	}
	body, err := generateBlock(clause.Body)
	if err != nil {
		return nil, err
	}
	generated.Body = body.List
	return generated, nil
}

func generateTypeSwitchCase(clause *kinmokuseiAST.TypeSwitchCase, guardName string) (*goast.CaseClause, error) {
	generated := &goast.CaseClause{}
	if clause.Nil {
		generated.List = []goast.Expr{goast.NewIdent("nil")}
	} else if !clause.Default {
		generated.List = []goast.Expr{goType(clause.Type)}
	}
	body, err := generateBlock(clause.Body)
	if err != nil {
		return nil, err
	}
	if !clause.Nil && !clause.Default && clause.Name != "_" && clause.Used {
		binding := &goast.AssignStmt{
			Lhs: []goast.Expr{goast.NewIdent(goName(clause.Name))}, Tok: token.DEFINE,
			Rhs: []goast.Expr{goast.NewIdent(guardName)},
		}
		generated.Body = append(generated.Body, binding)
	}
	generated.Body = append(generated.Body, body.List...)
	return generated, nil
}

func generateForClause(stmt kinmokuseiAST.Statement, initializer bool) (goast.Stmt, error) {
	if variable, ok := stmt.(*kinmokuseiAST.VariableDecl); ok {
		if variable.DiscardArity > 0 {
			return generateDiscard(variable.Value, variable.DiscardArity, variable.Type)
		}
		value, err := generateExpression(variable.Value)
		if err != nil {
			return nil, err
		}
		name := goName(variable.Name)
		operator := token.DEFINE
		if !variable.Used {
			name = "_"
			operator = token.ASSIGN
		}
		return &goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent(name)}, Tok: operator, Rhs: []goast.Expr{value}}, nil
	}
	if declaration, ok := stmt.(*kinmokuseiAST.MultiVariableDecl); ok {
		value, err := generateExpression(declaration.Value)
		if err != nil {
			return nil, err
		}
		targets := make([]goast.Expr, len(declaration.Bindings))
		hasName := false
		for i, binding := range declaration.Bindings {
			name := binding.Name
			if name != "_" && !binding.Used {
				name = "_"
			}
			if name != "_" {
				hasName = true
			}
			targets[i] = goast.NewIdent(goName(name))
		}
		operator := token.DEFINE
		if !hasName {
			operator = token.ASSIGN
		}
		return &goast.AssignStmt{Lhs: targets, Tok: operator, Rhs: []goast.Expr{value}}, nil
	}
	generated, err := generateStatement(stmt)
	if err != nil {
		return nil, err
	}
	switch generated.(type) {
	case *goast.AssignStmt, *goast.IncDecStmt, *goast.ExprStmt, *goast.SendStmt:
		return generated, nil
	default:
		position := "post"
		if initializer {
			position = "initializer"
		}
		return nil, fmt.Errorf("unsupported for %s %T", position, stmt)
	}
}

func rangeBindingNeeded(binding kinmokuseiAST.RangeBinding) bool {
	return binding.Name != "_" && (binding.Used || binding.Assigned)
}
