package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Source classes, structs and interfaces have checked method metadata, but
// their provisional Go storage types do not necessarily contain those methods.
// Use the source signatures here: Go storage would also erase text/nullability
// contracts on defined-type methods that do have synthetic Go signatures.
func (c *Checker) nativeConstraintMethods(argument Type) (map[string]methodSymbol, nativeTypeBindings, bool, bool) {
	pointer := argument.Kind == GoPointer && argument.Element != nil
	if pointer {
		argument = *argument.Element
	}
	switch argument.Kind {
	case Class:
		if symbol := c.classes[argument.Name]; symbol != nil {
			if pointer { // A source class already lowers to *Class; **Class has no methods.
				return nil, nil, false, true
			}
			return symbol.methods, nativeClassBindings(symbol, argument), true, true
		}
	case Struct:
		if symbol := c.structs[argument.Name]; symbol != nil {
			return symbol.methods, nativeStructBindings(symbol, argument), pointer, true
		}
	case Interface:
		if symbol := c.interfaces[argument.Name]; symbol != nil {
			if pointer { // A pointer to an interface does not implement its methods.
				return nil, nil, false, true
			}
			return symbol.methods, nativeInterfaceBindings(symbol, argument), true, true
		}
	case GoNamed:
		if named, ok := gotypes.Unalias(argument.GoType).(*gotypes.Named); ok {
			if symbol := c.nativeTypes[named.Obj().Name()]; symbol != nil && symbol.goNamed == named.Origin() && !symbol.declaration.Alias {
				return symbol.methods, nativeDefinedTypeBindings(symbol, argument), pointer, true
			}
		}
	}
	return nil, nil, false, false
}

func nativeMethodsSatisfyConstraint(methods map[string]methodSymbol, bindings nativeTypeBindings, pointer bool, constraint *gotypes.Interface) bool {
	for i := 0; i < constraint.NumMethods(); i++ {
		required := constraint.Method(i)
		if !required.Exported() {
			return false // A generated package cannot implement another package's private method.
		}
		requiredType, err := kinmokuseiTypeFromGo(required.Type())
		if err != nil {
			return false
		}
		provided, ok := nativeConstraintMethod(methods, bindings, pointer, required.Name())
		if !ok || !identicalMethodSignature(provided, requiredType) {
			return false
		}
	}
	return true
}

func nativeConstraintMethod(methods map[string]methodSymbol, bindings nativeTypeBindings, pointer bool, name string) (Type, bool) {
	for _, method := range methods {
		if method.goName == name && method.visibility == ast.Public && !method.static && (!method.pointerReceiver || pointer) && !method.typeInfo.Generic && len(method.typeInfo.TypeParameters) == 0 {
			return substituteNativeTypeParameters(method.typeInfo, bindings), true
		}
	}
	return Type{}, false
}

// Dependent parameters can be determined by a native implementation's method
// signatures (for example E in Getter<E>). Probe source metadata before the Go
// inference fallback, whose provisional storage can have an empty method set.
// Commit only a complete, invariant match; conflicting methods must not leave
// partially inferred arguments behind.
func (c *Checker) inferNativeConstraintMethods(constraint *gotypes.Interface, argument Type, bindings nativeTypeBindings) {
	methods, ownerBindings, pointer, native := c.nativeConstraintMethods(argument)
	if !native || argument.Kind == Nullable || constraint.NumMethods() == 0 {
		return
	}
	trial := make(nativeTypeBindings, len(bindings))
	for parameter, value := range bindings {
		trial[parameter] = value
	}
	for i := 0; i < constraint.NumMethods(); i++ {
		required := constraint.Method(i)
		if !required.Exported() {
			return
		}
		formal, err := kinmokuseiTypeFromGo(required.Type())
		if err != nil {
			return
		}
		actual, ok := nativeConstraintMethod(methods, ownerBindings, pointer, required.Name())
		if !ok {
			return
		}
		if actual.Result != nil && actual.Result.Kind == Result {
			result := methodResultStorageShape(*actual.Result)
			actual.Result = &result
		}
		if err := c.inferNativeTypeArguments(formal, actual, trial); err != nil || !identicalMethodSignature(substituteNativeTypeParameters(formal, trial), actual) {
			return
		}
	}
	for parameter, value := range trial {
		bindings[parameter] = value
	}
}

// Retain the type set after checking source methods separately. Simply removing
// explicit methods is insufficient: embedded named interfaces can contain
// methods too, and comparable may be represented without any embedded types.
func constraintWithoutMethods(constraint *gotypes.Interface) *gotypes.Interface {
	var embedded []gotypes.Type
	seen := map[*gotypes.Interface]bool{}
	var collect func(*gotypes.Interface)
	collect = func(current *gotypes.Interface) {
		if seen[current] {
			return // Shared/diamond embeddings are idempotent intersections.
		}
		seen[current] = true
		for i := 0; i < current.NumEmbeddeds(); i++ {
			value := current.EmbeddedType(i)
			if nested, ok := value.Underlying().(*gotypes.Interface); ok {
				collect(nested)
			} else {
				embedded = append(embedded, value)
			}
		}
	}
	collect(constraint)
	if constraint.IsComparable() {
		embedded = append(embedded, gotypes.Universe.Lookup("comparable").Type())
	}
	return gotypes.NewInterfaceType(nil, embedded).Complete()
}
