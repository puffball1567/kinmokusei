package sema

import (
	"fmt"
	gotypes "go/types"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func nativeContracts(values []Type) []ast.NativeContract {
	result := make([]ast.NativeContract, len(values))
	for i, value := range values {
		result[i] = nativeContract(value)
	}
	return result
}

func receiverContractParameters(values []Type) []ast.TypeParameter {
	parameters := make([]ast.TypeParameter, len(values))
	for i, value := range values {
		parameters[i].Name = value.Name
	}
	return parameters
}

func receiverTypeParameters(source, owner []ast.TypeParameter, arguments []Type) []ast.TypeParameter {
	parameters := append([]ast.TypeParameter(nil), source...)
	bindings := map[string]ast.TypeRef{}
	for i, parameter := range owner {
		if i < len(source) {
			bindings[parameter.Name] = ast.TypeRef{Name: source[i].Name, TypeParameter: true}
		}
	}
	for i := range parameters {
		if i < len(owner) && owner[i].Constraint != nil {
			constraint := substituteNativeTypeRefParameters(*owner[i].Constraint, bindings)
			parameters[i].Constraint = &constraint
		}
		if i < len(arguments) {
			if parameter, ok := arguments[i].GoType.(*gotypes.TypeParam); ok {
				if bound, ok := parameter.Constraint().Underlying().(*gotypes.Interface); ok {
					parameters[i].InferredComparable = bound.IsComparable()
				}
			}
		}
	}
	return parameters
}

func recordReceiverContracts(expr *ast.MemberExpr, owner Type, receiver Type, callable Type, method methodSymbol) {
	if len(owner.TypeArguments) == 0 {
		return
	}
	expr.ReceiverContractArguments = nativeContracts(owner.TypeArguments)
	expr.ReceiverContractTypes = nil
	for _, argument := range owner.TypeArguments {
		expr.ReceiverContractTypes = append(expr.ReceiverContractTypes, typeRefFromType(argument, expr.Span))
	}
	name := owner.Name
	if object := goTypeNameObject(owner.GoType); object != nil {
		name = object.Name()
	}
	expr.ReceiverContractHelper = "__kinmokuseiReceiver" + name + generatedIdentifier(method.goName)
	functionRef, receiverRef := typeRefFromType(callable, expr.Span), typeRefFromType(receiver, expr.Span)
	expr.ReceiverContractType, expr.ReceiverContractReceiver = &functionRef, &receiverRef
	if !method.pointerReceiver && receiver.Kind == GoPointer && receiver.Element != nil {
		expr.ReceiverContractDereference = true
		valueRef := typeRefFromType(*receiver.Element, expr.Span)
		expr.ReceiverContractReceiver = &valueRef
	}
	if method.pointerReceiver && receiver.Kind != GoPointer {
		expr.GenericReceiverAddress = true
		pointer := Type{Kind: GoPointer, Element: &receiver}
		pointerRef := typeRefFromType(pointer, expr.Span)
		expr.ReceiverContractReceiver = &pointerRef
	}
}

// Nominal storage is checked by Go's projection assertion. The witness retains
// the additional source qualifiers, also inside instantiated nominal types.
// Nominal fields are not expanded, so recursive declarations terminate.
func nativeContract(value Type) ast.NativeContract {
	if value.Kind == TypeParameter {
		return ast.NativeContract{Parameter: value.Name}
	}
	result := ast.NativeContract{Tag: fmt.Sprintf("kind:%d", value.Kind)}
	if value.Kind == Class || value.Kind == Struct || value.Kind == Interface {
		result.Tag += ":" + value.Name
		result.Parts = nativeContracts(value.TypeArguments)
		return result
	}
	if value.GoType != nil {
		if named, ok := gotypes.Unalias(value.GoType).(*gotypes.Named); ok {
			object := named.Origin().Obj()
			path := ""
			if object.Pkg() != nil {
				path = object.Pkg().Path()
			}
			result.Tag += ":" + path + ":" + object.Name()
			result.Parts = nativeContracts(value.TypeArguments)
			return result
		}
	}
	if value.Kind == FixedArray {
		result.Tag += fmt.Sprintf(":%d", value.Length)
	}
	if value.Kind == GoChannel {
		result.Tag += ":" + value.Name
	}
	if value.Kind == Function && value.Variadic {
		result.Tag += ":variadic"
	}
	for index, child := range []*Type{value.Element, value.Key, value.Result} {
		if child != nil {
			result.Parts = append(result.Parts, ast.NativeContract{Tag: fmt.Sprintf("slot:%d", index), Parts: []ast.NativeContract{nativeContract(*child)}})
		}
	}
	for index, group := range [][]Type{value.Parameters, value.Results, value.TypeArguments} {
		if len(group) > 0 {
			result.Parts = append(result.Parts, ast.NativeContract{Tag: fmt.Sprintf("group:%d", index), Parts: nativeContracts(group)})
		}
	}
	if value.Kind == Object || value.Kind == GoStruct {
		names := make([]string, 0, len(value.Fields))
		for name := range value.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			result.Parts = append(result.Parts, ast.NativeContract{Tag: "field:" + name, Parts: []ast.NativeContract{nativeContract(value.Fields[name])}})
		}
		for _, field := range value.GoFields {
			result.Parts = append(result.Parts, ast.NativeContract{Tag: "field:" + field.Name, Parts: []ast.NativeContract{nativeContract(field.Type)}})
		}
	}
	return result
}
