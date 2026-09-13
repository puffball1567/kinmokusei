package sema

import "testing"

func TestResultValueStorageBoundary(t *testing.T) {
	t.Parallel()
	integer := builtins["int"]
	result := Type{Kind: Result, Element: &integer}
	callback := Type{Kind: Function, Result: &result}
	factoryResult := Type{Kind: Result, Element: &callback}
	for _, test := range []struct {
		name   string
		value  Type
		stored bool
	}{
		{"raw result", result, true},
		{"callback", callback, false},
		{"callback parameter", Type{Kind: Function, Parameters: []Type{callback}, Result: &integer}, false},
		{"result parameter", Type{Kind: Function, Parameters: []Type{result}, Result: &integer}, true},
		{"callback factory", Type{Kind: Function, Result: &factoryResult}, false},
		{"callback slice", Type{Kind: Array, Element: &callback}, false},
		{"result slice", Type{Kind: Array, Element: &result}, true},
		{"callback map", Type{Kind: Map, Key: &integer, Element: &callback}, false},
		{"result map", Type{Kind: Map, Key: &integer, Element: &result}, true},
		{"object callback", Type{Kind: Object, Fields: map[string]Type{"load": callback}}, false},
		{"object result", Type{Kind: Object, Fields: map[string]Type{"load": result}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := containsStoredResultType(test.value); got != test.stored {
				t.Fatalf("got %v want %v", got, test.stored)
			}
		})
	}
}
