package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

type goFunctionIdentity struct{ packagePath, name string }

type callArgumentBorrow struct {
	borrowedParameters int
	borrowedStart      int
	variadic           bool
}

// An audited borrow permits observing verified-text storage, but promises no
// writes through reflection, retention after return or returned alias. DeepEqual
// invokes no methods. Sprintf is a separate callback-aware contract: it invokes
// Format/GoString/String/Error with no writable aliases to other arguments, and
// its raw string result cannot expose the original storage. Ordinary call-effect
// invalidation remains enabled. Never infer a contract just from an any parameter
// or a scalar result; check the complete signature and package/function identity.
type goCallContract struct {
	callArgumentBorrow
	resultKind gotypes.BasicKind
}

var trustedGoCallContracts = map[goFunctionIdentity]goCallContract{
	{"reflect", "DeepEqual"}: {callArgumentBorrow: callArgumentBorrow{borrowedParameters: 2}, resultKind: gotypes.Bool},
	{"fmt", "Sprintf"}:       {callArgumentBorrow: callArgumentBorrow{borrowedParameters: 1, borrowedStart: 1, variadic: true}, resultKind: gotypes.String},
}

var emptyGoInterface = gotypes.NewInterfaceType(nil, nil).Complete()

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
	for i := 0; i < signature.Params().Len(); i++ {
		expected := gotypes.Type(emptyGoInterface)
		if i < contract.borrowedStart {
			expected = gotypes.Typ[gotypes.String]
		} else if contract.variadic {
			expected = gotypes.NewSlice(emptyGoInterface)
		}
		if !gotypes.Identical(signature.Params().At(i).Type(), expected) {
			return goCallContract{}, false
		}
	}
	return contract, true
}

func (c *Checker) recordGoCallContract(expression ast.Expression, function *gotypes.Func) {
	if contract, ok := trustedGoCallContract(function); ok {
		c.recordArgumentBorrow(expression, contract.callArgumentBorrow)
	}
}

// The source loader marks the audited declaration. Also verify that the
// resolved signature still accepts a variadic empty interface and returns void.
func (c *Checker) recordEmbeddedCallBorrow(expression ast.Expression, callable Type) {
	if callable.Kind != Function || callable.Generic || !callable.Variadic || len(callable.Parameters) != 1 ||
		callable.Result == nil || callable.Result.Kind != Void {
		return
	}
	if element := callable.Parameters[0].GoType; element != nil && gotypes.Identical(element, emptyGoInterface) {
		c.recordArgumentBorrow(expression, callArgumentBorrow{borrowedParameters: 1, variadic: true})
	}
}

func (c *Checker) recordArgumentBorrow(expression ast.Expression, borrow callArgumentBorrow) {
	if c.callArgumentBorrows == nil {
		c.callArgumentBorrows = map[ast.Expression]callArgumentBorrow{}
	}
	c.callArgumentBorrows[expression] = borrow
}

func (c *Checker) borrowedCallArgumentAssignable(call *ast.CallExpr, index int, expected, actual Type) bool {
	borrow, known := c.callArgumentBorrows[call.Callee]
	if !known || index < borrow.borrowedStart || !borrow.variadic && index >= borrow.borrowedStart+borrow.borrowedParameters || containsTaskType(actual) {
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
