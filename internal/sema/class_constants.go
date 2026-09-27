package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// Constants are checked on demand, before users of their values. Their lexical
// context is the declaring module/class, never the caller's locals or generics.
func (c *Checker) ensureClassConstantChecked(owner string, field *ast.FieldDecl) {
	switch c.classConstantChecks[field] {
	case globalBindingChecking:
		c.report(field.NameSpan, "class constant has an initialization cycle")
		return
	case globalBindingChecked:
		return
	}
	c.classConstantChecks[field] = globalBindingChecking
	dependency := c.initializerChecker()
	dependency.currentClass = owner
	dependency.inFieldInitializer = true
	dependency.result = builtins["void"]
	dependency.globalDependencyOwner = staticFieldDependency(owner, field.Name)
	expected := dependency.resolveType(field.Type)
	if field.Initializer != nil {
		actual := dependency.checkExpressionExpectedSlot(&field.Initializer, expected)
		dependency.requireAssignable(expected, actual, field.Initializer.GetSpan())
		if expected.Kind != Invalid && actual.Kind != Invalid {
			if !isScalarConstantType(expected) {
				dependency.report(field.Type.Span, "class constants require a numeric, string, or boolean type")
			} else if info, known := dependency.scalarConstant(field.Initializer); !known || !initializerEmitsConstant(field.Initializer) {
				dependency.report(field.Initializer.GetSpan(), "class constant initializer must be a compile-time constant")
			} else if target, ok := goTypeOf(expected); ok {
				if value, err := c.convertNumericConstant(info, target); err == nil {
					c.classConstantValues[field] = value
				} else {
					dependency.report(field.Initializer.GetSpan(), err.Error())
				}
			}
		}
	}
	c.finishInitializerCheck(dependency)
	c.classConstantChecks[field] = globalBindingChecked
}
