package parser

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/puffball1567/kinmokusei/internal/lexer"
)

func TestRejectsExcessiveSyntaxNesting(t *testing.T) {
	const n = 2000
	for name, input := range map[string]string{
		"arrays":        "const x = " + strings.Repeat("[", n) + "0" + strings.Repeat("]", n) + ";",
		"prefix":        "const x = " + strings.Repeat("!", n) + "true;",
		"parentheses":   "const x = " + strings.Repeat("(", n) + "0" + strings.Repeat(")", n) + ";",
		"binary chain":  "const x = 0" + strings.Repeat("+1", n) + ";",
		"member chain":  "const x = a" + strings.Repeat(".b", n) + ";",
		"slice types":   "alias X = int" + strings.Repeat("[]", n) + ";",
		"pointer types": "alias X = " + strings.Repeat("*", n) + "int;",
		"generic types": "alias X = " + strings.Repeat("List<", n) + "int" + strings.Repeat(">", n) + ";",
		"blocks":        "function f():void{" + strings.Repeat("{", n) + strings.Repeat("}", n) + "}",
		"else if":       "function f():void{if(true){}" + strings.Repeat("else if(true){}", n) + "}",
	} {
		t.Run(name, func(t *testing.T) {
			tokens, lexErrors := lexer.Lex("deep.km", input)
			if len(lexErrors) != 0 {
				t.Fatal(lexErrors)
			}
			program, diagnostics := Parse(tokens)
			if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "syntax nesting is too deep") {
				t.Fatalf("diagnostics=%v", diagnostics)
			}
			if diagnostics[0].Span.Path != "deep.km" || diagnostics[0].Span.Start.Line != 1 || diagnostics[0].Span.Start.Column < 1 {
				t.Fatalf("missing source location: %#v", diagnostics[0])
			}
			if program == nil || len(program.Declarations) != 0 || len(program.Imports) != 0 || len(program.Decorators) != 0 {
				t.Fatal("depth-limited file must not expose an unsafe partial tree")
			}
		})
	}
}

func TestOrdinaryDepthAndWideProgramsRemainValid(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"parentheses": "const x=" + strings.Repeat("(", 64) + "1" + strings.Repeat(")", 64) + ";",
		"prefix":      "const x=" + strings.Repeat("!", 64) + "true;",
		"members":     "const x=a" + strings.Repeat(".b", 64) + ";",
		"types":       "alias X=int" + strings.Repeat("[]", 64) + ";",
		"wide":        "function f():void{" + strings.Repeat("f();", 5000) + "}",
	} {
		t.Run(name, func(t *testing.T) {
			tokens, _ := lexer.Lex("ordinary.km", input)
			program, diagnostics := Parse(tokens)
			if len(diagnostics) != 0 || len(program.Declarations) != 1 {
				t.Fatalf("diagnostics=%v", diagnostics)
			}
		})
	}
}

func TestNestingFailureDoesNotModifyTokensOrPoisonLaterParses(t *testing.T) {
	input := "const x=f<List<" + strings.Repeat("List<", 1000) + "int" + strings.Repeat(">", 1001) + ">();"
	tokens, _ := lexer.Lex("deep.km", input)
	original := append(tokens[:0:0], tokens...)
	for range 2 {
		_, diagnostics := Parse(tokens)
		if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "nesting") {
			t.Fatalf("diagnostics=%v", diagnostics)
		}
		if !reflect.DeepEqual(tokens, original) {
			t.Fatal("parser modified caller tokens")
		}
		valid, _ := lexer.Lex("good.km", "function good():int{return 1;}")
		program, diagnostics := Parse(valid)
		if len(diagnostics) != 0 || len(program.Declarations) != 1 {
			t.Fatalf("subsequent parse failed: %v", diagnostics)
		}
	}
}

// A fatal stack overflow cannot be recovered. Exercise the reported sizes in
// a bounded-stack subprocess so a regression fails this test, not its host.
func TestLargeNestingDoesNotCrash(t *testing.T) {
	if os.Getenv("KINMOKUSEI_NESTING_TEST_HELPER") == "1" {
		debug.SetMaxStack(32 << 20)
		for _, input := range []string{
			"const x=" + strings.Repeat("[", 90000),
			"const x=" + strings.Repeat("!", 80000) + "true;",
			"const x=" + strings.Repeat("(", 100000) + "1" + strings.Repeat(")", 100000) + ";",
		} {
			tokens, diagnostics := lexer.Lex("large.km", input)
			if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			_, diagnostics = Parse(tokens)
			if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "nesting") {
				t.Fatalf("diagnostics=%v", diagnostics)
			}
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLargeNestingDoesNotCrash$", "-test.count=1")
	command.Env = append(os.Environ(), "KINMOKUSEI_NESTING_TEST_HELPER=1")
	if output, err := command.CombinedOutput(); err != nil {
		if len(output) > 1500 {
			output = output[:1500]
		}
		t.Fatalf("bounded-stack parser process failed: %v\n%s", err, output)
	}
}
