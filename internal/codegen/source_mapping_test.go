package codegen

import (
	"bytes"
	"go/importer"
	"strings"
	"sync"
	"testing"

	kmast "github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
	"github.com/puffball1567/kinmokusei/internal/sema"
)

func TestSourceMappingGeneration(t *testing.T) {
	t.Parallel()
	fixtures := []string{
		"function choose(flag: boolean): int { if (flag) { return 1; } else { return 2; } }",
		"const double = (value: int): int => value * 2; function run(): int { const next = (value: int): int => value + 1; return next(double(2)); }",
		"function run(): int { let sum = 0; for (let i = 0; i < 3; i++) { sum += i; } return sum; }",
		"class Counter { value: int = 1; constructor() { this.value = 2; } function add(): int { this.value++; return this.value; } }",
		"function run(): int { try { return 1; } finally { let ignored = 2; } }",
		"function run(): int { const even = (n: int): boolean => n == 0 || odd(n - 1); const odd = (n: int): boolean => n != 0 && even(n - 1); if (even(4)) { return 1; } return 0; }",
		"function text(): string {\r\n  let value: string = \"金木犀\";\r\n  return value;\r\n}\r\n",
	}
	for _, input := range fixtures {
		t.Run(input[:min(45, len(input))], func(t *testing.T) {
			program := checkedMappingProgram(t, input)
			plain, err := Generate(program, "sample")
			if err != nil {
				t.Fatal(err)
			}
			generated, mappings, err := GenerateMappedWithTarget(program, "sample", importer.Default(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(plain, generated) {
				t.Fatalf("mapping changed output:\nplain:\n%s\nmapped:\n%s", plain, generated)
			}
			if len(mappings) == 0 {
				t.Fatal("no mappings")
			}
			for i, mapping := range mappings {
				if mapping.Source.Path != "mapping.km" || mapping.Source.Start.Offset >= mapping.Source.End.Offset || mapping.Source.End.Offset > len(input) {
					t.Fatalf("invalid origin: %+v", mapping)
				}
				if mapping.Generated.Start.Offset >= mapping.Generated.End.Offset || mapping.Generated.End.Offset > len(generated) {
					t.Fatalf("invalid generated range: %+v", mapping)
				}
				if i > 0 && mappings[i-1].Generated.End.Offset > mapping.Generated.Start.Offset {
					t.Fatal("overlapping generated ranges")
				}
			}
			again, err := Generate(program, "sample")
			if err != nil || !bytes.Equal(plain, again) {
				t.Fatal("checked input was mutated")
			}
		})
	}
}

func checkedMappingProgram(t *testing.T, input string) *kmast.Program {
	t.Helper()
	tokens, lex := lexer.Lex("mapping.km", input)
	program, parse := parser.Parse(tokens)
	if len(lex)+len(parse) > 0 {
		t.Fatalf("frontend: %v %v", lex, parse)
	}
	if diagnostics := sema.Check(program); len(diagnostics) > 0 {
		t.Fatalf("checking: %v", diagnostics)
	}
	return program
}

func TestSourceMappingNestedOrigins(t *testing.T) {
	t.Parallel()
	input := "function run(flag: boolean): int {\n if (flag) {\n return 123;\n }\n return 456;\n}"
	program := checkedMappingProgram(t, input)
	generated, mappings, err := GenerateMappedWithTarget(program, "sample", importer.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for fragment, line := range map[string]int{"if flag": 2, "return 123": 3, "return 456": 5} {
		offset := bytes.Index(generated, []byte(fragment))
		found := false
		for _, mapping := range mappings {
			if offset >= mapping.Generated.Start.Offset && offset < mapping.Generated.End.Offset {
				if mapping.Source.Start.Line != line {
					t.Fatalf("%s mapped to %+v", fragment, mapping.Source)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("unmapped %s", fragment)
		}
	}
}

func TestSourceMappingSharedProgram(t *testing.T) {
	t.Parallel()
	program := checkedMappingProgram(t, "const run = (value: int): int => value + 1;")
	var wait sync.WaitGroup
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			generated, mappings, err := GenerateMappedWithTarget(program, "sample", importer.Default(), nil)
			if err != nil || len(mappings) == 0 || strings.Contains(string(generated), mappingMarker) {
				t.Errorf("generation: %v, mappings=%d", err, len(mappings))
			}
		}()
	}
	wait.Wait()
}

func TestSourceMappingMalformedMarkers(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"package sample\nfunc run() {\n" + mappingMarker + "start_broken\n}\n", "package sample\nfunc run() {\n" + mappingMarker + "end_7b7d\n}\n"} {
		if _, _, err := extractSourceMappings([]byte(input)); err == nil {
			t.Fatal("invalid mapping accepted")
		}
	}
}
