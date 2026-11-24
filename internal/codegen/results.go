package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func functionResults(ref kinmokuseiAST.TypeRef) *goast.FieldList {
	if len(ref.NamedResults) != 0 {
		fields := make([]*goast.Field, len(ref.NamedResults))
		for i, binding := range ref.NamedResults {
			fields[i] = &goast.Field{Names: []*goast.Ident{namedResultIdentifier(binding, i)}, Type: goType(ref.NamedResultType(i))}
		}
		return &goast.FieldList{List: fields}
	}
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

const resultZeroSlot = "__kinmokuseiResultZero"

// Reserve the typed zero in the Go signature before lowering source defers.
// The name of the success type may be hidden by a parameter or local binding
// inside the body. A nested callable owns its own result slots.
func bindResultZeroSlots(file *goast.File) {
	goast.Inspect(file, func(node goast.Node) bool {
		switch callable := node.(type) {
		case *goast.FuncDecl:
			if callable.Body != nil {
				bindResultZeroSlot(callable.Type, callable.Body)
			}
		case *goast.FuncLit:
			bindResultZeroSlot(callable.Type, callable.Body)
		}
		return true
	})
}

func bindResultZeroSlot(signature *goast.FuncType, body *goast.BlockStmt) {
	results := signature.Results
	if results == nil || len(results.List) != 2 {
		return
	}
	var failures []*goast.ReturnStmt
	goast.Inspect(body, func(node goast.Node) bool {
		if returned, ok := node.(*goast.ReturnStmt); ok && len(returned.Results) == 2 {
			if value, ok := returned.Results[0].(*goast.Ident); ok && value.Name == resultZeroSlot {
				failures = append(failures, returned)
			}
		}
		_, nested := node.(*goast.FuncLit)
		return !nested
	})
	if len(failures) == 0 {
		return
	}
	if resultTypeAcceptsNil(results.List[0].Type) {
		for _, returned := range failures {
			returned.Results[0] = goast.NewIdent("nil")
		}
		return
	}
	results.List[0].Names = []*goast.Ident{goast.NewIdent(resultZeroSlot)}
	results.List[1].Names = []*goast.Ident{goast.NewIdent("__kinmokuseiResultError")}
}

func resultTypeAcceptsNil(result goast.Expr) bool {
	switch result := result.(type) {
	case *goast.StarExpr, *goast.MapType, *goast.ChanType, *goast.InterfaceType, *goast.FuncType:
		return true
	case *goast.ArrayType:
		return result.Len == nil
	}
	return false
}

func generateResultReturn(stmt *kinmokuseiAST.ReturnStmt) (goast.Stmt, error) {
	if stmt.ResultKind == kinmokuseiAST.ResultMatchReturn {
		return generateMatchResultReturn(stmt)
	}
	if stmt.CrossesTry {
		return generateExceptionReturn(stmt)
	}
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
			results = append(results, goast.NewIdent(resultZeroSlot))
		}
		results = append(results, errorValue)
		return &goast.ReturnStmt{Results: results}, nil
	case kinmokuseiAST.ResultForwardReturn:
		return generateForwardedResultReturn(stmt)
	default:
		return nil, fmt.Errorf("unknown Result return kind %d", stmt.ResultKind)
	}
}

func generateExceptionReturn(stmt *kinmokuseiAST.ReturnStmt) (goast.Stmt, error) {
	if stmt.NamedResultCount != 0 {
		return generateNamedExceptionReturn(stmt)
	}
	returnPanic := exceptionReturnPanic

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
		return generateForwardedResultReturn(stmt)
	default:
		return nil, fmt.Errorf("unknown Result return kind %d", stmt.ResultKind)
	}
}

func generateForwardedResultReturn(stmt *kinmokuseiAST.ReturnStmt) (goast.Stmt, error) {
	producer, err := generateExpression(stmt.Value)
	if err != nil {
		return nil, err
	}
	if !stmt.NonNullResult && !stmt.CrossesTry {
		return &goast.ReturnStmt{Results: []goast.Expr{producer}}, nil
	}
	// Result forwarding requires an exact source contract. Inferred captures
	// already have the target's storage and interface boxing, so validation
	// can avoid spelling a type name shadowed in the source body.
	errorValue := goast.NewIdent("err")
	bindings := []goast.Expr{errorValue}
	var value goast.Expr
	if stmt.ResultType.Name != "void" {
		value = goast.NewIdent("value")
		bindings = append([]goast.Expr{value}, bindings...)
	}
	body := &goast.BlockStmt{List: []goast.Stmt{
		&goast.AssignStmt{Lhs: bindings, Tok: token.DEFINE, Rhs: []goast.Expr{producer}},
	}}
	if stmt.NonNullResult {
		success := referenceSuccess(kinmokuseiAST.NonNullIfNoError, errorValue)
		body.List = append(body.List, generateTypedNonNullCheck(value, stmt.ResultType, stmt.Value.GetSpan(), "non-null result", success))
	}
	if stmt.CrossesTry {
		body.List = append(body.List, exceptionReturnPanic(value, errorValue))
	} else {
		body.List = append(body.List, &goast.ReturnStmt{Results: bindings})
	}
	return body, nil
}

func exceptionReturnPanic(value, errValue goast.Expr) goast.Stmt {
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
	var exit goast.Stmt
	if expr.CrossesTry {
		exit = exceptionReturnPanic(nil, goast.NewIdent(errorName))
	} else {
		results := []goast.Expr{}
		if expr.ResultType.Name != "void" {
			results = append(results, goast.NewIdent(resultZeroSlot))
		}
		results = append(results, goast.NewIdent(errorName))
		exit = &goast.ReturnStmt{Results: results}
	}
	propagate := &goast.IfStmt{
		Cond: &goast.BinaryExpr{X: goast.NewIdent(errorName), Op: token.NEQ, Y: goast.NewIdent("nil")},
		Body: &goast.BlockStmt{List: []goast.Stmt{exit}},
	}
	statements := []goast.Stmt{declaration, propagate}
	if explicitType && expr.ValueType.Name != "void" {
		binding := &goast.DeclStmt{Decl: &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{&goast.ValueSpec{
			Names: []*goast.Ident{goast.NewIdent(goName(variable.Name))}, Type: goType(variable.Type), Values: []goast.Expr{goast.NewIdent(valueName)},
		}}}}
		statements = append(statements, binding)
	}
	if expr.NonNull {
		// The error branch has exited and an explicit destination has already
		// performed interface boxing. Validate before later source statements.
		statements = append(statements, generateTypedNonNullCheck(
			goast.NewIdent(goName(variable.Name)), expr.ValueType, expr.Span, "non-null storage boundary", nil,
		))
	}
	return statements, nil
}

func zeroValue(ref kinmokuseiAST.TypeRef) goast.Expr {
	return &goast.StarExpr{X: &goast.CallExpr{Fun: goast.NewIdent("new"), Args: []goast.Expr{goType(ref)}}}
}
