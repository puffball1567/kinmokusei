package sema

import (
	"strings"
	"testing"
)

func TestTypedExceptionSemanticMatrix(t *testing.T) {
	diagnostics := checkSource(t, `
import go errors from "errors";

function run(shouldThrow: boolean): string {
  let outcome = "normal";
  try {
    if (shouldThrow) { throw errors.New("boom"); }
    outcome = "success";
  } catch (err: error) {
    outcome = err.Error();
  } finally {
    outcome += ":finally";
  }
  return outcome;
}

function finallyOnly(err: error): void {
  try { throw err; } finally { const marker = 1; }
}

function nestedLoop(err: error): int {
  let value = 0;
  try {
    while (value < 3) {
      value++;
      if (value == 2) { continue; }
      if (value == 3) { break; }
    }
  } catch (_: error) {
    throw;
  }
  return value;
}

function arrowBoundary(): int {
  try {
    const callback = (): int => { return 42; };
    returnValue(callback());
  } finally {}
  return 42;
}

function returnValue(value: int): void {}
function terminal(err: error): int { throw err; }
function terminalTry(err: error): int { try { throw err; } finally {} }

class NotFoundException extends Exception {
  constructor(message: string) { super(message); }
}
function typedReturns(kind: int): string {
  try {
    if (kind == 0) { return "ok"; }
    if (kind == 1) { throw new NotFoundException("missing"); }
    throw new Exception("other");
  } catch (err: NotFoundException) {
    return "not-found:" + err.message;
  } catch (err: Exception) {
    return "exception:" + err.message;
  } finally {
    returnValue(kind);
  }
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
}

func TestRejectsInvalidTypedExceptionUses(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{"throw non-error", `function bad(): void { throw "boom"; }`, "cannot use string as error"},
		{"bare throw outside catch", `function bad(): void { throw; }`, "bare throw may only be used inside a catch block"},
		{"bare throw in try", `function bad(input: error): void { try { throw; } catch (_: error) {} }`, "bare throw may only be used inside a catch block"},
		{"bare throw in finally", `function bad(input: error): void { try { throw input; } finally { throw; } }`, "bare throw may only be used inside a catch block"},
		{"bare throw in nested arrow", `function bad(input: error): void { try { throw input; } catch (_: error) { const nested = (): void => { throw; }; nested(); } }`, "bare throw may only be used inside a catch block"},
		{"catch non-error", `function bad(): void { try {} catch (value: int) {} }`, "catch binding type must implement error"},
		{"catch after error", `function bad(input: error): void { try { throw input; } catch (_: error) {} catch (_: Exception) {} }`, "unreachable because an earlier catch for error"},
		{"derived catch after base", `class Missing extends Exception { constructor(message: string) { super(message); } } function bad(input: error): void { try { throw input; } catch (_: Exception) {} catch (_: Missing) {} }`, "unreachable because an earlier catch for Exception"},
		{"duplicate typed catch", `class Missing extends Exception { constructor(message: string) { super(message); } } function bad(input: error): void { try { throw input; } catch (_: Missing) {} catch (_: Missing) {} }`, "unreachable because an earlier catch for Missing"},
		{"assign catch binding", `function bad(err: error): void { try { throw err; } catch (caught: error) { caught = err; } }`, "cannot assign to const"},
		{"catch binding scope", `function bad(err: error): error { try { throw err; } catch (caught: error) {} return caught; }`, `undefined name "caught"`},
		{"break across try", `function bad(): void { while (true) { try { break; } finally {} } }`, "break may only be used"},
		{"continue across catch", `function bad(err: error): void { while (true) { try { throw err; } catch (_: error) { continue; } } }`, "continue may only be used"},
		{"pending task at throw", `import go errors from "errors"; function work(): int { return 1; } function bad(): void { const task = go work(); throw errors.New("stop"); }`, "must be consumed with await or detach"},
		{"catch does not inherit try-only nullable fact", `class Box { public value: int; constructor(value: int) { this.value = value; } } function bad(input: error): int { let box: Box | null = null; let result = 0; try { box = new Box(1); throw input; } catch (_: error) { result = box.value; } return result; }`, "nullable"},
		{"finally includes exceptional nullable path", `class Box { public value: int; constructor(value: int) { this.value = value; } } function bad(stop: boolean, input: error): int { let box: Box | null = null; let result = 0; try { if (stop) { throw input; } box = new Box(1); } finally { result = box.value; } return result; }`, "nullable"},
		{"exception interface name collision", `function __kinmokuseiException(): void {} function bad(input: error): void { try { throw input; } catch (_: error) {} }`, "reserved by the exception runtime"},
		{"exception value name collision", `function __kinmokuseiThrown(): void {} function bad(input: error): void { throw input; }`, "reserved by the exception runtime"},
		{"catch built-in name", `function bad(input: error): void { try { throw input; } catch (error: error) {} }`, "conflicts with a built-in type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, test.want) {
				t.Fatalf("diagnostics = %v, want substring %q", diagnostics, test.want)
			}
		})
	}
}
