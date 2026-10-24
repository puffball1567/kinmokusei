package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

type goFunctionIdentity struct{ packagePath, name string }

// An audited borrow permits observing verified-text storage, but promises no
// writes through reflection, retention after return or returned alias. DeepEqual
// invokes no methods. Sprintf is a separate callback-aware contract: it invokes
// Format/GoString/String/Error with no writable aliases to other arguments, and
// its raw string result cannot expose the original storage. Ordinary call-effect
// invalidation remains enabled. Never infer a contract just from an any parameter
// or a scalar result; check the complete signature and package/function identity.
type goCallContract struct {
	borrowedParameters int
	resultKind         gotypes.BasicKind
	borrowedStart      int
	variadic           bool
}

var trustedGoCallContracts = map[goFunctionIdentity]goCallContract{
	{"reflect", "DeepEqual"}: {borrowedParameters: 2, resultKind: gotypes.Bool},
	{"fmt", "Sprintf"}:       {borrowedParameters: 1, borrowedStart: 1, variadic: true, resultKind: gotypes.String},
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
	if !ok || signature.Recv() != nil || signature.TypeParams().Len() != 0 || signature.Variadic() != contract.variadic ||
		signature.Params().Len() != contract.borrowedStart+contract.borrowedParameters || signature.Results().Len() != 1 ||
		!gotypes.Identical(signature.Results().At(0).Type(), gotypes.Typ[contract.resultKind]) {
		return goCallContract{}, false
	}
	anyType := gotypes.NewInterfaceType(nil, nil).Complete()
	for i := 0; i < signature.Params().Len(); i++ {
		expected := gotypes.Type(anyType)
		if i < contract.borrowedStart {
			expected = gotypes.Typ[gotypes.String]
		} else if contract.variadic {
			expected = gotypes.NewSlice(anyType)
		}
		if !gotypes.Identical(signature.Params().At(i).Type(), expected) {
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
	if !known || index < contract.borrowedStart || !contract.variadic && index >= contract.borrowedStart+contract.borrowedParameters || containsTaskType(actual) {
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
