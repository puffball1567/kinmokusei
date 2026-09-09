package sema

import (
	"strings"
	"testing"
)

func TestGenericHelperOwnerNameConflicts(t *testing.T) {
	for _, input := range []string{
		`class A<A> {}`,
		`class A { public function keep<A>(value: A): A { return value; } }`,
		`struct A { public function keep<A>(value: A): A { return value; } }`,
		`struct A<A> { public function keep<U>(value: U): U { return value; } }`,
		`constraint Lookup<K extends comparable,V>=Map<K,V>;class A<K extends comparable,V,A extends Lookup<K,A>>{}`,
	} {
		t.Run(input, func(t *testing.T) {
			if diagnostics := strings.Join(checkSource(t, input), "\n"); !strings.Contains(diagnostics, "conflicts with enclosing type A") {
				t.Fatalf("expected owner name conflict, got %s", diagnostics)
			}
		})
	}
	if diagnostics := checkSource(t, `class A<T> {} struct B<T> { public function keep<U>(value: U): U { return value; } }`); len(diagnostics) != 0 {
		t.Fatalf("distinct names must remain valid: %v", diagnostics)
	}
}
