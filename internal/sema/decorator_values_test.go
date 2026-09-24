package sema

import (
	"strings"
	"testing"
)

func TestDecoratorValueRequiresConcretePayload(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`function box<T>(value:T):DecoratorValue{return decoratorValue(value);}`,
		`function box<T>(value:T[]):DecoratorValue{return decoratorValue(value);}`,
		`class Box<T>{} function box<T>(value:Box<T>):DecoratorValue{return decoratorValue(value);}`,
		`function unbox<T>(value:DecoratorValue):Result<T>{return decoratorValueAs<T>(value);}`,
		`function unbox<T>(value:DecoratorValue):Result<T[]>{return decoratorValueAs<T[]>(value);}`,
	} {
		diagnostics := checkSource(t, source)
		found := false
		for _, diagnostic := range diagnostics {
			if strings.Contains(diagnostic, "DecoratorValue requires a concrete payload type") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected concrete-type diagnostic for %s: %v", source, diagnostics)
		}
	}
}

func TestDecoratorValueContractPreservesStructure(t *testing.T) {
	t.Parallel()
	leaf := Type{Kind: Class, Name: "Leaf"}
	nullableLeaf := Type{Kind: Nullable, Element: &leaf}
	callback := Type{Kind: Function, Result: &leaf}
	nullableCallback := Type{Kind: Nullable, Element: &callback}
	callbackReturningNullable := Type{Kind: Function, Result: &nullableLeaf}
	if decoratorValueContract(nullableCallback) == decoratorValueContract(callbackReturningNullable) {
		t.Fatal("nullable callback collides with nullable return")
	}
	one := Type{Kind: Object, Fields: map[string]Type{"a": leaf, "b": nullableLeaf}}
	two := Type{Kind: Object, Fields: map[string]Type{"b": nullableLeaf, "a": leaf}}
	if decoratorValueContract(one) != decoratorValueContract(two) {
		t.Fatal("object field order changes the contract")
	}
}
