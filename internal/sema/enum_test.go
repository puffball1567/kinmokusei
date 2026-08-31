package sema

import (
	"strings"
	"testing"
)

func TestEnumSemanticSuccessMatrix(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"default and explicit values", `enum Status { Pending, Running = 4, Complete, } function use(): Status { return Status.Complete; }`},
		{"fixed signed boundary", `enum Code: int8 { Minimum = -128, Zero = 0, Maximum = 127, } function use(value: Code): boolean { return value === Code.Maximum; }`},
		{"fixed unsigned boundary", `enum Code: uint16 { Empty, Maximum = 65535, } function use(): Code { return Code.Maximum; }`},
		{"switch and map key", `enum Status { Pending, Complete } function use(value: Status, values: Map<Status, string>): string { switch (value) { case Status.Pending { return "pending"; } default { return ""; } } }`},
		{"explicit constant arithmetic", `enum Code: int16 { First = 4 * 2, Second, } function use(): Code { return Code.Second; }`},
		{"value and pointer receiver methods", `enum Status: int8 { Pending, Running, Complete } public function active(this: Status): boolean { return this === Status.Running; } public function advance(this: *Status): Status { *this = Status(int8(*this) + int8(1)); return *this; } function use(): Status { let value = Status.Pending; value.advance(); return value; }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestEnumSemanticFailureMatrix(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"empty", `enum Status {}`, "must declare at least one member"},
		{"duplicate member", `enum Status { Pending, Pending }`, `duplicate enum member "Pending"`},
		{"blank member", `enum Status { _ }`, "enum member name cannot be '_'"},
		{"generated name collision", `enum Status { Pending } const StatusPending: int = 1;`, "generated Go name"},
		{"noninteger underlying", `enum Status: string { Pending }`, "underlying type must be an integer type"},
		{"unknown member", `enum Status { Pending } function use(): Status { return Status.Missing; }`, `has no member "Missing"`},
		{"positive overflow", `enum Code: int8 { Invalid = 128 }`, "cannot be represented as Code"},
		{"negative overflow", `enum Code: uint16 { Invalid = -1 }`, "cannot be represented as Code"},
		{"nonconstant", `let next: int = 1; enum Code { Value = next }`, "initializer must be an integer constant expression"},
		{"enum reference initializer", `enum Code { First, Second = Code.First + 1 }`, "without enum-member references"},
		{"wrong assignment", `enum Status { Pending } function use(value: int): Status { return value; }`, "cannot use int as Status"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
