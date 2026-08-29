package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRawGoChannelSelectCompilesAndRuns(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "select.otm")
	input := `
import go time from "time";
import go atomic from "sync/atomic";

function record(counter: *atomic.Int64, digit: int64): void {
  counter.Store(counter.Load() * 10 + digit);
}
function observedChannel(counter: *atomic.Int64, channel: GoChannel<int>, digit: int64): GoChannel<int> {
  record(counter, digit);
  return channel;
}
function observedValue(counter: *atomic.Int64, digit: int64): int {
  record(counter, digit);
  return int(digit);
}
function observedIndex(counter: *atomic.Int64): int {
  counter.Add(1);
  return 0;
}

function selectReceive(): int {
  const channel = goChannel[int](1);
  channel <- 42;
  select {
    case const value = <-channel { return value; }
    default { return -1; }
  }
}
function selectCheckedClosed(): boolean {
  const channel = goChannel[int]();
  closeGoChannel(channel);
  select { case const [value, open] = <-channel { return value === 0 && !open; } }
}
function selectCheckedAssignment(): boolean {
  const channel = goChannel[int]();
  closeGoChannel(channel);
  let value = 1;
  let open = true;
  select { case [value, open] = <-channel { return value === 0 && !open; } }
}
function selectSend(): int {
  const channel = goChannel[int](1);
  select {
    case channel <- 7 { return <-channel; }
    default { return -1; }
  }
}
function selectDefault(): int {
  const channel = goChannel[int]();
  select { case const value = <-channel { return value; } default { return 9; } }
}
function selectIgnoresNil(): int {
  let channel: GoChannel<int> = nil;
  select { case <-channel { return -1; } default { return 11; } }
}
function selectOneReady(): int {
  const left = goChannel[int](1);
  const right = goChannel[int](1);
  left <- 1;
  right <- 2;
  select { case const value = <-left { return value; } case const value = <-right { return value; } }
}
function selectUnusedBinding(): int {
  const channel = goChannel[int]();
  closeGoChannel(channel);
  select { case const unused = <-channel { return 1; } }
}
function selectBreak(): int {
  let result = 0;
  select { default { result = 13; break; } }
  return result;
}
function selectContinue(): int {
  let count = 0;
  for (let index = 0; index < 3; index = index + 1) {
    select { default { count = count + 1; continue; } }
    count = 100;
  }
  return count;
}
function selectTimer(): boolean {
  select { case const instant = <-time.After(0) { return !instant.IsZero(); } }
}
function selectSendEvaluation(): int64 {
  let channel: GoChannel<int> = nil;
  let counter: atomic.Int64 = atomic.Int64{};
  select { case observedChannel(&counter, channel, 1) <- observedValue(&counter, 2) {} default {} }
  return counter.Load();
}
function selectReceiveEvaluationOrder(): int64 {
  let channel: GoChannel<int> = nil;
  let counter: atomic.Int64 = atomic.Int64{};
  select {
    case <-observedChannel(&counter, channel, 1) {}
    case <-observedChannel(&counter, channel, 2) {}
    default {}
  }
  return counter.Load();
}
function selectReceiveTargetIsLazy(): int64 {
  let channel: GoChannel<int> = nil;
  let counter: atomic.Int64 = atomic.Int64{};
  let values = [0];
  select { case values[observedIndex(&counter)] = <-channel {} default {} }
  return counter.Load();
}
function selectSendClosed(): void {
  const channel = goChannel[int](1);
  closeGoChannel(channel);
  select { case channel <- 1 {} }
}
function selectForever(): void { select {} }
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "selection")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{
		"case value := <-channel:",
		"case value, open := <-channel:",
		"case value, open = <-channel:",
		"case channel <- 7:",
		"case <-channel:",
		"default:",
		"select {}",
		"case instant := <-time.After(0):",
		"case observedChannel(&counter, channel, 1) <- observedValue(&counter, 2):",
		"case values[observedIndex(&counter)] = <-channel:",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "sync/atomic"
  "time"
)
func record(counter *atomic.Int64, digit int64) {
  counter.Store(counter.Load() * 10 + digit)
}
func observedChannel(counter *atomic.Int64, channel chan int, digit int64) chan int {
  record(counter, digit)
  return channel
}
func observedValue(counter *atomic.Int64, digit int64) int {
  record(counter, digit)
  return int(digit)
}
func observedIndex(counter *atomic.Int64) int {
  counter.Add(1)
  return 0
}
func SelectReceive() int {
  channel := make(chan int, 1)
  channel <- 42
  select {
  case value := <-channel:
    return value
  default:
    return -1
  }
}
func SelectCheckedClosed() bool {
  channel := make(chan int)
  close(channel)
  select {
  case value, open := <-channel:
    return value == 0 && !open
  }
}
func SelectCheckedAssignment() bool {
  channel := make(chan int)
  close(channel)
  value, open := 1, true
  select {
  case value, open = <-channel:
    return value == 0 && !open
  }
}
func SelectSend() int {
  channel := make(chan int, 1)
  select {
  case channel <- 7:
    return <-channel
  default:
    return -1
  }
}
func SelectDefault() int {
  channel := make(chan int)
  select {
  case value := <-channel:
    return value
  default:
    return 9
  }
}
func SelectIgnoresNil() int {
  var channel chan int
  select {
  case <-channel:
    return -1
  default:
    return 11
  }
}
func SelectOneReady() int {
  left := make(chan int, 1)
  right := make(chan int, 1)
  left <- 1
  right <- 2
  select {
  case value := <-left:
    return value
  case value := <-right:
    return value
  }
}
func SelectOneReadyAllows(value int) bool { return value == 1 || value == 2 }
func SelectUnusedBinding() int {
  channel := make(chan int)
  close(channel)
  select {
  case <-channel:
    return 1
  }
}
func SelectBreak() int {
  result := 0
  select {
  default:
    result = 13
    break
  }
  return result
}
func SelectContinue() int {
  count := 0
  for index := 0; index < 3; index++ {
    select {
    default:
      count++
      continue
    }
    count = 100
  }
  return count
}
func SelectTimer() bool {
  select {
  case instant := <-time.After(0):
    return !instant.IsZero()
  }
}
func SelectSendEvaluation() int64 {
  var channel chan int
  var counter atomic.Int64
  select {
  case observedChannel(&counter, channel, 1) <- observedValue(&counter, 2):
  default:
  }
  return counter.Load()
}
func SelectReceiveEvaluationOrder() int64 {
  var channel chan int
  var counter atomic.Int64
  select {
  case <-observedChannel(&counter, channel, 1):
  case <-observedChannel(&counter, channel, 2):
  default:
  }
  return counter.Load()
}
func SelectReceiveTargetIsLazy() int64 {
  var channel chan int
  var counter atomic.Int64
  values := []int{0}
  select {
  case values[observedIndex(&counter)] = <-channel:
  default:
  }
  return counter.Load()
}
func SelectSendClosed() {
  channel := make(chan int, 1)
  close(channel)
  select { case channel <- 1: }
}
func SelectForever() { select {} }
`
	testSource := `package selection
import (
  "testing"
  reference "selection.test/reference"
)
func didPanic(operation func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  operation()
  return false
}
func TestSelect(t *testing.T) {
  if got, want := selectReceive(), reference.SelectReceive(); got != want { t.Errorf("selectReceive = %d, Go = %d", got, want) }
  if got, want := selectCheckedClosed(), reference.SelectCheckedClosed(); got != want { t.Errorf("selectCheckedClosed = %v, Go = %v", got, want) }
  if got, want := selectCheckedAssignment(), reference.SelectCheckedAssignment(); got != want { t.Errorf("selectCheckedAssignment = %v, Go = %v", got, want) }
  if got, want := selectSend(), reference.SelectSend(); got != want { t.Errorf("selectSend = %d, Go = %d", got, want) }
  if got, want := selectDefault(), reference.SelectDefault(); got != want { t.Errorf("selectDefault = %d, Go = %d", got, want) }
  if got, want := selectIgnoresNil(), reference.SelectIgnoresNil(); got != want { t.Errorf("selectIgnoresNil = %d, Go = %d", got, want) }
  if got := selectOneReady(); !reference.SelectOneReadyAllows(got) { t.Errorf("selectOneReady = %d, outside Go outcome set", got) }
  if got := reference.SelectOneReady(); !reference.SelectOneReadyAllows(got) { t.Fatalf("invalid handwritten Go select outcome %d", got) }
  if got, want := selectUnusedBinding(), reference.SelectUnusedBinding(); got != want { t.Errorf("selectUnusedBinding = %d, Go = %d", got, want) }
  if got, want := selectBreak(), reference.SelectBreak(); got != want { t.Errorf("selectBreak = %d, Go = %d", got, want) }
  if got, want := selectContinue(), reference.SelectContinue(); got != want { t.Errorf("selectContinue = %d, Go = %d", got, want) }
  if got, want := selectTimer(), reference.SelectTimer(); got != want { t.Errorf("selectTimer = %v, Go = %v", got, want) }
  if got, want := selectSendEvaluation(), reference.SelectSendEvaluation(); got != want { t.Errorf("selectSendEvaluation = %d, Go = %d", got, want) }
  if got, want := selectReceiveEvaluationOrder(), reference.SelectReceiveEvaluationOrder(); got != want { t.Errorf("selectReceiveEvaluationOrder = %d, Go = %d", got, want) }
  if got, want := selectReceiveTargetIsLazy(), reference.SelectReceiveTargetIsLazy(); got != want { t.Errorf("selectReceiveTargetIsLazy = %d, Go = %d", got, want) }
  if got, want := didPanic(selectSendClosed), didPanic(reference.SelectSendClosed); got != want { t.Errorf("selectSendClosed panic = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "selection.test", generated, referenceSource, testSource)
}
