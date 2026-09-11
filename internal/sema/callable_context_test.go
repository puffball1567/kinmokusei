package sema

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestCallableControlStateNestedRestoration(t *testing.T) {
	t.Parallel()
	for _, catches := range [][]int{nil, {}, {11, 23}} {
		t.Run(fmt.Sprint(catches), func(t *testing.T) {
			c := &Checker{callableControlState: callableControlState{
				result: builtins["int"], loopDepth: 2, breakableDepth: 3,
				exceptionDepth: 4, catchTargets: catches,
			}, currentClass: "Owner", inConstructor: true}
			outer := c.callableControlState
			savedOuter := c.enterCallableControl()
			if !reflect.DeepEqual(c.callableControlState, callableControlState{result: outer.result}) {
				t.Fatalf("inner control state was not reset: %#v", c.callableControlState)
			}
			c.result = builtins["string"]
			c.loopDepth, c.breakableDepth, c.exceptionDepth = 5, 6, 7
			c.catchTargets = append(c.catchTargets, 37)
			inner := c.callableControlState
			savedInner := c.enterCallableControl()
			c.result = builtins["void"]
			c.catchTargets = append(c.catchTargets, 41)
			c.callableControlState = savedInner
			if !reflect.DeepEqual(c.callableControlState, inner) {
				t.Fatalf("inner state was not restored: %#v", c.callableControlState)
			}
			c.callableControlState = savedOuter
			if !reflect.DeepEqual(c.callableControlState, outer) {
				t.Fatalf("outer state was not restored: %#v", c.callableControlState)
			}
			if c.currentClass != "Owner" || !c.inConstructor {
				t.Fatal("control boundary changed receiver context")
			}
		})
	}
}

func TestCallableControlBoundaryMatrix(t *testing.T) {
	t.Parallel()
	wrappers := []struct{ name, source string }{
		{"function", `function run(): void { %s }`},
		{"constructor", `class Owner { constructor() { %s } }`},
		{"class method", `class Owner { public function run(): void { %s } }`},
		{"static method", `class Owner { public static function run(): void { %s } }`},
		{"struct method", `struct Owner { public function run(): void { %s } }`},
		{"defined type method", `type Owner = distinct int; public function run(this: Owner): void { %s }`},
		{"arrow", `function run(): void { const callback = (): void => { %s }; callback(); }`},
	}
	for _, wrapper := range wrappers {
		t.Run(wrapper.name, func(t *testing.T) {
			t.Parallel()
			body := `
while (true) {
  const nested = (): string => { return "nested"; };
  nested();
  break;
}
try {} catch (_: error) {
  const nested = (): int => { try { return 1; } finally {} };
  nested();
  throw;
}
return;`
			if wrapper.name == "constructor" {
				body = strings.TrimSuffix(body, "return;")
			}
			if messages := checkSource(t, fmt.Sprintf(wrapper.source, body)); len(messages) != 0 {
				t.Fatalf("nested callable changed enclosing control context: %v", messages)
			}
			for _, branch := range []string{"break", "continue"} {
				body := fmt.Sprintf(`while (true) { const nested = (): void => { %s; }; nested(); break; }`, branch)
				messages := checkSource(t, fmt.Sprintf(wrapper.source, body))
				if len(messages) != 1 || !strings.Contains(messages[0], branch+" may only be used") {
					t.Fatalf("%s crossed a callable boundary or damaged outer loop: %v", branch, messages)
				}
			}
		})
	}
}
