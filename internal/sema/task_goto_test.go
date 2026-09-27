package sema

import (
	"strings"
	"testing"
)

func TestTaskGotoRejectsOwnershipBypasses(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"skip await":                  `const task=go work();if(flag){goto done;}await task;done:return;`,
		"repeat await":                `const task=go work();again:await task;if(flag){goto again;}`,
		"repeat detach":               `const task=go work();again:detach task;if(flag){goto again;}`,
		"leave nested scope":          `{const task=go work();if(flag){goto done;}await task;}done:return;`,
		"restart pending declaration": `again:work();const task=go work();if(flag){goto again;}await task;`,
		"join consumed and pending":   `const task=go work();if(flag){await task;goto done;}done:await task;`,
		"leave loop initializer":      `for(const task=go work();flag;){if(flag){goto done;}await task;}done:return;`,
		"nested forward edges":        `const task=go work();if(flag){goto first;}await task;goto last;first:goto last;last:return;`,
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := checkSource(t, `function work():void{}function f(flag:boolean):void{`+body+`}`)
			if !strings.Contains(strings.Join(diagnostics, "\n"), `Task "task"`) {
				t.Fatalf("expected Task ownership diagnostic, got %v", diagnostics)
			}
		})
	}
}

func TestTaskGotoAllowsStableOwnership(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"forward pending":      `const task=go work();goto done;done:await task;`,
		"forward consumed":     `const task=go work();await task;goto done;done:return;`,
		"both pending":         `const task=go work();if(flag){goto done;}work();done:await task;`,
		"both consumed":        `const task=go work();if(flag){await task;goto done;}await task;done:return;`,
		"pending backedge":     `const task=go work();again:if(flag){flag=false;goto again;}await task;`,
		"fresh task per jump":  `again:work();const task=go work();await task;if(flag){flag=false;goto again;}`,
		"outer task survives":  `const task=go work();{if(flag){goto done;}}done:await task;`,
		"shadowed nested task": `const task=go work();{const task=go work();await task;goto done;}done:await task;`,
		"independent callback": `const task=go work();const callback=():void=>{goto done;done:return;};callback();await task;`,
		"inside repeated loop": `for(let i=0;i<3;i++){const task=go work();if(flag){goto done;}work();done:await task;}`,
		"both branches jump":   `const task=go work();if(flag){goto done;}else{goto done;}done:await task;`,
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostics := checkSource(t, `function work():void{}function f(flag:boolean):void{`+body+`}`); len(diagnostics) != 0 {
				t.Fatalf("stable ownership rejected: %v", diagnostics)
			}
		})
	}
}
