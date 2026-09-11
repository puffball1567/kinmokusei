package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
)

func TestParseSourceExports(t *testing.T) {
	t.Parallel()
	for _, declaration := range []string{
		`function value(): int { return 1 }`, `const value = 1`, `let value: int = 1`,
		`class value {}`, `final class value {}`, `struct value { public n: int; }`,
		`interface value {}`, `type value = distinct int;`, `alias value = int;`,
		`enum value { First, Second }`, `constraint value = int | string;`,
	} {
		t.Run(declaration, func(t *testing.T) {
			input := "export " + declaration
			tokens, _ := lexer.Lex("exports.km", input)
			program, diagnostics := Parse(tokens)
			if len(diagnostics) != 0 || len(program.Exports) != 1 || len(program.Declarations) != 1 {
				t.Fatalf("program=%+v diagnostics=%v", program, diagnostics)
			}
			exported := program.Exports[0]
			if !exported.Inline || len(exported.Names) != 1 || exported.Names[0].Name != "value" || !ast.SourceExported(program, program.Declarations[0]) {
				t.Fatal(exported)
			}
			span := exported.Names[0].NameSpan
			if input[span.Start.Offset:span.End.Offset] != "value" || exported.Span.Start.Offset != 0 {
				t.Fatal(exported)
			}
		})
	}
	for _, input := range []string{
		`export {};`, `export { value }; const value = 1;`,
		"const value = 1\nexport {\n value, // comment\n}\n",
		`export c("native") function native(): int32 { return 1; } export { native };`,
	} {
		program, diagnostics := parseSource(t, input)
		if diagnostics != 0 || len(program.Exports) != 1 || program.Exports[0].Inline {
			t.Fatalf("%s: %+v %v", input, program, diagnostics)
		}
	}
}

func TestParseSourceExportErrors(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`export`, `export 42;`, `export export {};`, `export { 1 };`, `export { a b };`,
		`export { a`, `export { a } const value = 1;`, `export type`, `export constraint`,
		`export function`, `export class`, `export interface`, `export enum`, `export let`,
		`struct S {} export function value(this: S): void {}`,
		`function value(): void { export const local = 1; }`,
	} {
		t.Run(input, func(t *testing.T) {
			_, diagnostics := parseSource(t, input)
			if diagnostics == 0 {
				t.Fatal("invalid export accepted")
			}
		})
	}
}
