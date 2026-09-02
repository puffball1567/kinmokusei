package codegen

import (
	"strings"
	"testing"
)

func TestGenericAliasesAreExpandedForGo123(t *testing.T) {
	generated := string(generateCheckedSource(t, `
class Box<T> { constructor(public value: T) {} }
alias Identity<T> = T;
alias Values<T> = T[];
alias Lookup<K, V> = Map<K, V>;
alias Transform<T, U> = (value: T) => U;
alias BoxRef<T> = Box<T>;
alias Names = Values<string>;

function identity<T>(value: Identity<T>): Identity<T> { return Identity<T>(value); }
function convert(values: int[]): Values<int> { return Values<int>(values); }
function count(values: Values<string>): int { return len(values); }
function lookup(values: Lookup<string, int>, key: string): int { return values[key]; }
function apply(transform: Transform<int, string>, value: int): string { return transform(value); }
function unbox(box: BoxRef<string>): string { return box.value; }
function first(values: Names): string { return values[0]; }
`))

	for _, unexpected := range []string{
		"type Identity[",
		"type Values[",
		"type Lookup[",
		"type Transform[",
		"type BoxRef[",
	} {
		if strings.Contains(generated, unexpected) {
			t.Errorf("generated Go unexpectedly contains %q:\n%s", unexpected, generated)
		}
	}
	for _, expected := range []string{
		"type Names = []string",
		"func identity[T any](value T) T",
		"return T(value)",
		"func convert(values []int) []int",
		"return []int(values)",
		"func count(values []string) int",
		"func lookup(values map[string]int, key string) int",
		"func apply(transform func(int) string, value int) string",
		"func unbox(box *Box[string]) string",
		"func first(values Names) string",
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}
}

func TestNestedGenericAliasesExpandRecursively(t *testing.T) {
	generated := string(generateCheckedSource(t, `
alias Values<T> = T[];
alias Rows<T> = Values<T>[];
alias Matrix<T> = Rows<T>[];
function diagonal(values: Matrix<int>): int { return values[0][0][0]; }
`))
	if strings.Contains(generated, "type Values[") || strings.Contains(generated, "type Rows[") || strings.Contains(generated, "type Matrix[") {
		t.Fatalf("generated Go retained a generic alias declaration:\n%s", generated)
	}
	if !strings.Contains(generated, "func diagonal(values [][][]int) int") {
		t.Fatalf("generated Go did not recursively expand aliases:\n%s", generated)
	}
}
