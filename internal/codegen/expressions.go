package codegen

import (
	"fmt"
	goast "go/ast"
	"go/token"
	"sort"
	"strconv"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
)

func generateExpression(expr kinmokuseiAST.Expression) (goast.Expr, error) {
	switch expr := expr.(type) {
	case *kinmokuseiAST.IdentifierExpr:
		if expr.GoMember != nil {
			return generateExpression(expr.GoMember)
		}
		return goast.NewIdent(goName(expr.Name)), nil
	case *kinmokuseiAST.LiteralExpr:
		if expr.Kind == kinmokuseiAST.NilLiteral || expr.Kind == kinmokuseiAST.NullLiteral {
			return goast.NewIdent("nil"), nil
		}
		if expr.Kind == kinmokuseiAST.BooleanLiteral {
			return goast.NewIdent(expr.Text), nil
		}
		kind := token.INT
		switch expr.Kind {
		case kinmokuseiAST.FloatLiteral:
			kind = token.FLOAT
		case kinmokuseiAST.ImaginaryLiteral:
			kind = token.IMAG
		case kinmokuseiAST.StringLiteral:
			kind = token.STRING
		}
		return &goast.BasicLit{Kind: kind, Value: expr.Text}, nil
	case *kinmokuseiAST.UnaryExpr:
		value, err := generateExpression(expr.Operand)
		if err != nil {
			return nil, err
		}
		return &goast.UnaryExpr{Op: goToken(expr.Operator), X: value}, nil
	case *kinmokuseiAST.BinaryExpr:
		left, err := generateExpression(expr.Left)
		if err != nil {
			return nil, err
		}
		right, err := generateExpression(expr.Right)
		if err != nil {
			return nil, err
		}
		return &goast.BinaryExpr{X: left, Op: goToken(expr.Operator), Y: right}, nil
	case *kinmokuseiAST.GoTypeAssertionExpr:
		value, err := generateExpression(expr.Value)
		if err != nil {
			return nil, err
		}
		if expr.ClassDowncast {
			name := mustDowncastName(expr.SourceClass, expr.Type.Name)
			if expr.Checked {
				name = downcastName(expr.SourceClass, expr.Type.Name)
			}
			return &goast.CallExpr{Fun: indexedGoType(goast.NewIdent(name), expr.Type.GenericArguments), Args: []goast.Expr{value}}, nil
		}
		return &goast.TypeAssertExpr{X: value, Type: goType(expr.Type)}, nil
	case *kinmokuseiAST.TaskStartExpr:
		return generateTaskStart(expr)
	case *kinmokuseiAST.AwaitExpr:
		return generateAwait(expr)
	case *kinmokuseiAST.PropagateExpr:
		return nil, fmt.Errorf("result propagation was not lowered from its statement context")
	case *kinmokuseiAST.CallExpr:
		if expr.SuperConstructor {
			args := []goast.Expr{&goast.UnaryExpr{Op: token.AND, X: &goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(expr.SuperBase)}}}
			for _, argument := range expr.Arguments {
				generated, err := generateExpression(argument)
				if err != nil {
					return nil, err
				}
				args = append(args, generated)
			}
			call := &goast.CallExpr{Fun: goast.NewIdent(initializerName(expr.SuperBase)), Args: args}
			if expr.Expanded {
				call.Ellipsis = token.Pos(1)
			}
			return call, nil
		}
		if expr.Builtin == kinmokuseiAST.ResultOKCall || expr.Builtin == kinmokuseiAST.ResultFailCall {
			return nil, fmt.Errorf("Result constructor was not lowered from a return statement")
		}
		if expr.Builtin == kinmokuseiAST.CopyArrayCall || expr.Builtin == kinmokuseiAST.ViewArrayCall {
			if len(expr.TypeArguments) != 1 || len(expr.Arguments) != 1 {
				return nil, fmt.Errorf("slice-to-array lowering received an invalid call shape")
			}
			target := goType(expr.TypeArguments[0])
			if expr.Builtin == kinmokuseiAST.ViewArrayCall {
				target = &goast.ParenExpr{X: &goast.StarExpr{X: target}}
			}
			argument, err := generateExpression(expr.Arguments[0])
			if err != nil {
				return nil, err
			}
			return &goast.CallExpr{Fun: target, Args: []goast.Expr{argument}}, nil
		}
		if expr.Builtin == kinmokuseiAST.MakeSliceCall || expr.Builtin == kinmokuseiAST.MakeMapCall {
			if (expr.Builtin == kinmokuseiAST.MakeSliceCall && len(expr.TypeArguments) != 1) || (expr.Builtin == kinmokuseiAST.MakeMapCall && len(expr.TypeArguments) != 2) {
				return nil, fmt.Errorf("collection make lowering received invalid type arguments")
			}
			var collectionType goast.Expr
			if expr.Builtin == kinmokuseiAST.MakeSliceCall {
				collectionType = &goast.ArrayType{Elt: goType(expr.TypeArguments[0])}
			} else {
				collectionType = &goast.MapType{Key: goType(expr.TypeArguments[0]), Value: goType(expr.TypeArguments[1])}
			}
			arguments := make([]goast.Expr, len(expr.Arguments))
			for index, argument := range expr.Arguments {
				generated, err := generateExpression(argument)
				if err != nil {
					return nil, err
				}
				arguments[index] = generated
			}
			if expr.Builtin == kinmokuseiAST.MakeSliceCall && len(arguments) == 2 {
				// Evaluate size expressions in source order before invoking Go's make
				// intrinsic so behavior does not vary between Go toolchains.
				for index := range arguments {
					if index < len(expr.IntegerSizeArguments) && expr.IntegerSizeArguments[index] {
						arguments[index] = &goast.CallExpr{Fun: goast.NewIdent("int"), Args: []goast.Expr{arguments[index]}}
					}
				}
				return orderedSliceMake(collectionType, arguments[0], arguments[1]), nil
			}
			args := append([]goast.Expr{collectionType}, arguments...)
			return &goast.CallExpr{Fun: goast.NewIdent("make"), Args: args}, nil
		}
		if expr.Builtin == kinmokuseiAST.MakeGoChannelCall {
			if len(expr.TypeArguments) != 1 {
				return nil, fmt.Errorf("goChannel lowering requires exactly one type argument")
			}
			args := []goast.Expr{&goast.ChanType{Dir: goast.SEND | goast.RECV, Value: goType(expr.TypeArguments[0])}}
			for _, argument := range expr.Arguments {
				generated, err := generateExpression(argument)
				if err != nil {
					return nil, err
				}
				args = append(args, generated)
			}
			return &goast.CallExpr{Fun: goast.NewIdent("make"), Args: args}, nil
		}
		if expr.Builtin == kinmokuseiAST.CloseGoChannelCall {
			args := make([]goast.Expr, len(expr.Arguments))
			for i, argument := range expr.Arguments {
				generated, err := generateExpression(argument)
				if err != nil {
					return nil, err
				}
				args[i] = generated
			}
			return &goast.CallExpr{Fun: goast.NewIdent("close"), Args: args}, nil
		}
		if expr.Builtin >= kinmokuseiAST.LenCall && expr.Builtin <= kinmokuseiAST.ImagCall {
			name := map[kinmokuseiAST.BuiltinCallKind]string{
				kinmokuseiAST.LenCall: "len", kinmokuseiAST.CapCall: "cap", kinmokuseiAST.AppendCall: "append", kinmokuseiAST.CopyCall: "copy", kinmokuseiAST.DeleteCall: "delete",
				kinmokuseiAST.ClearCall: "clear", kinmokuseiAST.MinCall: "min", kinmokuseiAST.MaxCall: "max",
				kinmokuseiAST.ComplexCall: "complex", kinmokuseiAST.RealCall: "real", kinmokuseiAST.ImagCall: "imag",
			}[expr.Builtin]
			args := make([]goast.Expr, len(expr.Arguments))
			for i, argument := range expr.Arguments {
				generated, err := generateExpression(argument)
				if err != nil {
					return nil, err
				}
				args[i] = generated
			}
			call := &goast.CallExpr{Fun: goast.NewIdent(name), Args: args}
			if expr.Expanded {
				call.Ellipsis = token.Pos(1)
			}
			return call, nil
		}
		var callee goast.Expr
		var genericReceiver goast.Expr
		if expr.ConversionType != nil {
			callee = goType(*expr.ConversionType)
		} else if member, ok := expr.Callee.(*kinmokuseiAST.MemberExpr); ok && member.GenericMethod {
			callee = goast.NewIdent(goName(member.ResolvedName))
			if member.GenericReceiverSuperBase != "" {
				genericReceiver = &goast.UnaryExpr{Op: token.AND, X: &goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(member.GenericReceiverSuperBase)}}
			} else {
				var err error
				genericReceiver, err = generateExpression(member.Object)
				if err != nil {
					return nil, err
				}
				if member.GenericReceiverAddress {
					genericReceiver = &goast.UnaryExpr{Op: token.AND, X: genericReceiver}
				}
			}
			if member.GenericReceiverUpcast != "" {
				genericReceiver = &goast.CallExpr{
					Fun:  indexedGoType(goast.NewIdent(member.GenericReceiverUpcast), member.GenericReceiverTypeArguments),
					Args: []goast.Expr{genericReceiver},
				}
			}
		} else {
			var err error
			callee, err = generateExpression(expr.Callee)
			if err != nil {
				return nil, err
			}
			if name, ok := callee.(*goast.Ident); ok {
				name.Name = goTypeName(name.Name)
			}
		}
		if expr.ConversionType == nil {
			typeArguments := expr.TypeArguments
			if len(expr.ResolvedTypeArguments) != 0 {
				typeArguments = expr.ResolvedTypeArguments
			}
			callee = indexedGoType(callee, typeArguments)
		}
		args := make([]goast.Expr, 0, len(expr.Arguments)+1)
		if genericReceiver != nil {
			args = append(args, genericReceiver)
		}
		for _, arg := range expr.Arguments {
			generated, err := generateExpression(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, generated)
		}
		call := &goast.CallExpr{Fun: callee, Args: args}
		if expr.Expanded {
			call.Ellipsis = token.Pos(1)
		}
		return call, nil
	case *kinmokuseiAST.ArrowExpr:
		parameters := make([]*goast.Field, 0, len(expr.Parameters))
		for _, parameter := range expr.Parameters {
			parameters = append(parameters, goParameterField(parameter))
		}
		fnType := &goast.FuncType{Params: &goast.FieldList{List: parameters}}
		fnType.Results = functionResults(expr.ResolvedReturnType)
		var body *goast.BlockStmt
		if expr.BlockBody != nil {
			generatedBody, err := generateBlock(expr.BlockBody)
			if err != nil {
				return nil, err
			}
			body = generatedBody
		} else {
			value, err := generateExpression(expr.ExpressionBody)
			if err != nil {
				return nil, err
			}
			if expr.ResolvedReturnType.Name == "void" {
				body = &goast.BlockStmt{List: []goast.Stmt{&goast.ExprStmt{X: value}}}
			} else {
				body = &goast.BlockStmt{List: []goast.Stmt{&goast.ReturnStmt{Results: []goast.Expr{value}}}}
			}
		}
		return &goast.FuncLit{Type: fnType, Body: body}, nil
	case *kinmokuseiAST.ArrayLiteralExpr:
		elements := make([]goast.Expr, 0, len(expr.Elements))
		for _, element := range expr.Elements {
			generated, err := generateExpression(element)
			if err != nil {
				return nil, err
			}
			elements = append(elements, generated)
		}
		arrayType := &goast.ArrayType{Elt: goType(expr.ResolvedElementType)}
		if expr.Fixed {
			arrayType.Len = &goast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(expr.ResolvedLength, 10)}
		}
		return &goast.CompositeLit{Type: arrayType, Elts: elements}, nil
	case *kinmokuseiAST.ObjectLiteralExpr:
		fields := make([]*goast.Field, 0, len(expr.Fields))
		values := make([]goast.Expr, 0, len(expr.Fields))
		indices := make([]int, len(expr.Fields))
		for i := range indices {
			indices[i] = i
		}
		sort.Slice(indices, func(i, j int) bool { return expr.Fields[indices[i]].Name < expr.Fields[indices[j]].Name })
		for _, i := range indices {
			field := expr.Fields[i]
			resolvedName := expr.ResolvedFieldNames[i]
			if resolvedName == "" {
				resolvedName = field.Name
			}
			fields = append(fields, &goast.Field{
				Names: []*goast.Ident{goast.NewIdent(goName(resolvedName))}, Type: goType(expr.ResolvedFieldTypes[i]),
				Tag: &goast.BasicLit{Kind: token.STRING, Value: "`json:\"" + field.Name + "\"`"},
			})
			value, err := generateExpression(field.Value)
			if err != nil {
				return nil, err
			}
			values = append(values, &goast.KeyValueExpr{Key: goast.NewIdent(goName(resolvedName)), Value: value})
		}
		return &goast.CompositeLit{Type: &goast.StructType{Fields: &goast.FieldList{List: fields}}, Elts: values}, nil
	case *kinmokuseiAST.GoCompositeLiteralExpr:
		fields := make([]goast.Expr, 0, len(expr.Fields))
		for index, field := range expr.Fields {
			value, err := generateExpression(field.Value)
			if err != nil {
				return nil, err
			}
			name := field.Name
			if index < len(expr.ResolvedFieldNames) && expr.ResolvedFieldNames[index] != "" {
				name = expr.ResolvedFieldNames[index]
			}
			fields = append(fields, &goast.KeyValueExpr{Key: goast.NewIdent(goName(name)), Value: value})
		}
		return &goast.CompositeLit{Type: goType(expr.Type), Elts: fields}, nil
	case *kinmokuseiAST.MemberExpr:
		if expr.Static {
			return goast.NewIdent(goName(expr.ResolvedName)), nil
		}
		object, err := generateExpression(expr.Object)
		if err != nil {
			return nil, err
		}
		name := expr.ResolvedName
		if name == "" {
			name = expr.Name
		}
		if expr.Super {
			if expr.VirtualOwner != "" {
				name = virtualSlotName(expr.VirtualOwner, name)
			}
			return &goast.SelectorExpr{X: &goast.SelectorExpr{X: goast.NewIdent("this"), Sel: goast.NewIdent(expr.SuperBase)}, Sel: goast.NewIdent(goName(name))}, nil
		}
		if expr.VirtualDispatch {
			object = &goast.SelectorExpr{X: object, Sel: goast.NewIdent(virtualSelfName(expr.VirtualOwner))}
			name = virtualSlotName(expr.VirtualOwner, name)
		}
		return &goast.SelectorExpr{X: object, Sel: goast.NewIdent(goName(name))}, nil
	case *kinmokuseiAST.IndexExpr:
		object, err := generateExpression(expr.Object)
		if err != nil {
			return nil, err
		}
		index, err := generateExpression(expr.Index)
		if err != nil {
			return nil, err
		}
		return &goast.IndexExpr{X: object, Index: index}, nil
	case *kinmokuseiAST.SliceExpr:
		object, err := generateExpression(expr.Object)
		if err != nil {
			return nil, err
		}
		generateBound := func(bound kinmokuseiAST.Expression) (goast.Expr, error) {
			if bound == nil {
				return nil, nil
			}
			return generateExpression(bound)
		}
		low, err := generateBound(expr.Low)
		if err != nil {
			return nil, err
		}
		high, err := generateBound(expr.High)
		if err != nil {
			return nil, err
		}
		max, err := generateBound(expr.Max)
		if err != nil {
			return nil, err
		}
		return &goast.SliceExpr{X: object, Low: low, High: high, Max: max, Slice3: expr.Full}, nil
	case *kinmokuseiAST.NewExpr:
		args := make([]goast.Expr, 0, len(expr.Arguments))
		for _, argument := range expr.Arguments {
			generated, err := generateExpression(argument)
			if err != nil {
				return nil, err
			}
			args = append(args, generated)
		}
		call := &goast.CallExpr{Fun: indexedGoType(goast.NewIdent("New"+expr.ClassName), expr.TypeArguments), Args: args}
		if expr.Expanded {
			call.Ellipsis = token.Pos(1)
		}
		return call, nil
	case *kinmokuseiAST.ClassUpcastExpr:
		value, err := generateExpression(expr.Value)
		if err != nil {
			return nil, err
		}
		return &goast.CallExpr{Fun: indexedGoType(goast.NewIdent(upcastName(expr.SourceClass, expr.TargetClass)), expr.SourceType.GenericArguments), Args: []goast.Expr{value}}, nil
	default:
		return nil, fmt.Errorf("unsupported expression %T", expr)
	}
}
