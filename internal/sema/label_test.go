package sema

import (
	"strings"
	"testing"
)

func TestGotoAndLabeledBranches(t *testing.T) {
	diagnostics := checkSource(t, `
function classify(value: int): int {
  goto dispatch;
  dispatch: if (value < 0) { return -1; }
  let total = 0;
  count: total++;
  if (total < value) { goto count; }
  outer: for (let row = 0; row < 3; row++) {
    for (let column = 0; column < 3; column++) {
      if (column === 1) { continue outer; }
      if (row === 2) { break outer; }
    }
  }
  return total;
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestLabelFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"undefined goto", `function bad(): void { goto missing; }`, `undefined label "missing"`},
		{"duplicate", `function bad(): void { goto same; same: call(); same: call(); }`, `duplicate label "same"`},
		{"unused", `function bad(): void { unused: call(); }`, `label "unused" is declared but not used`},
		{"break not enclosing", `function bad(): void { target: while (true) { break; } break target; }`, `does not enclose this branch`},
		{"continue switch", `function bad(value: int): void { target: switch (value) { default { continue target; } } }`, `must target a loop`},
		{"break block", `function bad(): void { target: { break target; } }`, `must target a loop, switch, or select`},
		{"label declaration", `function bad(): void { goto value; value: let item = 1; }`, `cannot be attached to a variable declaration`},
		{"jump over declaration", `function bad(): int { goto done; let value = 1; done: return value; }`, `jumps over declaration of "value"`},
		{"jump into block", `function bad(): void { goto inside; if (true) { inside: call(); } }`, `cannot jump into a nested block`},
		{"jump out of try", `function bad(err: error): void { try { goto done; } catch (caught: error) {} done: call(); }`, `cannot cross a try, catch, or finally boundary`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
