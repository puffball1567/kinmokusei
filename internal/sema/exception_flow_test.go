package sema

import (
	"strings"
	"testing"
)

const exceptionFlowPrelude = `class Item{public value:int=1;}
class Box{constructor(public item:Item|null){}}
class NotFoundException extends Exception{constructor(){super("missing");}}
function stop(err:error):void{throw err;}
`

func TestExceptionFlowRejectsIntermediateNulls(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"caught conditional throw":      `try{p=null;if(flag){throw err;}p=new Item();}catch(_:error){const n=p.value;}`,
		"caught call":                   `try{p=null;stop(err);p=new Item();}catch(_:error){const n=p.value;}`,
		"finally on return":             `try{p=null;return;}finally{const n=p.value;}`,
		"finally after restored try":    `try{p=null;if(flag){throw err;}p=new Item();}finally{const n=p.value;}`,
		"finally after restored catch":  `try{throw err;}catch(_:error){p=null;if(flag){throw err;}p=new Item();}finally{const n=p.value;}`,
		"outer catch sees nested throw": `try{try{p=null;if(flag){throw err;}p=new Item();}finally{}}catch(_:error){const n=p.value;}`,
		"member restored at normal end": `box.item=new Item();try{box.item=null;if(flag){throw err;}box.item=new Item();}catch(_:error){const n=box.item.value;}`,
	} {
		t.Run(name, func(t *testing.T) {
			input := exceptionFlowPrelude + `function f(err:error,flag:boolean,box:Box):void{let p:Item|null=new Item();` + body + `}`
			if diagnostics := checkSource(t, input); !strings.Contains(strings.Join(diagnostics, "\n"), "must be checked against null") {
				t.Fatalf("expected nullable diagnostic: %v", diagnostics)
			}
		})
	}
}

func TestExceptionFlowKeepsSafeNarrowing(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"check in catch":                        `try{p=null;stop(err);p=new Item();}catch(_:error){if(p!==null){const n=p.value;}}`,
		"check in finally":                      `try{p=null;return;}finally{if(p!==null){const n=p.value;}}`,
		"restore in finally":                    `try{p=null;if(flag){throw err;}p=new Item();}catch(_:error){}finally{p=new Item();}const n=p.value;`,
		"no mutation":                           `try{if(flag){throw err;}}catch(_:error){const n=p.value;}finally{const n=p.value;}`,
		"normal completion retains restoration": `try{p=null;p=new Item();}catch(_:error){p=new Item();}const n=p.value;`,
		"unreachable writes":                    `try{return;p=null;}finally{const n=p.value;}`,
		"shadowed local":                        `try{let p:Item|null=null;if(flag){throw err;}}catch(_:error){const n=p.value;}`,
		"independent callback":                  `try{const callback=():void=>{let p:Item|null=null;throw err;};}finally{const n=p.value;}`,
		"sibling catch isolation":               `try{throw err;}catch(_:NotFoundException){p=null;}catch(_:error){const n=p.value;}`,
	} {
		t.Run(name, func(t *testing.T) {
			input := exceptionFlowPrelude + `function f(err:error,flag:boolean):void{let p:Item|null=new Item();` + body + `}`
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("safe exception flow rejected: %v", diagnostics)
			}
		})
	}
}
