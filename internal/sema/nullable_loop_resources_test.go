package sema

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
)

func nestedNullableLoopSource(kind, value string, depth int) string {
	prelude, valueType, seed := "", "*int", "q"
	switch value {
	case "checked pointer":
		seed = "q!"
	case "class":
		prelude, valueType = "class Item{}", "Item"
	}
	loop := "while(b){p=" + seed + ";"
	switch kind {
	case "for":
		loop = "for(;b;){p=" + seed + ";"
	case "range":
		loop = "for(const _ of xs){p=" + seed + ";"
	}
	return prelude + "function f(b:boolean,q:" + valueType + ",xs:int[]):void{let p:" + valueType + "|null=" + seed + ";" + strings.Repeat(loop, depth) + "p=null;" + strings.Repeat("}", depth) + "}"
}

func TestNestedNullableLoopCheckResources(t *testing.T) {
	if os.Getenv("KINMOKUSEI_LOOP_RESOURCES_HELPER") == "1" {
		for _, kind := range []string{"while", "for", "range"} {
			for _, value := range []string{"raw pointer", "checked pointer", "class"} {
				for _, depth := range []int{12, 24, 40} {
					started := time.Now()
					if messages := checkSource(t, nestedNullableLoopSource(kind, value, depth)); len(messages) != 0 {
						t.Fatalf("%s depth %d: %v", kind, depth, messages)
					}
					t.Logf("%s/%s depth %d: %s", kind, value, depth, time.Since(started))
				}
			}
		}
		return
	}
	// This is a coarse process watchdog, not a machine-speed benchmark. The
	// old recursive re-solving grows exponentially and cannot finish the cases.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNestedNullableLoopCheckResources$", "-test.count=1", "-test.v")
	command.Env = append(os.Environ(), "KINMOKUSEI_LOOP_RESOURCES_HELPER=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("nested loop resource check failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}

func BenchmarkNestedNullableLoopCheck(b *testing.B) {
	for _, depth := range []int{6, 12, 24, 40} {
		b.Run(fmt.Sprint(depth), func(b *testing.B) {
			input := nestedNullableLoopSource("while", "class", depth)
			b.ReportAllocs()
			for range b.N {
				tokens, _ := lexer.Lex("loops.km", input)
				program, diagnostics := parser.Parse(tokens)
				if len(diagnostics) != 0 {
					b.Fatal(diagnostics)
				}
				if diagnostics := Check(program); len(diagnostics) != 0 {
					b.Fatal(diagnostics)
				}
			}
		})
	}
}
