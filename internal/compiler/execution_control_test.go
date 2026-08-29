package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeferAndGoStatementsCompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "execution.otm")
	input := `
import go atomic from "sync/atomic";
import go sync from "sync";

function store(value: *atomic.Int64, input: int64): void { value.Store(input); }
function appendDigit(value: *atomic.Int64, digit: int64): void {
  value.Store(value.Load() * 10 + digit);
}
function scheduleDeferred(value: *atomic.Int64): void {
  defer appendDigit(value, 1);
  defer appendDigit(value, 2);
}
function scheduleCapturedArgument(value: *atomic.Int64): void {
  let input: int64 = 1;
  defer store(value, input);
  input = 2;
}
function worker(value: *atomic.Int64, wait: *sync.WaitGroup): void {
  defer wait.Done();
  value.Add(1);
}
function deferredOrder(): int64 {
  let value = atomic.Int64{};
  scheduleDeferred(&value);
  return value.Load();
}
function deferredArgument(): int64 {
  let value = atomic.Int64{};
  scheduleCapturedArgument(&value);
  return value.Load();
}
function concurrentCall(): int64 {
  let value = atomic.Int64{};
  let wait = sync.WaitGroup{};
  wait.Add(1);
  go worker(&value, &wait);
  wait.Wait();
  return value.Load();
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "execution")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{
		"defer appendDigit(value, 1)",
		"defer appendDigit(value, 2)",
		"defer store(value, input)",
		"defer wait.Done()",
		"go worker(&value, &wait)",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import (
  "sync"
  "sync/atomic"
)
func store(value *atomic.Int64, input int64) { value.Store(input) }
func appendDigit(value *atomic.Int64, digit int64) {
  value.Store(value.Load() * 10 + digit)
}
func scheduleDeferred(value *atomic.Int64) {
  defer appendDigit(value, 1)
  defer appendDigit(value, 2)
}
func scheduleCapturedArgument(value *atomic.Int64) {
  input := int64(1)
  defer store(value, input)
  input = 2
}
func worker(value *atomic.Int64, wait *sync.WaitGroup) {
  defer wait.Done()
  value.Add(1)
}
func DeferredOrder() int64 {
  var value atomic.Int64
  scheduleDeferred(&value)
  return value.Load()
}
func DeferredArgument() int64 {
  var value atomic.Int64
  scheduleCapturedArgument(&value)
  return value.Load()
}
func ConcurrentCall() int64 {
  var value atomic.Int64
  var wait sync.WaitGroup
  wait.Add(1)
  go worker(&value, &wait)
  wait.Wait()
  return value.Load()
}
`
	testSource := `package execution
import (
  "testing"
  reference "execution.test/reference"
)
func TestExecutionControl(t *testing.T) {
  if got, want := deferredOrder(), reference.DeferredOrder(); got != want { t.Errorf("deferredOrder = %d, Go = %d", got, want) }
  if got, want := deferredArgument(), reference.DeferredArgument(); got != want { t.Errorf("deferredArgument = %d, Go = %d", got, want) }
  if got, want := concurrentCall(), reference.ConcurrentCall(); got != want { t.Errorf("concurrentCall = %d, Go = %d", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "execution.test", generated, referenceSource, testSource)
}

func TestRawGoChannelDirectionsSendReceiveAndStandardAPICompileAndRun(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "channels.otm")
	input := `
import go time from "time";

function relay(input: GoReceiveChannel<int>, output: GoSendChannel<int>): void {
  output <- <-input + 1;
}
function roundTrip(channel: GoChannel<int>): int {
  channel <- 42;
  return <-channel;
}
function waitForTimer(): boolean {
  const instant = <-time.After(0);
  return !instant.IsZero();
}
function sendValue(channel: GoSendChannel<int>, value: int): void { channel <- value; }
function internalBuffered(): int {
  const channel = goChannel[int](1);
  channel <- 42;
  const [value, open] = <-channel;
  closeGoChannel(channel);
  if (open) { return value; }
  return -1;
}
function internalUnbuffered(): int {
  const channel = goChannel[int]();
  go sendValue(channel, 7);
  return <-channel;
}
function closedReceive(): boolean {
  const channel = goChannel[int](1);
  closeGoChannel(channel);
  const [value, open] = <-channel;
  return value === 0 && !open;
}
function checkedAssignment(): boolean {
  const channel = goChannel[int](1);
  closeGoChannel(channel);
  let value = 1;
  let open = true;
  [value, open] = <-channel;
  return value === 0 && !open;
}
function durationChannel(): time.Duration {
  const channel = goChannel[time.Duration](1);
  channel <- time.Second;
  return <-channel;
}
function closeTwice(): void {
  const channel = goChannel[int]();
  closeGoChannel(channel);
  closeGoChannel(channel);
}
function rangeSum(): int {
  const channel = goChannel[int](3);
  channel <- 1;
  channel <- 2;
  channel <- 3;
  closeGoChannel(channel);
  let total = 0;
  for (const value of channel) {
    if (value === 2) { continue; }
    total = total + value;
  }
  return total;
}
function mutableRange(): int {
  const channel = goChannel[int](1);
  channel <- 1;
  closeGoChannel(channel);
  let total = 0;
  for (let value: int of channel) {
    value = value + 1;
    total = total + value;
  }
  return total;
}
function emptyRange(): int {
  const channel = goChannel[int]();
  closeGoChannel(channel);
  let count = 0;
  for (const value of channel) { count = count + value; }
  return count;
}
function unusedRange(): void {
  const channel = goChannel[int](1);
  channel <- 1;
  closeGoChannel(channel);
  for (const unused of channel) {}
}
function breakRange(): int {
  const channel = goChannel[int](2);
  channel <- 7;
  channel <- 8;
  closeGoChannel(channel);
  for (const value of channel) { return value; }
  return 0;
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "channels")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, want := range []string{
		"func relay(input <-chan int, output chan<- int)",
		"output <- <-input + 1",
		"func roundTrip(channel chan int) int",
		"var instant = <-time.After(0)",
		"var channel = make(chan int, 1)",
		"var value, open = <-channel",
		"close(channel)",
		"make(chan time.Duration, 1)",
		"for value := range channel",
		"for range channel",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, generated)
		}
	}
	referenceSource := `package reference
import "time"
func Relay(input <-chan int, output chan<- int) { output <- <-input + 1 }
func RoundTrip(channel chan int) int {
  channel <- 42
  return <-channel
}
func WaitForTimer() bool {
  instant := <-time.After(0)
  return !instant.IsZero()
}
func sendValue(channel chan<- int, value int) { channel <- value }
func InternalBuffered() int {
  channel := make(chan int, 1)
  channel <- 42
  value, open := <-channel
  close(channel)
  if open { return value }
  return -1
}
func InternalUnbuffered() int {
  channel := make(chan int)
  go sendValue(channel, 7)
  return <-channel
}
func ClosedReceive() bool {
  channel := make(chan int, 1)
  close(channel)
  value, open := <-channel
  return value == 0 && !open
}
func CheckedAssignment() bool {
  channel := make(chan int, 1)
  close(channel)
  value, open := 1, true
  value, open = <-channel
  return value == 0 && !open
}
func DurationChannel() time.Duration {
  channel := make(chan time.Duration, 1)
  channel <- time.Second
  return <-channel
}
func CloseTwice() {
  channel := make(chan int)
  close(channel)
  close(channel)
}
func RangeSum() int {
  channel := make(chan int, 3)
  channel <- 1
  channel <- 2
  channel <- 3
  close(channel)
  total := 0
  for value := range channel {
    if value == 2 { continue }
    total += value
  }
  return total
}
func MutableRange() int {
  channel := make(chan int, 1)
  channel <- 1
  close(channel)
  total := 0
  for value := range channel {
    value++
    total += value
  }
  return total
}
func EmptyRange() int {
  channel := make(chan int)
  close(channel)
  count := 0
  for value := range channel { count += value }
  return count
}
func UnusedRange() {
  channel := make(chan int, 1)
  channel <- 1
  close(channel)
  for range channel {}
}
func BreakRange() int {
  channel := make(chan int, 2)
  channel <- 7
  channel <- 8
  close(channel)
  for value := range channel { return value }
  return 0
}
`
	testSource := `package channels
import (
  "testing"
  reference "channels.test/reference"
)
func didPanic(operation func()) (panicked bool) {
  defer func() { panicked = recover() != nil }()
  operation()
  return false
}
func TestChannels(t *testing.T) {
  input := make(chan int, 1)
  output := make(chan int, 1)
  input <- 41
  relay(input, output)
  goInput := make(chan int, 1)
  goOutput := make(chan int, 1)
  goInput <- 41
  reference.Relay(goInput, goOutput)
  if got, want := <-output, <-goOutput; got != want { t.Errorf("relay = %d, Go = %d", got, want) }
  bidirectional := make(chan int, 1)
  goBidirectional := make(chan int, 1)
  if got, want := roundTrip(bidirectional), reference.RoundTrip(goBidirectional); got != want { t.Errorf("roundTrip = %d, Go = %d", got, want) }
  if got, want := waitForTimer(), reference.WaitForTimer(); got != want { t.Errorf("waitForTimer = %v, Go = %v", got, want) }
  if got, want := internalBuffered(), reference.InternalBuffered(); got != want { t.Errorf("internalBuffered = %d, Go = %d", got, want) }
  if got, want := internalUnbuffered(), reference.InternalUnbuffered(); got != want { t.Errorf("internalUnbuffered = %d, Go = %d", got, want) }
  if got, want := closedReceive(), reference.ClosedReceive(); got != want { t.Errorf("closedReceive = %v, Go = %v", got, want) }
  if got, want := checkedAssignment(), reference.CheckedAssignment(); got != want { t.Errorf("checkedAssignment = %v, Go = %v", got, want) }
  if got, want := durationChannel(), reference.DurationChannel(); got != want { t.Errorf("durationChannel = %d, Go = %d", got, want) }
  if got, want := rangeSum(), reference.RangeSum(); got != want { t.Errorf("rangeSum = %d, Go = %d", got, want) }
  if got, want := mutableRange(), reference.MutableRange(); got != want { t.Errorf("mutableRange = %d, Go = %d", got, want) }
  if got, want := emptyRange(), reference.EmptyRange(); got != want { t.Errorf("emptyRange = %d, Go = %d", got, want) }
  unusedRange()
  reference.UnusedRange()
  if got, want := breakRange(), reference.BreakRange(); got != want { t.Errorf("breakRange = %d, Go = %d", got, want) }
  if got, want := didPanic(closeTwice), didPanic(reference.CloseTwice); got != want { t.Errorf("closeTwice panic = %v, Go = %v", got, want) }
}
`
	runGeneratedGoDifferentialTest(t, temp, "channels.test", generated, referenceSource, testSource)
}
