package sema

import (
	"strings"
	"testing"
)

const forPostPrelude = `class Item{public value:int=1;}
class Box{constructor(public item:Item|null){}}
function clear(box:Box):void{box.item=null;}
function work():void{}
`

func TestForPostFlowRejectsUnsafeContinue(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"nullable post":              `for(;flag;p.value++){if(flag){p=null;continue;}p=new Item();}`,
		"labeled continue post":      `outer:for(;flag;p.value++){while(flag){p=null;continue outer;}p=new Item();}`,
		"post mutation backedge":     `if(box.item===null){return;}for(;flag;clear(box)){const n=box.item.value;continue;}`,
		"post after switch continue": `for(;flag;p.value++){switch(flag){case true{p=null;continue;}default{}}p=new Item();}`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, forPostPrelude+`function f(p:Item|null,box:Box,flag:boolean):void{`+body+`}`)
			if !strings.Contains(strings.Join(diagnostics, "\n"), "must be checked against null") {
				t.Fatalf("expected nullable diagnostic: %v", diagnostics)
			}
		})
	}
	diagnostics := checkSource(t, forPostPrelude+`function f(flag:boolean):void{const task=go work();for(;flag;await task){continue;}await task;}`)
	if !strings.Contains(strings.Join(diagnostics, "\n"), "already have been consumed") {
		t.Fatalf("expected repeated Task consumption: %v", diagnostics)
	}
}

func TestForPostFlowAllowsRestoredState(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"restored by post":       `p=new Item();for(;flag;p=new Item()){const n=p.value;p=null;continue;}`,
		"conditional continue":   `p=new Item();for(;flag;p=new Item()){const n=p.value;if(flag){p=null;continue;}p=null;}`,
		"scoped continue locals": `p=new Item();for(;flag;p=new Item()){const local=1;const n=p.value;p=null;continue;}`,
		"labeled continue":       `outer:for(let current:Item|null=new Item();flag;current=new Item()){const n=current.value;while(flag){current=null;continue outer;}current=null;}`,
		"fresh task in post":     `for(;flag;await go work()){flag=false;continue;}`,
		"break skips task post":  `const task=go work();for(;flag;await task){break;}await task;`,
		"break skips mutation":   `p=new Item();for(;flag;p=null){break;}const n=p.value;`,
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostics := checkSource(t, forPostPrelude+`function f(p:Item|null,flag:boolean):void{`+body+`}`); len(diagnostics) != 0 {
				t.Fatalf("safe post flow rejected: %v", diagnostics)
			}
		})
	}
}

func TestForPostKeepsLexicalTypeChecks(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`for(;flag;work(local)){const local=1;continue;}`,
		`for(;flag;missing()){break;}`,
	} {
		diagnostics := checkSource(t, forPostPrelude+`function f(flag:boolean):void{`+body+`}`)
		if !strings.Contains(strings.Join(diagnostics, "\n"), "undefined ") {
			t.Fatalf("expected name diagnostic: %v", diagnostics)
		}
	}
}
