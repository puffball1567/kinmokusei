package compiler

import (
	"go/importer"
	gotypes "go/types"
)

// GoTypeMethodExpressionSignature exposes the explicit-receiver signature used
// by T.M / (*T).M. Unlike bound selectors, it never implicitly takes an address.
func (r Result) GoTypeMethodExpressionSignature(path, typeName string, pointer bool, methodName string) (GoFunctionSignature, bool, error) {
	goImporter := r.goImporter
	if goImporter == nil {
		goImporter = importer.Default()
	}
	packageInfo, err := goImporter.Import(path)
	if err != nil {
		return GoFunctionSignature{}, false, err
	}
	object, ok := packageInfo.Scope().Lookup(typeName).(*gotypes.TypeName)
	if !ok || !object.Exported() {
		return GoFunctionSignature{}, false, nil
	}
	receiver := object.Type()
	if named, ok := gotypes.Unalias(receiver).(*gotypes.Named); ok && named.TypeParams().Len() != 0 && named.TypeArgs().Len() == 0 {
		return GoFunctionSignature{}, false, nil
	}
	if contract, ok := receiver.Underlying().(*gotypes.Interface); ok && !contract.IsMethodSet() {
		return GoFunctionSignature{}, false, nil
	}
	if pointer {
		receiver = gotypes.NewPointer(receiver)
	}
	selected := gotypes.NewMethodSet(receiver).Lookup(nil, methodName)
	if selected == nil || !selected.Obj().Exported() {
		return GoFunctionSignature{}, false, nil
	}
	signature := goFunctionSignature(selected.Type().(*gotypes.Signature))
	signature.ParameterNames = append([]string{"receiver"}, signature.ParameterNames...)
	signature.ParameterTypes = append([]string{formatGoTypeForSignature(receiver)}, signature.ParameterTypes...)
	return signature, true, nil
}
