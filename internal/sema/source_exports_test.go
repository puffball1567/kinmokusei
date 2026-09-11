package sema

import (
	"strings"
	"testing"
)

func TestSourceExportValidation(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`export {}; function hidden(): int { return 1; }`,
		`export { value }; function value(): int { return 1; }`,
		`export function value(): int { return hidden(); } function hidden(): int { return 2; }`,
		`export class Box {} export interface Item {} export const value = 1;`,
		`export c("c_native") function native(): int32 { return 1; } export { native };`,
	} {
		if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
			t.Fatalf("%s: %v", input, diagnostics)
		}
	}
	for _, test := range []struct{ input, want string }{
		{`export { missing };`, `exported name "missing" is not a local top-level declaration`},
		{`export const value = 1; export { value };`, `duplicate exported name "value"`},
		{`const value = 1; export { value, value };`, `duplicate exported name "value"`},
		{`import go { Sprint } from "fmt"; export { Sprint };`, `not a local top-level declaration`},
		{`function f():void { const local = 1; } export { local };`, `not a local top-level declaration`},
	} {
		diagnostics := checkSource(t, test.input)
		found := false
		for _, d := range diagnostics {
			found = found || strings.Contains(d, test.want)
		}
		if !found {
			t.Fatalf("%s: %v, want %s", test.input, diagnostics, test.want)
		}
	}
}
