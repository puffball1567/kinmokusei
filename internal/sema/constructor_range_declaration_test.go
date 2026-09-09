package sema

import (
	"strings"
	"testing"
)

func TestConstructorRangeProofThroughDeclarations(t *testing.T) {
	for _, body := range []string{
		`if (len(values) > 0) { const count = len(values); let label = "item"; for (const value of values) { this.user = new User(label); } } else { this.user = new User("empty"); }`,
		`if (len(values) === 0) { throw new Exception("empty"); } const count = len(values); const positive = count > 0; for (const value of values) { this.user = new User("item"); }`,
		`if (len(values) > 0) { const count = len(values); if (enabled) { const label = "enabled"; for (const value of values) { this.user = new User(label); } } else { let label = "disabled"; for (const value of values) { this.user = new User(label); } } } else { this.user = new User("empty"); }`,
		`switch (len(values)) { case 0 { this.user = new User("empty"); } default { const count = len(values); for (const value of values) { this.user = new User("item"); } } }`,
		`switch (len(values)) { case 1, 2 { let count = len(values) + 1; for (const value of values) { this.user = new User("item"); } } default { this.user = new User("other"); } }`,
		`if (len(values) > 0 && len(other) > 0) { const count = len(values) + len(other); if (enabled) { for (const value of values) { this.user = new User("left"); } } else { for (const value of other) { this.user = new User("right"); } } } else { this.user = new User("empty"); }`,
		`if (len(values) > 0) { const alias = values; for (const value of values) { this.user = new User("item"); } } else { this.user = new User("empty"); }`,
	} {
		t.Run(body, func(t *testing.T) {
			diagnostics := checkSource(t, `class User { constructor(public name: string) {} } class Holder { private user: User; constructor(values: int[], other: int[], enabled: boolean) { `+body+` } }`)
			if len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}
}

func TestConstructorRangeDeclarationInvalidation(t *testing.T) {
	for _, body := range []string{
		`const count = len(values); values = [];`,
		`const pointer = &values; *pointer = [];`,
		`const count = callback(values);`,
		`const count = 1 + callback(values);`,
		`const keep = enabled && callback(values) > 0;`,
		`const count = len(values); const changed = callback(values);`,
		`const alias = values; const changed = callback(alias);`,
		`const values = other;`,
		`const count = len(values); const values = other;`,
		`const len = callback; const count = len(values);`,
		`const change = (): int => { values = []; return 0; }; const count = change();`,
	} {
		t.Run(body, func(t *testing.T) {
			diagnostics := checkSource(t, `class User {} class Holder { private user: User; constructor(values: int[], other: int[], enabled: boolean, callback: (values: int[]) => int) { if (len(values) > 0) { `+body+` for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`)
			if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "every constructor path") {
				t.Fatalf("diagnostics = %v, want one incomplete-initialization diagnostic", diagnostics)
			}
		})
	}
}

func TestConstructorMapRangeProofInvalidation(t *testing.T) {
	for _, body := range []string{
		`const alias = values; clear(alias); const count = len(values);`,
		`const changed = callback(values);`,
		`const alias = values; const changed = callback(alias);`,
	} {
		diagnostics := checkSource(t, `class User {} class Holder { private user: User; constructor(values: Map<string, int>, callback: (values: Map<string, int>) => int) { if (len(values) > 0) { `+body+` for (const value of values) { this.user = new User(); } } else { this.user = new User(); } } }`)
		if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "every constructor path") {
			t.Fatalf("%s: diagnostics = %v, want one incomplete-initialization diagnostic", body, diagnostics)
		}
	}
}

func TestConstructorDeeplyNestedRangeGuards(t *testing.T) {
	// A linear source tree should not trigger both ordinary and proof-aware
	// analysis recursively at every level.
	body := `for (const value of values) { this.user = new User(); }`
	for range 20 {
		body = `if (len(values) > 0) { ` + body + ` } else { this.user = new User(); }`
	}
	diagnostics := checkSource(t, `class User {} class Holder { private user: User; constructor(values: int[]) { `+body+` } }`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}
