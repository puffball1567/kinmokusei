package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

type goFunctionIdentity struct{ packagePath, name string }

// An audited borrow permits observing verified-text storage, but promises no
// writes, retention after return, method/callback invocation or returned alias.
// The full signature is checked as well as package/function identity. Do not
// infer this contract from an any parameter or an apparently scalar result.
type goCallContract struct {
	borrowedParameters int
	resultKind         gotypes.BasicKind
}

var trustedGoCallContracts = map[goFunctionIdentity]goCallContract{
	{"reflect", "DeepEqual"}: {borrowedParameters: 2, resultKind: gotypes.Bool},
}

func trustedGoCallContract(function *gotypes.Func) (goCallContract, bool) {
	if function == nil || function.Pkg() == nil || function.Parent() != function.Pkg().Scope() {
		return goCallContract{}, false
	}
	contract, known := trustedGoCallContracts[goFunctionIdentity{function.Pkg().Path(), function.Name()}]
	if !known {
		return goCallContract{}, false
	}
	signature, ok := function.Type().(*gotypes.Signature)
	if !ok || signature.Recv() != nil || signature.TypeParams().Len() != 0 || signature.Variadic() ||
		signature.Params().Len() != contract.borrowedParameters || signature.Results().Len() != 1 ||
		!gotypes.Identical(signature.Results().At(0).Type(), gotypes.Typ[contract.resultKind]) {
		return goCallContract{}, false
	}
	anyType := gotypes.NewInterfaceType(nil, nil).Complete()
	for i := 0; i < signature.Params().Len(); i++ {
		if !gotypes.Identical(signature.Params().At(i).Type(), anyType) {
			return goCallContract{}, false
		}
	}
	return contract, true
}

func (c *Checker) recordGoCallContract(expression ast.Expression, function *gotypes.Func) {
	if contract, ok := trustedGoCallContract(function); ok {
		if c.goCallContracts == nil {
			c.goCallContracts = map[ast.Expression]goCallContract{}
		}
		c.goCallContracts[expression] = contract
	}
}

func (c *Checker) borrowedGoCallArgumentAssignable(call *ast.CallExpr, index int, expected, actual Type) bool {
	contract, known := c.goCallContracts[call.Callee]
	if !known || index < 0 || index >= contract.borrowedParameters || containsTaskType(actual) {
		return false
	}
	switch actual.Kind {
	case Invalid, Void, Result, MultiValue, GoPackage, GoTypeName, Null:
		return false
	}
	// This exception is local to the already checked argument boundary. Nested
	// calls, assertions, null guards and numeric materialization keep their own
	// normal checks; no ambient permission is enabled while checking arguments.
	storage, ok := c.goTypeForNativeStorage(actual)
	return ok && expected.GoType != nil && nativeStorageGraphReady(storage, map[gotypes.Type]bool{}) && gotypes.AssignableTo(storage, expected.GoType)
}
