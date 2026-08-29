package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypedExceptionsMatchIndependentGo(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "exception.otm")
	input := `
import go errors from "errors";

let runtimeCaught = 0;
let runtimeFinalized = 0;
let returnFinalized = 0;

class NotFoundException extends Exception {
  constructor(message: string) { super(message); }
}
class GoneException extends NotFoundException {
  constructor(message: string) { super(message); }
}
class PermissionException extends Exception {
  constructor(message: string) { super(message); }
}

function typedReturn(kind: int): string {
  try {
    if (kind == 0) { return "ok"; }
    if (kind == 1) { throw new NotFoundException("missing"); }
    if (kind == 2) { throw new PermissionException("other"); }
    throw new GoneException("gone");
  } catch (err: NotFoundException) {
    return "not-found:" + err.message;
  } catch (err: Exception) {
    return "exception:" + err.message;
  } finally {
    returnFinalized++;
  }
}

function bareRethrow(): string {
  let result = "start";
  try {
    try {
      throw new GoneException("again");
    } catch (err: NotFoundException) {
      result += ":parent:" + err.message;
      throw;
    } finally {
      result += ":finally";
    }
  } catch (err: GoneException) {
    result += ":gone:" + err.message;
  }
  return result;
}

function finallyOverridesReturn(): string {
  try { return "try"; } finally { return "finally"; }
}

function resultInsideTry(succeed: boolean): Result<int> {
  try {
    if (succeed) { return ok(42); }
    return fail(errors.New("result failure"));
  } finally {
    returnFinalized++;
  }
}

function resetReturnFinalized(): void { returnFinalized = 0; }

function outcome(shouldThrow: boolean): string {
  let result = "try";
  try {
    if (shouldThrow) { throw errors.New("boom"); }
    result += ":success";
  } catch (err: error) {
    result += ":catch:" + err.Error();
  } finally {
    result += ":finally";
  }
  return result;
}

function blankCatch(): string {
  let result = "try";
  try { throw errors.New("ignored"); } catch (_: error) { result += ":caught"; }
  return result;
}

function nestedRethrow(): string {
  let result = "outer";
  try {
    try {
      result += ":inner";
      throw errors.New("first");
    } catch (err: error) {
      result += ":catch:" + err.Error();
      throw errors.New("second");
    } finally {
      result += ":inner-finally";
    }
  } catch (err: error) {
    result += ":outer-catch:" + err.Error();
  } finally {
    result += ":outer-finally";
  }
  return result;
}

function finallyOnly(): string {
  let result = "start";
  try {
    try { throw errors.New("only"); } finally { result += ":finally"; }
  } catch (err: error) {
    result += ":catch:" + err.Error();
  }
  return result;
}

function replacingFinally(): string {
  let result = "";
  try {
    try { throw errors.New("first"); } finally { throw errors.New("replacement"); }
  } catch (err: error) {
    result = err.Error();
  }
  return result;
}

function nilError(): boolean {
  let caughtNil = false;
  try { throw nil; } catch (err: error) { caughtNil = err == nil; }
  return caughtNil;
}

function loopInsideTry(): string {
  let value = 0;
  let result = "";
  try {
    while (value < 5) {
      value++;
      if (value == 2) { continue; }
      if (value == 4) { break; }
      result += string(int32(value + 48));
    }
  } finally { result += ":done"; }
  return result;
}

function terminalThrow(): int {
  try { throw errors.New("terminal"); } finally {}
}

function catchTerminal(): string {
  let result = "";
  try {
    const value = terminalThrow();
    result = string(int32(value));
  } catch (err: error) {
    result = err.Error();
  }
  return result;
}

function catchCallback(callback: () => void): string {
  let result = "normal";
  try {
    callback();
  } catch (err: error) {
    result = err.Error();
  }
  return result;
}

function catchCallbackAsException(callback: () => void): string {
  try {
    callback();
    return "normal";
  } catch (err: Exception) {
    return err.message;
  }
}

function resetRuntimePanicState(): void {
  runtimeCaught = 0;
  runtimeFinalized = 0;
}
function triggerRuntimePanic(): void {
  try {
    const values: int[] = [];
    runtimeCaught += values[0];
  } catch (_: error) {
    runtimeCaught++;
  } finally {
    runtimeFinalized++;
  }
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	crossDirectory := filepath.Join(temp, "cross")
	if err := os.MkdirAll(crossDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	crossSource := `package cross
import "errors"
type exception struct { err error }
func (value exception) OnsenTamagoExceptionError() error { return value.err }
func Throw() { panic(exception{err: errors.New("foreign")}) }
func Panic() { panic("raw") }
`
	if err := os.WriteFile(filepath.Join(crossDirectory, "cross.go"), []byte(crossSource), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "exceptionmatrix")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	text := string(generated)
	for _, want := range []string{
		"type __ontamaThrown struct",
		"type __ontamaException interface",
		"panic(__ontamaThrown{err:",
		"panic(__ontama_thrown_",
		"recover()",
		"recovered_",
		".(__ontamaException)",
		".OnsenTamagoExceptionError()",
		"panic(__ontama_recovered_",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}

	referenceSource := `package reference
import "errors"

type exception interface { OnsenTamagoExceptionError() error }
type thrown struct { err error }
func (value thrown) OnsenTamagoExceptionError() error { return value.err }
func throw(err error) { panic(thrown{err: err}) }
func execute(body func(), catcher func(error), final func()) {
  if final != nil { defer final() }
  if catcher == nil { body(); return }
  caught := false
  var caughtError error
  func() {
    defer func() {
      recovered := recover()
      if recovered == nil { return }
      value, ok := recovered.(exception)
      if !ok { panic(recovered) }
      caught = true
      caughtError = value.OnsenTamagoExceptionError()
    }()
    body()
  }()
  if caught { catcher(caughtError) }
}

func Outcome(shouldThrow bool) string {
  result := "try"
  execute(func() { if shouldThrow { throw(errors.New("boom")) }; result += ":success" }, func(err error) { result += ":catch:" + err.Error() }, func() { result += ":finally" })
  return result
}
func BlankCatch() string { result := "try"; execute(func() { throw(errors.New("ignored")) }, func(error) { result += ":caught" }, nil); return result }
func NestedRethrow() string {
  result := "outer"
  execute(func() {
    execute(func() { result += ":inner"; throw(errors.New("first")) }, func(err error) { result += ":catch:" + err.Error(); throw(errors.New("second")) }, func() { result += ":inner-finally" })
  }, func(err error) { result += ":outer-catch:" + err.Error() }, func() { result += ":outer-finally" })
  return result
}
func FinallyOnly() string {
  result := "start"
  execute(func() { execute(func() { throw(errors.New("only")) }, nil, func() { result += ":finally" }) }, func(err error) { result += ":catch:" + err.Error() }, nil)
  return result
}
func ReplacingFinally() string {
  result := ""
  execute(func() { execute(func() { throw(errors.New("first")) }, nil, func() { throw(errors.New("replacement")) }) }, func(err error) { result = err.Error() }, nil)
  return result
}
func NilError() bool { caughtNil := false; execute(func() { throw(nil) }, func(err error) { caughtNil = err == nil }, nil); return caughtNil }
func LoopInsideTry() string {
  value, result := 0, ""
  execute(func() { for value < 5 { value++; if value == 2 { continue }; if value == 4 { break }; result += string(rune(value + 48)) } }, nil, func() { result += ":done" })
  return result
}
func TerminalThrow() int { execute(func() { throw(errors.New("terminal")) }, nil, func() {}); panic("unreachable") }
func CatchTerminal() string { result := ""; execute(func() { value := TerminalThrow(); result = string(rune(value)) }, func(err error) { result = err.Error() }, nil); return result }
func CatchCallback(callback func()) string { result := "normal"; execute(callback, func(err error) { result = err.Error() }, nil); return result }
func CatchCallbackAsException(callback func()) (result string) {
  caught := false
  execute(callback, func(err error) { caught = true; if err != nil { result = err.Error() } }, nil)
  if !caught { return "normal" }
  return result
}

var returnFinalized int
func TypedReturn(kind int) (result string) {
  defer func() { returnFinalized++ }()
  if kind == 0 { return "ok" }
  if kind == 1 { return "not-found:missing" }
  if kind == 3 { return "not-found:gone" }
  return "exception:other"
}
type goneError struct { message string }
func (value *goneError) Error() string { return value.message }
func BareRethrow() string {
  result := "start"
  execute(func() {
    execute(func() { throw(&goneError{message: "again"}) }, func(err error) {
      result += ":parent:" + err.(*goneError).message
      throw(err)
    }, func() { result += ":finally" })
  }, func(err error) { result += ":gone:" + err.(*goneError).message }, nil)
  return result
}
func FinallyOverridesReturn() (result string) {
  defer func() { result = "finally" }()
  return "try"
}
func ResultInsideTry(succeed bool) (value int, err error) {
  defer func() { returnFinalized++ }()
  if succeed { return 42, nil }
  return 0, errors.New("result failure")
}
func ResetReturnFinalized() { returnFinalized = 0 }
func ReturnFinalized() int { return returnFinalized }

var runtimeCaught int
var runtimeFinalized int
func ResetRuntimePanicState() { runtimeCaught = 0; runtimeFinalized = 0 }
func TriggerRuntimePanic() {
  execute(func() { values := []int{}; runtimeCaught += values[0] }, func(error) { runtimeCaught++ }, func() { runtimeFinalized++ })
}
func RuntimePanicState() (int, int) { return runtimeCaught, runtimeFinalized }
`
	testSource := `package exceptionmatrix
import (
  "fmt"
  "sync"
  "testing"
  cross "exception.test/cross"
  reference "exception.test/reference"
)

func TestExceptionValueMatrix(t *testing.T) {
  for _, value := range []bool{false, true} {
    if got, want := outcome(value), reference.Outcome(value); got != want { t.Errorf("outcome(%v) = %q, Go = %q", value, got, want) }
  }
  comparisons := []struct { name, got, want string }{
    {"blank catch", blankCatch(), reference.BlankCatch()},
    {"nested rethrow", nestedRethrow(), reference.NestedRethrow()},
    {"finally only", finallyOnly(), reference.FinallyOnly()},
    {"replacing finally", replacingFinally(), reference.ReplacingFinally()},
    {"loop", loopInsideTry(), reference.LoopInsideTry()},
    {"terminal", catchTerminal(), reference.CatchTerminal()},
	{"bare rethrow", bareRethrow(), reference.BareRethrow()},
  }
  for _, comparison := range comparisons { if comparison.got != comparison.want { t.Errorf("%s = %q, Go = %q", comparison.name, comparison.got, comparison.want) } }
  if got, want := nilError(), reference.NilError(); got != want { t.Errorf("nil error = %v, Go = %v", got, want) }
  if got, want := catchCallback(cross.Throw), reference.CatchCallback(cross.Throw); got != want { t.Errorf("foreign generated-shape exception = %q, Go = %q", got, want) }
  if got, want := catchCallbackAsException(cross.Throw), reference.CatchCallbackAsException(cross.Throw); got != want { t.Errorf("foreign Exception root = %q, Go = %q", got, want) }
}

func TestTypedCatchReturnAndFinallyMatrix(t *testing.T) {
  resetReturnFinalized()
  reference.ResetReturnFinalized()
  for _, kind := range []int{0, 1, 2, 3} {
    if got, want := typedReturn(kind), reference.TypedReturn(kind); got != want { t.Errorf("typedReturn(%d) = %q, Go = %q", kind, got, want) }
  }
  if got, want := finallyOverridesReturn(), reference.FinallyOverridesReturn(); got != want { t.Errorf("finally override = %q, Go = %q", got, want) }
  for _, succeed := range []bool{false, true} {
    gotValue, gotError := resultInsideTry(succeed)
    wantValue, wantError := reference.ResultInsideTry(succeed)
    if gotValue != wantValue || fmt.Sprint(gotError) != fmt.Sprint(wantError) { t.Errorf("result(%v) = (%d, %v), Go = (%d, %v)", succeed, gotValue, gotError, wantValue, wantError) }
  }
  if got, want := returnFinalized, reference.ReturnFinalized(); got != want { t.Errorf("finally count = %d, Go = %d", got, want) }
}

func observePanic(call func()) (panicked bool, value string) {
  defer func() { if recovered := recover(); recovered != nil { panicked = true; value = fmt.Sprint(recovered) } }()
  call()
  return false, ""
}
func TestRuntimePanicIsNotCaughtAndFinallyRuns(t *testing.T) {
  resetRuntimePanicState()
  reference.ResetRuntimePanicState()
  gotPanic, gotValue := observePanic(triggerRuntimePanic)
  wantPanic, wantValue := observePanic(reference.TriggerRuntimePanic)
  if gotPanic != wantPanic || gotValue != wantValue { t.Errorf("panic = (%v, %q), Go = (%v, %q)", gotPanic, gotValue, wantPanic, wantValue) }
  wantCaught, wantFinalized := reference.RuntimePanicState()
  if runtimeCaught != wantCaught || runtimeFinalized != wantFinalized { t.Errorf("state = (%d, %d), Go = (%d, %d)", runtimeCaught, runtimeFinalized, wantCaught, wantFinalized) }
}

func TestForeignRawPanicIsNotCaught(t *testing.T) {
  gotPanic, gotValue := observePanic(func() { catchCallback(cross.Panic) })
  wantPanic, wantValue := observePanic(func() { reference.CatchCallback(cross.Panic) })
  if gotPanic != wantPanic || gotValue != wantValue { t.Errorf("foreign panic = (%v, %q), Go = (%v, %q)", gotPanic, gotValue, wantPanic, wantValue) }
}

func TestExceptionConcurrentMatrix(t *testing.T) {
  const workers = 64
  var wait sync.WaitGroup
  failures := make(chan string, workers)
  for index := 0; index < workers; index++ {
    wait.Add(1)
    go func(index int) {
      defer wait.Done()
      shouldThrow := index%2 == 0
      if got, want := outcome(shouldThrow), reference.Outcome(shouldThrow); got != want { failures <- fmt.Sprintf("outcome(%d) = %q, Go = %q", index, got, want) }
      if got, want := nestedRethrow(), reference.NestedRethrow(); got != want { failures <- fmt.Sprintf("nested(%d) = %q, Go = %q", index, got, want) }
    }(index)
  }
  wait.Wait()
  close(failures)
  for failure := range failures { t.Error(failure) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "exception.test", generated, referenceSource, testSource)
}
