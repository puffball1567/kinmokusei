package compiler

import "testing"

func TestAcceptedReturnsGenerateValidGo(t *testing.T) {
	for _, input := range []string{
		`function f(): int { return 1; {} }`,
		`function f(): int { return 1; const unused = 2; }`,
		`function f(): int { { return 1; } }`,
		`function f(): int { switch (1) { default { break; return 1; } } }`,
		`function f(): int { goto end; return 1; end: {} }`,
		`function f(stop: boolean): int { switch (1) { default { if (stop) { break; } return 1; } } }`,
		`function f(): int { select { default { break; return 1; } } }`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, _, err := compilePipelineProperty(input); err != nil {
				t.Fatal(err)
			}
		})
	}
}
