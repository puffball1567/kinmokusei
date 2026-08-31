package sema

import (
	"strings"
	"testing"
)

func TestCheckedMapLookupSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"declaration", `function lookup(values: Map<string, int>, key: string): int { const [value, present] = values[key]; if (present) { return value; } return -1; }`},
		{"mutable declaration and reassignment", `function lookup(first: Map<string, int>, second: Map<string, int>, key: string): int { let [value, present] = first[key]; [value, present] = second[key]; if (present) { return value; } return -1; }`},
		{"blank bindings", `function lookup(values: Map<string, int>, key: string): boolean { const [_, present] = values[key]; let value = 0; [value, _] = values[key]; return present || value > 0; }`},
		{"defined map", `type Lookup = distinct Map<string, int>; function lookup(values: Lookup, key: string): boolean { const [value, present] = values[key]; return present && value >= 0; }`},
		{"generic map", `function lookup<K extends comparable, V>(values: Map<K, V>, key: K): boolean { const [_, present] = values[key]; return present; }`},
		{"class value", `class User { constructor(public name: string) {} } function lookup(values: Map<string, User>, key: string): string { const [user, present] = values[key]; if (present) { return user.name; } return "missing"; }`},
		{"imported named Go map", `import go http from "net/http"; function lookup(values: http.Header, key: string): boolean { const [items, present] = values[key]; return present && len(items) > 0; }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestCheckedMapLookupFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"slice", `function bad(values: int[]): void { const [value, present] = values[0]; }`, "requires a map operand"},
		{"array", `function bad(values: [2]int): void { const [value, present] = values[0]; }`, "requires a map operand"},
		{"string", `function bad(value: string): void { const [item, present] = value[0]; }`, "requires a map operand"},
		{"one binding", `function bad(values: Map<string, int>): void { const [value] = values["key"]; }`, "got 1 bindings for 2 results"},
		{"three bindings", `function bad(values: Map<string, int>): void { const [value, present, extra] = values["key"]; }`, "got 3 bindings for 2 results"},
		{"wrong key", `function bad(values: Map<string, int>): void { const [value, present] = values[1]; }`, "cannot use integer literal as string"},
		{"assignment result mismatch", `function bad(values: Map<string, int>): void { let value = ""; let present = false; [value, present] = values["key"]; }`, "cannot use int as string"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
