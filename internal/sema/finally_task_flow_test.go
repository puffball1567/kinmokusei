package sema

import (
	"strings"
	"testing"
)

func TestFinallyConsumesTaskOnReturn(t *testing.T) {
	t.Parallel()
	prelude := `import go errors from "errors";function work():int{return 7;}function load():Result<int>{return ok(3);}`
	for name, source := range map[string]string{
		"return":                  `function f():int{const task=go work();try{return 1;}finally{const n=await task;}}`,
		"consumed before return":  `function f():int{const task=go work();try{const n=await task;return n;}finally{}}`,
		"detached in finally":     `function f():int{const task=go work();try{return 1;}finally{detach task;}}`,
		"nested return":           `function f():int{const task=go work();try{try{return 1;}finally{}}finally{const n=await task;}}`,
		"Result propagation":      `function f():Result<int>{const task=go work();try{const n=load()?;return ok(n);}finally{const v=await task;}}`,
		"raw error propagation":   `function f():Result<int>{const task=go work();try{errors.New("failed")?;return ok(1);}finally{const v=await task;}}`,
		"normal and return paths": `function f(flag:boolean):int{const task=go work();try{if(flag){return 1;}}finally{const n=await task;}return 2;}`,
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostics := checkSource(t, prelude+source); len(diagnostics) != 0 {
				t.Fatalf("safe finally Task consumption rejected: %v", diagnostics)
			}
		})
	}
}

func TestFinallyRejectsUnconsumedTaskOnReturn(t *testing.T) {
	t.Parallel()
	prelude := `function work():int{return 7;}`
	for name, source := range map[string]string{
		"empty finally":     `function f():int{const task=go work();try{return 1;}finally{}}`,
		"try-local task":    `function f():int{try{const task=go work();return 1;}finally{}}`,
		"early return path": `function f(flag:boolean):int{const task=go work();try{if(flag){return 1;}const n=await task;return n;}finally{}}`,
		"conditional await": `function f(flag:boolean):int{const task=go work();try{return 1;}finally{if(flag){const n=await task;}}}`,
		"already consumed":  `function f():int{const task=go work();try{const n=await task;return n;}finally{const m=await task;}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostics := checkSource(t, prelude+source); !strings.Contains(strings.Join(diagnostics, "\n"), "Task") {
				t.Fatalf("unsafe finally Task ownership accepted: %v", diagnostics)
			}
		})
	}
}
