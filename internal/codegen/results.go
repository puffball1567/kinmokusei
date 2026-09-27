package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func functionResults(ref kinmokuseiAST.TypeRef) *goast.FieldList {
	if len(ref.GoResults) != 0 {
		fields := make([]*goast.Field, len(ref.GoResults))
		for i, result := range ref.GoResults {
			fields[i] = &goast.Field{Type: goType(result)}
		}
		return &goast.FieldList{List: fields}
	}
	if ref.Name == "void" {
		return nil
	}
	if ref.Name == "Result" && len(ref.GenericArguments) == 1 {
		element := ref.GenericArguments[0]
		fields := []*goast.Field{}
		if element.Name != "void" {
			fields = append(fields, &goast.Field{Type: goType(element)})
		}
		fields = append(fields, &goast.Field{Type: goast.NewIdent("error")})
		return &goast.FieldList{List: fields}
	}
	return &goast.FieldList{List: []*goast.Field{{Type: goType(ref)}}}
}

func generateResultReturn(stmt *kinmokuseiAST.ReturnStmt) (goast.Stmt, error) {
	call, isCall := stmt.Value.(*kinmokuseiAST.CallExpr)
	switch stmt.ResultKind {
	case kinmokuseiAST.ResultSuccessReturn:
		if !isCall || call.Builtin != kinmokuseiAST.ResultOKCall {
			return nil, fmt.Errorf("Result success return has an invalid expression")
		}
		results := []goast.Expr{}
		if stmt.ResultType.Name != "void" {
			if len(call.Arguments) != 1 {
				return nil, fmt.Errorf("non-void Result success requires one value")
			}
			value, err := generateExpression(call.Arguments[0])
			if err != nil {
				return nil, err
			}
			results = append(results, value)
		}
		results = append(results, goast.NewIdent("nil"))
		return &goast.ReturnStmt{Results: results}, nil
	case kinmokuseiAST.ResultFailureReturn:
		if !isCall || call.Builtin != kinmokuseiAST.ResultFailCall || len(call.Arguments) != 1 {
			return nil, fmt.Errorf("Result failure return requires one error")
		}
		errorValue, err := generateExpression(call.Arguments[0])
		if err != nil {
			return nil, err
		}
		results := []goast.Expr{}
		if stmt.ResultType.Name != "void" {
			results = append(results, zeroValue(stmt.ResultType))
		}
		results = append(results, errorValue)
		return &goast.ReturnStmt{Results: results}, nil
	case kinmokuseiAST.ResultForwardReturn:
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		return &goast.ReturnStmt{Results: []goast.Expr{value}}, nil
	default:
		return nil, fmt.Errorf("unknown Result return kind %d", stmt.ResultKind)
	}
}

func generateExceptionReturn(stmt *kinmokuseiAST.ReturnStmt) (goast.Stmt, error) {
	returnPanic := func(value, errValue goast.Expr) goast.Stmt {
		fields := []goast.Expr{}
		if value != nil {
			fields = append(fields, &goast.KeyValueExpr{Key: goast.NewIdent("value"), Value: value})
		}
		if errValue != nil {
			fields = append(fields, &goast.KeyValueExpr{Key: goast.NewIdent("err"), Value: errValue})
		}
		return &goast.ExprStmt{X: &goast.CallExpr{Fun: goast.NewIdent("panic"), Args: []goast.Expr{
			&goast.CompositeLit{Type: goast.NewIdent("__kinmokuseiReturn"), Elts: fields},
		}}}
	}

	if stmt.ResultKind == kinmokuseiAST.NormalReturn {
		if len(stmt.ResultType.GoResults) != 0 {
			return generateMultipleExceptionReturn(stmt)
		}
		if stmt.Value == nil {
			return returnPanic(nil, nil), nil
		}
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		return returnPanic(value, nil), nil
	}

	call, isCall := stmt.Value.(*kinmokuseiAST.CallExpr)
	switch stmt.ResultKind {
	case kinmokuseiAST.ResultSuccessReturn:
		if !isCall || call.Builtin != kinmokuseiAST.ResultOKCall {
			return nil, fmt.Errorf("Result success return has an invalid expression")
		}
		if stmt.ResultType.Name == "void" {
			return returnPanic(nil, goast.NewIdent("nil")), nil
		}
		if len(call.Arguments) != 1 {
			return nil, fmt.Errorf("non-void Result success requires one value")
		}
		value, err := generateExpression(call.Arguments[0])
		if err != nil {
			return nil, err
		}
		return returnPanic(value, goast.NewIdent("nil")), nil
	case kinmokuseiAST.ResultFailureReturn:
		if !isCall || call.Builtin != kinmokuseiAST.ResultFailCall || len(call.Arguments) != 1 {
			return nil, fmt.Errorf("Result failure return requires one error")
		}
		errorValue, err := generateExpression(call.Arguments[0])
		if err != nil {
			return nil, err
		}
		return returnPanic(nil, errorValue), nil
	case kinmokuseiAST.ResultForwardReturn:
		value, err := generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}
		body := &goast.BlockStmt{}
		if stmt.ResultType.Name == "void" {
			body.List = append(body.List,
				&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("err")}, Tok: token.DEFINE, Rhs: []goast.Expr{value}},
				returnPanic(nil, goast.NewIdent("err")),
			)
		} else {
			body.List = append(body.List,
				&goast.AssignStmt{Lhs: []goast.Expr{goast.NewIdent("value"), goast.NewIdent("err")}, Tok: token.DEFINE, Rhs: []goast.Expr{value}},
				returnPanic(goast.NewIdent("value"), goast.NewIdent("err")),
			)
		}
		return &goast.ExprStmt{X: &goast.CallExpr{Fun: &goast.FuncLit{Type: &goast.FuncType{Params: &goast.FieldList{}}, Body: body}}}, nil
	default:
		return nil, fmt.Errorf("unknown Result return kind %d", stmt.ResultKind)
	}
}

func generatePropagationStatements(expr *kinmokuseiAST.PropagateExpr, variable *kinmokuseiAST.VariableDecl) ([]goast.Stmt, error) {
	value, err := generateExpression(expr.Value)
	if err != nil {
		return nil, err
	}
	errorName := expr.ErrorName
	if errorName == "" {
		errorName = fmt.Sprintf("__kinmokusei_result_error_%d", expr.Span.Start.Offset)
	}
	names := []*goast.Ident{}
	valueName := ""
	explicitType := variable != nil && variable.Type.IsSpecified()
	if expr.ValueType.Name != "void" {
		if variable == nil {
			return nil, fmt.Errorf("non-void result propagation requires a variable")
		}
		valueName = goName(variable.Name)
		if explicitType {
			valueName = fmt.Sprintf("__kinmokusei_result_value_%d", expr.Span.Start.Offset)
		}
		names = append(names, goast.NewIdent(valueName))
	}
	names = append(names, goast.NewIdent(errorName))
	declaration := &goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
		Names: names, Values: []goast.Expr{value},
	}}}}
	results := []goast.Expr{}
	if expr.ResultType.Name != "void" {
		results = append(results, zeroValue(expr.ResultType))
	}
	results = append(results, goast.NewIdent(errorName))
	propagate := &goast.IfStmt{
		Cond: &goast.BinaryExpr{X: goast.NewIdent(errorName), Op: token.NEQ, Y: goast.NewIdent("nil")},
		Body: &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: results}}},
	}
	statements := []goast.Stmt{declaration, propagate}
	if explicitType && expr.ValueType.Name != "void" {
		binding := &goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
			Names: []*goast.Ident{goast.NewIdent(goName(variable.Name))}, Type: goType(variable.Type), Values: []goast.Expr{goast.NewIdent(valueName)},
		}}}}
		statements = append(statements, binding)
	}
	return statements, nil
}

func zeroValue(ref kinmokuseiAST.TypeRef) goast.Expr {
	return &goast.StarExpr{X: &goast.CallExpr{Fun: goast.NewIdent("new"), Args: []goast.Expr{goType(ref)}}}
}
