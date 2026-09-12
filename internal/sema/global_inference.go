package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

type globalBindingCheckState uint8

const (
	globalBindingChecking globalBindingCheckState = iota + 1
	globalBindingChecked
)

// Resolve dependencies on demand through ordinary lexical name lookup. Each
// initializer/body is checked once; this changes neither declaration order nor
// runtime initialization. A visiting binding retains its predeclared signature,
// so annotations break recursive inference dependencies without rechecking ASTs.
func (c *Checker) ensureGlobalBindingChecked(decl *ast.VariableDecl) {
	if c.globalBindingChecks[decl] != 0 {
		return
	}
	c.globalBindingChecks[decl] = globalBindingChecking

	// A global dependency is not a nested closure of its caller. Start with an
	// empty lexical/control context and explicitly share only program metadata.
	// In particular, locals, generic parameters, receiver access, nullable facts,
	// capture tracking and return inference must not cross this boundary.
	dependency := &Checker{
		functions: c.functions, globals: c.globals,
		classes: c.classes, structs: c.structs, interfaces: c.interfaces,
		nativeTypes: c.nativeTypes, enums: c.enums, allowed: c.allowed,
		goPackages: c.goPackages, goNamedImports: c.goNamedImports,
		goImporter: c.goImporter, allowUnsafeGo: c.allowUnsafeGo,
		nativeConstraintsReady:  c.nativeConstraintsReady,
		structGoTypesFinalized:  c.structGoTypesFinalized,
		deferredParameterBounds: c.deferredParameterBounds,
		parameterRangeShapes:    c.parameterRangeShapes,
		functionTypeParameters:  c.functionTypeParameters,
		receiverTypeParameters:  c.receiverTypeParameters,
		methodTypeParameters:    c.methodTypeParameters,
		validFallthrough:        c.validFallthrough,
		numericValues:           c.numericValues,
		globalDependencies:      c.globalDependencies,
		globalBindingChecks:     c.globalBindingChecks,
		memberFlow:              map[memberFlowKey]memberFlowState{},
		memberTypes:             map[memberFlowKey]Type{},
	}
	dependency.checkGlobalBinding(decl)
	c.diagnostics = append(c.diagnostics, dependency.diagnostics...)
	c.usesTasks = c.usesTasks || dependency.usesTasks
	c.usesExceptions = c.usesExceptions || dependency.usesExceptions
	c.globalBindingChecks[decl] = globalBindingChecked
}
