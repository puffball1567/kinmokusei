package sema

import (
	"strings"
	"testing"
)

func TestTaskBranchExits(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"conditional break":    `while(flag){const task=go work();if(flag){break;}await task;}`,
		"conditional continue": `while(flag){const task=go work();if(flag){continue;}await task;}`,
		"for break":            `for(let i=0;i<2;i++){const task=go work();if(flag){break;}await task;}`,
		"range continue":       `for(const i of [1,2]){const task=go work();if(flag){continue;}await task;}`,
		"switch break":         `switch(1){default{const task=go work();if(flag){break;}await task;}}`,
		"select break":         `select{default{const task=go work();if(flag){break;}await task;}}`,
		"type switch break":    `let input:io.Reader=nil;switch(input){case nil{const task=go work();if(flag){break;}await task;}default{}}`,
		"outer break":          `outer:while(flag){const task=go work();while(flag){if(flag){break outer;}break;}await task;}`,
		"outer continue":       `outer:while(flag){const task=go work();while(flag){if(flag){continue outer;}break;}await task;}`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, `import go io from "io";function work():void{}function f(flag:boolean):void{`+body+`}`)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, `Task "task" must be consumed`) {
				t.Fatalf("missing branch-exit diagnostic: %v", diagnostics)
			}
		})
	}
}

func TestBranchTargetsPreserveNullableFlow(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"loop inside switch": `switch(1){default{while(p!==null){p=null;break;}return p.value;}}`,
		"loop inside select": `select{default{while(p!==null){p=null;break;}return p.value;}}`,
		"labeled outer loop": `outer:while(p===null){while(flag){break outer;}return 0;}return p.value;`,
		"labeled switch":     `outer:switch(1){default{while(flag){p=null;break outer;}p=new Item();}}return p.value;`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, `class Item{public value:int=1;}function f(p:Item|null,flag:boolean):int{`+body+`}`)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, "must be checked against null") {
				t.Fatalf("missing nullable diagnostic: %v", diagnostics)
			}
		})
	}
}

func TestBranchTargetsAllowSurvivingTasks(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"outer task":                   `const task=go work();while(flag){break;}await task;`,
		"goto retains outer scope":     `const task=go work();switch(1){default{goto done;}}done:await task;`,
		"loop task outside switch":     `while(flag){const task=go work();switch(1){default{break;}}await task;break;}`,
		"loop task outside inner loop": `while(flag){const task=go work();while(flag){break;}await task;break;}`,
		"consumed before break":        `while(flag){const task=go work();await task;if(flag){break;}}`,
		"consumed before continue":     `while(flag){const task=go work();await task;if(flag){continue;}}`,
		"arrow has own targets":        `while(flag){const task=go work();const callback=():void=>{while(flag){break;}};callback();await task;break;}`,
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostics := checkSource(t, `function work():void{}function f(flag:boolean):void{`+body+`}`); len(diagnostics) != 0 {
				t.Fatalf("surviving or consumed Task rejected: %v", diagnostics)
			}
		})
	}
}
