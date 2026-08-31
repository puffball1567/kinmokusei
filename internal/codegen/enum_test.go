package codegen

import (
	"strings"
	"testing"
)

func TestGeneratesEnumAsNamedTypeAndTypedConstants(t *testing.T) {
	generated := string(generateCheckedSource(t, `
enum Status: int16 { Pending, Running = 4, Complete, Negative = -2 }
function use(value: Status): boolean { return value === Status.Complete; }
`))
	for _, want := range []string{
		"type Status int16",
		"StatusPending",
		"StatusRunning",
		"StatusComplete",
		"StatusNegative",
		"Status = 0",
		"Status = 4",
		"Status = 5",
		"Status = -2",
		"value == StatusComplete",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated Go missing %q:\n%s", want, generated)
		}
	}
}
