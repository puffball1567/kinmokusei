package sema

import gotypes "go/types"

func (c *Checker) nativeStructMethodSet(value Type) (map[string]methodSymbol, nativeTypeBindings, bool) {
	base := value
	pointer := base.Kind == GoPointer && base.Element != nil
	if pointer {
		base = *base.Element
	}
	if base.Kind != Struct || c.structs[base.Name] == nil {
		return nil, nil, false
	}
	methods, bindings := c.effectiveStructMethods(base, pointer)
	return methods, bindings, pointer
}

// Native struct storage is forward-declared for type checking and can lack
// methods that are present in generated Go. Compare the checked method set
// with both the Go signature and its source contracts.
func (c *Checker) nativeStructSatisfiesGoInterface(target, value Type) bool {
	if target.Kind != GoNamed && target.Kind != GoInterface || isSourceAnonymousInterface(target) {
		return false
	}
	contract := underlyingGoInterface(target.GoType)
	if contract == nil || !contract.IsMethodSet() || contract.NumMethods() == 0 {
		return false
	}
	origin, arguments := goInterfaceContractOrigin(target, contract)
	methods, bindings, pointer := c.nativeStructMethodSet(value)
	if methods == nil {
		return false
	}
	var storageType gotypes.Type
	for i := 0; i < contract.NumMethods(); i++ {
		required := contract.Method(i)
		if !required.Exported() {
			// Only a real Go embedding can supply a private method belonging
			// to another package. Its nominal storage retains that identity.
			if storageType == nil {
				var ok bool
				storageType, ok = c.goTypeForNativeStorage(value)
				if !ok || !nativeStorageGraphReady(storageType, map[gotypes.Type]bool{}) {
					return false
				}
			}
			object, _, _ := gotypes.LookupFieldOrMethod(storageType, false, required.Pkg(), required.Name())
			provided, ok := object.(*gotypes.Func)
			if !ok || !gotypes.Identical(provided.Type(), required.Type()) {
				return false
			}
			continue
		}
		provided, ok := nativeConstraintMethod(methods, bindings, pointer, required.Name())
		if !ok {
			return false
		}
		storage, ok := c.goTypeForNativeStorage(provided)
		if !ok || !gotypes.Identical(storage, required.Type()) {
			return false
		}
		signature, err := kinmokuseiTypeFromGo(origin.Method(i).Type())
		if err != nil || !identicalMethodSignature(substituteNativeTypeParameters(signature, arguments), provided) {
			return false
		}
	}
	return true
}
