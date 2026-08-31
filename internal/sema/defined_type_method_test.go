package sema

import (
	"strings"
	"testing"
)

func TestDefinedTypeReceiverMethodSemanticMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"value receiver", `type UserID = distinct string; public function text(this: UserID): string { return string(this); } function use(value: UserID): string { return value.text(); }`},
		{"defined result", `type UserID = distinct string; public function joined(this: UserID, suffix: UserID): UserID { return this + suffix; } function use(value: UserID): UserID { return value.joined(value); }`},
		{"pointer receiver mutation", `type Score = distinct int; public function add(this: *Score, delta: Score): void { *this += delta; } function use(value: *Score, delta: Score): void { value.add(delta); }`},
		{"addressable value pointer call", `type Score = distinct int; function increment(this: *Score): void { *this += 1; } function use(value: Score): Score { let copy = value; copy.increment(); return copy; }`},
		{"value method through pointer", `type Score = distinct int; function read(this: Score): int { return int(this); } function use(value: *Score): int { return value.read(); }`},
		{"method value", `type Label = distinct string; function size(this: Label): int { return len(this); } function use(value: Label): int { const size = value.size; return size(); }`},
		{"nil pointer receiver", `type Score = distinct int; function isNil(this: *Score): boolean { return this === nil; } function use(value: *Score): boolean { return value.isNil(); }`},
		{"Result method", `type UserID = distinct string; function present(this: UserID): Result<UserID> { return ok(this); } function use(value: UserID): Result<UserID> { return value.present(); }`},
		{"generic value and pointer receivers", `type Values<T> = distinct T[]; public function size<U>(this: Values<U>): int { return len(this); } public function push<U>(this: *Values<U>, value: U): void { *this = append(*this, value); } function use(values: Values<string>): int { let copy = values; copy.push("onsen"); return copy.size(); }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestDefinedTypeReceiverMethodFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"alias receiver", `alias UserID = string; function text(this: UserID): string { return this; }`, "is an alias; methods require a distinct defined type"},
		{"generic receiver missing binders", `type Values<T> = distinct T[]; function size(this: Values<int>): int { return len(this); }`, "requires 1 receiver type parameters, got 0"},
		{"pointer underlying receiver", `type Ref = distinct *int; function read(this: Ref): int { return int(*this); }`, "has a pointer or interface underlying type"},
		{"unknown receiver", `function text(this: Missing): string { return ""; }`, "is not a native struct or defined type"},
		{"duplicate method", `type Score = distinct int; function read(this: Score): int { return int(this); } function read(this: *Score): int { return int(*this); }`, "duplicate defined type method"},
		{"pointer method temporary", `type Score = distinct int; function increment(this: *Score): void { *this += 1; } function bad(): void { Score(1).increment(); }`, "requires an addressable Score value"},
		{"missing method", `type Score = distinct int; function bad(value: Score): int { return value.missing(); }`, `has no method "missing"`},
		{"wrong return", `type Score = distinct int; function text(this: Score): string { return int(this); }`, "cannot use int as string"},
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
