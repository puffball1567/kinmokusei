package sema

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func nestedNullableLoopContextSource(kind string, depth int) string {
	prelude := "class Item{}"
	parameters := "b:boolean,q:Item,xs:int[]"
	loops := strings.Repeat("while(b){p=q;", depth)
	end := strings.Repeat("}", depth)
	body := "let p:Item|null=q;" + loops + "p=null;" + end
	switch kind {
	case "exception":
		body = "try{" + body + "}catch(_:error){}finally{}"
	case "nested exceptions":
		body = "let p:Item|null=q;" + strings.Repeat("while(b){p=q;try{", depth) + "p=null;" + strings.Repeat("}finally{}}", depth)
	case "inferred arrow":
		body = "const run=()=>{" + body + "return 1;};run();"
	case "inferred returns":
		body = "const run=()=>{let p:Item|null=q;" + loops + "if(b){return 1;}p=null;" + end + "return 0;};run();"
	case "constructor":
		return prelude + "class Holder{constructor(" + parameters + "){" + body + "}}"
	case "field initializer":
		return prelude + "let b=false;const q=new Item();const xs=[1];class Holder{public value:()=>int=()=>{" + body + "return 1;};}"
	case "label":
		body = "outer:while(b){" + body + "continue outer;}"
	case "outer continue":
		body = "let p:Item|null=q;outer:while(b){" + loops + "p=null;if(b){continue outer;}" + end + "}"
	case "capture":
		body = "let p:Item|null=q;const run=():void=>{" + loops + "p=null;" + end + "};run();"
	case "local arrow":
		body = "let p:Item|null=q;" + strings.Repeat("while(b){const peer=()=>q;p=q;", depth) + "p=null;" + end
	}
	return prelude + "function f(" + parameters + "):void{" + body + "}"
}

func TestNestedNullableLoopEffectResources(t *testing.T) {
	if os.Getenv("KINMOKUSEI_LOOP_EFFECT_HELPER") == "1" {
		for _, kind := range []string{"exception", "nested exceptions", "inferred arrow", "inferred returns", "constructor", "field initializer", "label", "outer continue", "capture", "local arrow"} {
			for _, loop := range []string{"while(b){", "for(;b;){", "for(const _ of xs){"} {
				for _, depth := range []int{12, 24, 40} {
					start := time.Now()
					if messages := checkSource(t, strings.ReplaceAll(nestedNullableLoopContextSource(kind, depth), "while(b){", loop)); len(messages) != 0 {
						t.Fatalf("%s depth %d: %v", kind, depth, messages)
					}
					t.Logf("%s/%s depth %d: %s", kind, loop, depth, time.Since(start))
				}
			}
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNestedNullableLoopEffectResources$", "-test.count=1", "-test.v")
	command.Env = append(os.Environ(), "KINMOKUSEI_LOOP_EFFECT_HELPER=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("loop effect resource check failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}
