package sema

import "github.com/puffball1567/kinmokusei/internal/ast"

// Seed static field metadata before ordinary class declaration, so a constant
// may use another field's array type without checking its runtime initializer.
func (c *Checker) predeclareStaticFields(program *ast.Program) {
	declarations := map[string]*ast.ClassDecl{}
	for _, declaration := range program.Declarations {
		decl, ok := declaration.(*ast.ClassDecl)
		if !ok {
			continue
		}
		declarations[decl.Name] = decl
		class := c.classes[decl.Name]
		if class.fields == nil {
			class.fields = map[string]fieldSymbol{}
		}
		for index := range decl.Fields {
			field := &decl.Fields[index]
			if !field.Static {
				continue
			}
			if _, exists := class.fields[field.Name]; !exists {
				class.fields[field.Name] = fieldSymbol{
					typeInfo: Type{Kind: Invalid}, visibility: field.Visibility, static: true,
					goName:         staticMethodGoName(decl.Name, memberGoName(field.Name, field.Visibility), field.Visibility),
					declaringClass: decl.Name, declarationSpan: field.NameSpan, declaration: field,
				}
			}
		}
	}
	// Propagate metadata through forward-declared inheritance. Ordinary class
	// declaration still validates the hierarchy, field conflicts and visibility.
	visited := map[string]bool{}
	var inherit func(*ast.ClassDecl)
	inherit = func(decl *ast.ClassDecl) {
		if visited[decl.Name] || decl.Base == nil {
			return
		}
		visited[decl.Name] = true
		if base := declarations[decl.Base.Name]; base != nil {
			inherit(base)
			c.classes[decl.Name].ancestors = append([]string{base.Name}, c.classes[base.Name].ancestors...)
			for name, field := range c.classes[base.Name].fields {
				if _, exists := c.classes[decl.Name].fields[name]; !exists {
					c.classes[decl.Name].fields[name] = field
				}
			}
		}
	}
	for _, declaration := range program.Declarations {
		if decl, ok := declaration.(*ast.ClassDecl); ok {
			inherit(decl)
		}
	}
}

func (c *Checker) resolveStaticFieldType(field fieldSymbol) fieldSymbol {
	if field.typeInfo.Kind != Invalid || field.declaration == nil || field.declaration.Constant {
		return field
	}
	dependency := c.initializerChecker()
	dependency.currentClass = field.declaringClass
	typeInfo := dependency.resolveType(field.declaration.Type)
	c.finishInitializerCheck(dependency)
	field.typeInfo = typeInfo
	c.classes[field.declaringClass].fields[field.declaration.Name] = field
	return field
}

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
	if class := c.classes[owner]; class != nil {
		if symbol, exists := class.fields[field.Name]; exists && symbol.declaration == field {
			symbol.typeInfo = expected
			class.fields[field.Name] = symbol
		}
	}
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
