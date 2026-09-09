package sema

import (
	"strings"
	"testing"
)

func TestReturnTerminationRejectsFallthroughAndTrailingStatements(t *testing.T) {
	for _, input := range []string{
		`function f(): int { return 1; {} }`,
		`function f(): int { return 1; const unused = 2; }`,
		`function f(): int { goto end; return 1; end: {} }`,
		`function f(): int { switch (1) { default { break; return 1; } } }`,
		`function f(stop: boolean): int { switch (1) { default { if (stop) { break; } return 1; } } }`,
		`function f(): int { select { default { break; return 1; } } }`,
		`function f(): int { outer: switch (1) { default { while (true) { break outer; } return 1; } } }`,
	} {
		t.Run(input, func(t *testing.T) {
			for _, diagnostic := range checkSource(t, input) {
				if strings.Contains(diagnostic, "may complete without returning") {
					return
				}
			}
			t.Fatal("expected a return-path diagnostic")
		})
	}
}

func TestReturnTerminationPreservesNestedBreaksAndReturns(t *testing.T) {
	for _, input := range []string{
		`function f(): int { { return 1; } }`,
		`function f(): int { goto end; end: return 1; }`,
		`function f(): int { switch (1) { default { while (true) { break; } return 1; } } }`,
		`function f(): int { switch (1) { default { inner: while (true) { break inner; } return 1; } } }`,
		`function f(): int { switch (1) { default { switch (2) { default { break; } } return 1; } } }`,
		`function f(): int { select { default { for (const i of 2) { break; } return 1; } } }`,
		`function f(): int { select { default { for (; true; ) { break; } return 1; } } }`,
		`function f(): int { switch (1) { default { const callback = (): void => { while (true) { break; } }; return 1; } } }`,
	} {
		t.Run(input, func(t *testing.T) {
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}
}
