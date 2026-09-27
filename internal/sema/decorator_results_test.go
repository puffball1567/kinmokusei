package sema

import (
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/source"
)

func TestDecoratorInvocationResultContracts(t *testing.T) {
	t.Parallel()
	span := source.Span{}
	narrow := Type{Kind: Int8, Name: "int8"}
	leaf := Type{Kind: Class, Name: "Leaf"}
	nullable := Type{Kind: Nullable, Element: &leaf}
	list := Type{Kind: MultiValue, Results: []Type{narrow, nullable}}
	payload, slots, reason := decoratorInvocationResult(list, span)
	if reason != "" || payload.Kind != Array || len(slots) != 2 {
		t.Fatalf("payload=%v slots=%v reason=%s", payload, slots, reason)
	}
	if slots[0].Contract != decoratorValueContract(narrow) || slots[1].Contract != decoratorValueContract(nullable) || slots[1].Contract == decoratorValueContract(leaf) {
		t.Fatal("slot types were erased")
	}
	for _, result := range []Type{narrow, {Kind: Result, Element: &narrow}} {
		payload, slots, reason := decoratorInvocationResult(result, span)
		if reason != "" || len(slots) != 0 || payload.Kind != Int8 {
			t.Errorf("scalar/effect result changed: %v %v %s", payload, slots, reason)
		}
	}
	for _, invalid := range []Type{
		{Kind: MultiValue},
		{Kind: Result, Element: &list},
		{Kind: MultiValue, Results: []Type{narrow, {Kind: TypeParameter, Name: "T"}}},
		{Kind: MultiValue, Results: []Type{narrow, {Kind: Result, Element: &narrow}}},
	} {
		_, slots, reason := decoratorInvocationResult(invalid, span)
		if len(slots) != 0 || !strings.Contains(reason, "cannot be stored in DecoratorValue") {
			t.Errorf("unsupported result accepted: %v %v %q", invalid, slots, reason)
		}
	}
}
