package parser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
)

func TestOptionalTerminatorForms(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"imports":              "import go fmt from \"fmt\"\nimport { Item } from \"./item\"\nconst item = 1",
		"types":                "constraint Integer = ~int\n | ~int64\nalias Text = string\ntype ID = distinct int",
		"class":                "class Box {\npublic value: int = 1\nprivate extra: string\nconstructor() { this.extra = \"x\" }\npublic function read(): int { return this.value }\n}",
		"struct":               "struct Point {\npublic x: int\npublic y: int\n}",
		"interface":            "interface Reader {\nfunction read(): int\nfunction close(): void\n}",
		"export":               `function add(): int { return 1 } export c("add") { add }`,
		"bindings and updates": "function run(): void {\nlet value = 1\nvalue += 2\nvalue++\nvalue--\nconst [a, b] = pair();\n[a, b] = pair()\ncall()\nreturn\n}",
		"loops":                "function run(): void {\nfor (let i = 0; i < 3; i++) {\nif (i == 1) { continue }\nbreak\n}\nwhile (true) { break }\nfor (const value of [1,2]) { use(value) }\n}",
		"exceptions":           "function run(): void {\ntry { throw fail() } catch (_: error) { throw } finally { finish() }\n}",
		"effects":              "function run(channel: GoChannel<int>): void {\nchannel <- 1\nconst value = <-channel\ndefer finish()\ngo work()\ndetach go work()\n}",
		"switch":               "function run(): void {\nswitch (1) {\ncase 1 { fallthrough }\ndefault { break }\n}\n}",
		"labels":               "function run(): void {\nouter: while (true) {\nbreak outer\n}\ngoto done\ndone: return\n}",
		"multiline expression": "function run(): int {\nconst value = (1 +\n2)\nreturn value\n+ 3\n}",
		"multiline call":       "function run(): void {\ncall\n(1,\n2)\nother(3)\n}",
		"comments":             "const 名 = 1 /* block\ncomment */ const other = 2 // line\nconst last = 3",
		"crlf":                 "const first = 1\r\nconst second = 2\r\n",
		"arrow in loop header": "function run(): void { for (let next = (): int => {\nreturn 1\n}; next() < 3; ) { break } }",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tokens, lexDiagnostics := lexer.Lex("terminators.km", input)
			if len(lexDiagnostics) != 0 {
				t.Fatal(lexDiagnostics)
			}
			before := append(tokens[:0:0], tokens...)
			program, diagnostics := Parse(tokens)
			if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			if len(program.Declarations)+len(program.Imports) == 0 {
				t.Fatal("empty program")
			}
			if !reflect.DeepEqual(tokens, before) {
				t.Fatal("parser mutated lexer tokens")
			}
		})
	}
}

func TestOptionalTerminatorsDoNotReplaceRequiredSyntax(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"same-line binding":      `const first = 1 const second = 2`,
		"same-line import":       `import go fmt from "fmt" function run(): void {}`,
		"same-line statements":   `function run(): void { call() other() }`,
		"incomplete initializer": "const first =\nconst second = 2",
		"incomplete union":       "constraint Bad = ~int |\nconst second = 2",
		"for initializer":        "function run(): void { for (let i = 0\ni < 3; i++) {} }",
		"for assignment":         "function run(): void { let i = 0\nfor (i = 0\ni < 3; i++) {} }",
		"for condition":          "function run(): void { for (let i = 0; i < 3\ni++) {} }",
		"missing paren":          "function run(): void { call(1\n}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, count := parseSource(t, input); count == 0 {
				t.Fatal("expected syntax error")
			}
		})
	}
}

func TestRestrictedNewlineAndContinuationAST(t *testing.T) {
	t.Parallel()
	program, count := parseSource(t, "function run(): void {\nreturn /* boundary\n*/ call()\nthrow\nfail()\nbreak\nnext()\n}")
	if count != 0 {
		t.Fatalf("got %d diagnostics", count)
	}
	body := program.Declarations[0].(*ast.FunctionDecl).Body
	if len(body.Statements) != 6 {
		t.Fatalf("statements: %#v", body.Statements)
	}
	if body.Statements[0].(*ast.ReturnStmt).Value != nil || !body.Statements[2].(*ast.ThrowStmt).Bare || body.Statements[4].(*ast.BranchStmt).Label != "" {
		t.Fatal("newline consumed a value or label")
	}
	program, count = parseSource(t, "function run(): int {\nreturn call\n(1)\n+ 2\n}")
	if count != 0 {
		t.Fatalf("got %d diagnostics", count)
	}
	value := program.Declarations[0].(*ast.FunctionDecl).Body.Statements[0].(*ast.ReturnStmt).Value
	if _, ok := value.(*ast.BinaryExpr); !ok {
		t.Fatalf("multiline expression = %T", value)
	}
}

func TestImplicitTerminatorSpanAndRecovery(t *testing.T) {
	t.Parallel()
	input := "const 名 = 1 // comment\nconst next = 2"
	program, count := parseSource(t, input)
	if count != 0 {
		t.Fatalf("got %d diagnostics", count)
	}
	if end := program.Declarations[0].GetSpan().End.Offset; end != strings.Index(input, " //") {
		t.Fatalf("span consumed comment or next token: %d", end)
	}
	tokens, _ := lexer.Lex("recovery.km", "const broken =\nconst recovered = 2")
	program, diagnostics := Parse(tokens)
	if len(diagnostics) == 0 || len(program.Declarations) != 1 || program.Declarations[0].(*ast.VariableDecl).Name != "recovered" {
		t.Fatalf("recovery: %#v %v", program, diagnostics)
	}
}
