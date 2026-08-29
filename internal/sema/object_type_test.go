package sema

import (
	"strings"
	"testing"
)

func TestObjectTypeSuccessMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"empty", `function value(input: {}): {} { return input; }`},
		{"literal", `function value(): { message: string, count: int } { return { count: 1, message: "ok" }; }`},
		{"field order is structural", `function value(input: { second: string, first: int }): { first: int, second: string } { return input; }`},
		{"nested", `function value(input: { child: { count: int } }): int { return input.child.count; }`},
		{"slice", `function value(input: { count: int }[]): int { return input[0].count; }`},
		{"nil pointer field", `import go http from "net/http"; function value(): { client: *http.Client } { return { client: nil }; }`},
		{"function field", `function value(input: { callback: (value: int) => string }): string { return input.callback(1); }`},
		{"Go any", `import go fmt from "fmt"; function value(): string { return fmt.Sprint({ message: "ok", count: 1 }); }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics=%v", diagnostics)
			}
		})
	}
}

func TestObjectTypeFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"duplicate type field", `function value(input: { item: int, item: int }): int { return input.item; }`, `duplicate object type field "item"`},
		{"void field", `function value(input: { item: void }): int { return 1; }`, `field "item" cannot have type void`},
		{"missing literal field", `function value(): { item: int, name: string } { return { item: 1 }; }`, `missing field "name"`},
		{"extra literal field", `function value(): { item: int } { return { item: 1, extra: 2 }; }`, `has no field "extra"`},
		{"wrong literal field type", `function value(): { item: int } { return { item: "wrong" }; }`, `cannot use string as int`},
		{"nested mismatch", `function value(): { child: { item: int } } { return { child: { item: "wrong" } }; }`, `cannot use string as int`},
		{"different field set", `function value(input: { item: int }): { other: int } { return input; }`, `cannot use { item: int } as { other: int }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, test.want) {
				t.Fatalf("diagnostics=%v want=%q", diagnostics, test.want)
			}
		})
	}
}
