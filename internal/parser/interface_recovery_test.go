package parser

import "testing"

func TestInvalidInterfaceMembersMakeRecoveryProgress(t *testing.T) {
	for _, input := range []string{
		`interface Broken { return 1; }`,
		`interface Broken { function read(): int; const value = 1; }`,
		`interface Broken { class Value { function read(): int { return 1; } } }`,
		`interface Broken { ; ; return`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, count := parseSource(t, input); count == 0 {
				t.Fatal("expected a syntax diagnostic")
			}
		})
	}
}
