package sema

import (
	"strings"
	"testing"
)

func TestConstructorArrowReturns(t *testing.T) {
	diagnostics := checkSource(t, `
class Holder {
  private callback: () => int;
  constructor(value: int) {
    const nested = (): int => {
      const inner = (): int => { return value + 1; };
      return inner();
    };
    const ignore = (): void => { return; };
    const [parsed, failure] = ((): Result<int> => { return ok(nested()); })();
    ignore();
    this.callback = nested;
  }
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestConstructorArrowKeepsCallableBoundaries(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{`class Holder { constructor() { const callback = (): void => { return; }; callback(); return; } }`, "constructors cannot return early"},
		{`class Base {} class Holder extends Base { constructor() { const callback = (): void => { super(); }; callback(); } }`, "super(...) may only be called from a derived-class constructor"},
		{`class User {} class Holder { private user: User; constructor() { const initialize = (): void => { this.user = new User(); return; }; initialize(); } }`, "every constructor path"},
	} {
		t.Run(test.want, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
