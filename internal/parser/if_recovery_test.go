package parser

import "testing"

func TestIncompleteElseReportsDiagnosticsWithoutPanicking(t *testing.T) {
	for _, input := range []string{
		`function use(): void { if (true) {} else`,
		`function use(): void { if (true) {} else }`,
		`function use(): void { if (true) {} else if (false) {} else`,
		`class A{constructor(){if((0%%){}else`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, count := parseSource(t, input); count == 0 {
				t.Fatal("expected a syntax diagnostic")
			}
		})
	}
}
