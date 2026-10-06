package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

type globalBindingCheckState uint8

const (
	globalBindingChecking globalBindingCheckState = iota + 1
	globalBindingChecked
)

func (c *Checker) resolveAnnotatedBindingType(symbol valueSymbol) valueSymbol {
	if symbol.declaration == nil || !symbol.declaration.Type.IsSpecified() {
		return symbol
	}
	// A non-scalar annotation establishes the type without its initializer.
	// In particular, constant len/cap must not demand runtime initialization.
	// Scalar bindings still need ordinary checking to establish constant values.
	dependency := c.initializerChecker()
	typeInfo := dependency.resolveType(symbol.declaration.Type)
	c.finishInitializerCheck(dependency)
	if typeInfo.Kind != Invalid && !isScalarConstantType(typeInfo) {
		symbol.typeInfo, symbol.declaredType = typeInfo, typeInfo
		c.globals[symbol.declaration.Name] = symbol
	}
	return symbol
}

// Resolve dependencies on demand through ordinary lexical name lookup. Each
// initializer/body is checked once; this changes neither declaration order nor
// runtime initialization. A visiting binding retains its predeclared signature,
// so annotations break recursive inference dependencies without rechecking ASTs.
func (c *Checker) ensureGlobalBindingChecked(decl *ast.VariableDecl) {
	if c.globalBindingChecks[decl] != 0 {
		return
	}
	c.globalBindingChecks[decl] = globalBindingChecking
	dependency := c.initializerChecker()
	dependency.checkGlobalBinding(decl)
	c.finishInitializerCheck(dependency)
	c.globalBindingChecks[decl] = globalBindingChecked
}

func (c *Checker) initializerChecker() *Checker {
	// A global dependency is not a nested closure of its caller. Start with an
	// empty lexical/control context and explicitly share only program metadata.
	// In particular, locals, generic parameters, receiver access, nullable facts,
	// capture tracking and return inference must not cross this boundary.
	return &Checker{
		functions: c.functions, globals: c.globals,
		classes: c.classes, structs: c.structs, interfaces: c.interfaces,
		nativeTypes: c.nativeTypes, enums: c.enums, allowed: c.allowed, unimportedReferences: c.unimportedReferences,
		goPackages: c.goPackages, goNamedImports: c.goNamedImports,
		goImporter: c.goImporter, allowUnsafeGo: c.allowUnsafeGo, goSizes: c.goSizes,
		goFieldReceivers:         c.goFieldReceivers,
		nativeConstraintsReady:   c.nativeConstraintsReady,
		structGoTypesFinalized:   c.structGoTypesFinalized,
		deferredParameterBounds:  c.deferredParameterBounds,
		parameterRangeShapes:     c.parameterRangeShapes,
		parameterDeleteKeys:      c.parameterDeleteKeys,
		parameterCollectionTerms: c.parameterCollectionTerms,
		functionTypeParameters:   c.functionTypeParameters,
		receiverTypeParameters:   c.receiverTypeParameters,
		methodTypeParameters:     c.methodTypeParameters,
		validFallthrough:         c.validFallthrough,
		constantValues:           c.constantValues,
		arrayLengthChecks:        c.arrayLengthChecks,
		globalDependencies:       c.globalDependencies,
		globalBindingChecks:      c.globalBindingChecks,
		classConstantChecks:      c.classConstantChecks,
		classConstantValues:      c.classConstantValues,
		enumChecks:               c.enumChecks,
		resultErrorUses:          c.resultErrorUses,
		memberFlow:               map[memberFlowKey]memberFlowState{},
		memberTypes:              map[memberFlowKey]Type{},
	}
}

func (c *Checker) finishInitializerCheck(dependency *Checker) {
	c.diagnostics = append(c.diagnostics, dependency.diagnostics...)
	c.usesTasks = c.usesTasks || dependency.usesTasks
	c.usesExceptions = c.usesExceptions || dependency.usesExceptions
	c.usesUTF8 = c.usesUTF8 || dependency.usesUTF8
}
