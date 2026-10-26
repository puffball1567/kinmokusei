package sema

import (
	"strings"
	"testing"
)

func TestNonNullAssertionTypes(t *testing.T) {
	t.Parallel()
	input := `
import go flag from "flag";
class User { public name: string = "name"; }
interface Reader { function read(): int; }
alias Callback = () => boolean;
alias Maybe = User | null;
type Link = distinct *int;
type Chain<T> = distinct T[];
function user(value: Maybe): string { return value!.name; }
function pointer(value: *int | null): int { return *value!; }
function raw(value: *flag.Flag): bstring { return value!.Name; }
function callable(value: Callback | null): boolean { return !value!(); }
function collection(value: int[] | null): int { return value![0]; }
function lookup(value: Map<string, int> | null): int { return value!["x"]; }
function channel(value: GoChannel<int> | null): int { return <-value!; }
function named(value: Link | null): int { return *value!; }
function genericNamed(value: Chain<int> | null): int { return value![0]; }
function contract(value: Reader | null): int { return value!.read(); }
function retained(value: *int | null): int { const checked: *int | null = value!; return *checked; }
function standalone(value: User | null): void { value!; }
function result(value: User | null): Result<User> { return ok(value!); }
constraint Pointer = ~*int;
function constrained<T extends Pointer>(value: T | null): T { return value!; }
`
	if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestRejectsInvalidNonNullAssertions(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"number":                  `function f(): int { return 42!; }`,
		"boolean":                 `function f(value: boolean): boolean { return value!; }`,
		"nil":                     `function f(): void { nil!; }`,
		"null":                    `function f(): void { null!; }`,
		"struct":                  `struct Item { public x: int; } function f(value: Item): Item { return value!; }`,
		"unconstrained parameter": `function f<T>(value: T): T { return value!; }`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := strings.Join(checkSource(t, input), "\n"); !strings.Contains(diagnostics, "postfix ! requires") {
				t.Fatalf("expected assertion diagnostic, got %s", diagnostics)
			}
		})
	}
}

func TestNonNullAssertionDoesNotEraseResultOrTask(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"Result":                `function load(): Result<*int> { let n=1; return ok(&n); } function f(): void { const value=load()!; }`,
		"multiple results":      `function load(): (*int, boolean) { let n=1; return &n,true; } function f(): void { const value=load()!; }`,
		"Task":                  `function load(): int { return 1; } function f(): void { const task=go load(); task!; detach task; }`,
		"no original narrowing": `class User {public name:string="name";} function f(value:User|null):string{value!;return value.name;}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diagnostics := checkSource(t, input); len(diagnostics) == 0 {
				t.Fatal("unsafe assertion accepted")
			}
		})
	}
}

func TestUnsafePointerRetainsNullableState(t *testing.T) {
	t.Parallel()
	input := `import go unsafe from "unsafe";
function f(raw: unsafe.Pointer): boolean {
 const value: unsafe.Pointer | null = raw;
 if (value === null) { return true; }
 const checked: unsafe.Pointer | null = raw!;
 return checked === nil;
}`
	if diagnostics := checkSourceWithPolicy(t, input, GoInteropPolicy{AllowUnsafe: true}); len(diagnostics) != 0 {
		t.Fatalf("unsafe nullable flow: %v", diagnostics)
	}
}
