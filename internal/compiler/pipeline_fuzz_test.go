package compiler

import (
	"bytes"
	"fmt"
	"go/importer"
	goparser "go/parser"
	"go/token"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/codegen"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	kinmokuseiParser "github.com/puffball1567/kinmokusei/internal/parser"
	"github.com/puffball1567/kinmokusei/internal/sema"
)

var pipelineFuzzSeeds = []string{
	`function add(left: int, right: int): int { return left + right; }`,
	`struct Point { public x: int; public y: int; pointer function move(dx: int, dy: int): void { this.x += dx; this.y += dy; } }`,
	`class Box<T extends comparable> { constructor(public value: T) {} public function get(): T { return this.value; } } function use(): string { return new Box<string>("value").get(); }`,
	`function parse(okay: boolean): Result<int> { if (okay) { return ok(42); } return err(new Exception("no value")); }`,
	`function collect(values: int[]): int { let total = 0; for (const value of values) { total += value; } return total; }`,
	`import go strings from "strings"; function upper(value: string): string { return strings.ToUpper(value); }`,
	`function broken<T extends>(value: T): T { return value; }`,
	`class Incomplete { private value: string; constructor(flag: boolean) { if (flag) { this.value = "set"; } } }`,
	`class Switched { private value: string; constructor(values: int[]) { switch (len(values)) { case 0 { this.value = "empty"; } default { for (const item of values) { this.value = "set"; } } } } }`,
	`for (((`,
	"\xff\x00",
}

func compilePipelineProperty(input string) ([]byte, bool, error) {
	tokens, lexDiagnostics := lexer.Lex("fuzz.km", input)
	program, parseDiagnostics := kinmokuseiParser.Parse(tokens)
	if program == nil {
		return nil, false, fmt.Errorf("parser returned a nil program")
	}
	if len(lexDiagnostics) != 0 || len(parseDiagnostics) != 0 {
		return nil, false, nil
	}
	goImporter := importer.Default()
	if diagnostics := sema.CheckScopedWithGoImporter(program, nil, goImporter); len(diagnostics) != 0 {
		return nil, false, nil
	}
	first, err := codegen.GenerateWithImporter(program, "fuzzpkg", goImporter)
	if err != nil {
		return nil, true, err
	}
	second, err := codegen.GenerateWithImporter(program, "fuzzpkg", goImporter)
	if err != nil {
		return nil, true, err
	}
	if !bytes.Equal(first, second) {
		return nil, true, fmt.Errorf("code generation is not deterministic")
	}
	if _, err := goparser.ParseFile(token.NewFileSet(), "generated.go", first, goparser.AllErrors); err != nil {
		return nil, true, fmt.Errorf("generated Go does not parse: %w", err)
	}
	return first, true, nil
}

func TestCompilePipelineProperties(t *testing.T) {
	generated := 0
	for index, seed := range pipelineFuzzSeeds {
		output, reachedCodegen, err := compilePipelineProperty(seed)
		if err != nil {
			t.Fatalf("seed %d: %v", index, err)
		}
		if reachedCodegen {
			generated++
			if len(output) == 0 {
				t.Fatalf("seed %d reached codegen with empty output", index)
			}
		}
	}
	if generated < 6 {
		t.Fatalf("only %d seeds reached semantic analysis and code generation; want at least 6", generated)
	}
}

func FuzzCompilePipelineNeverPanics(f *testing.F) {
	for _, seed := range pipelineFuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 64<<10 {
			t.Skip()
		}
		if _, _, err := compilePipelineProperty(input); err != nil {
			t.Fatal(err)
		}
	})
}
