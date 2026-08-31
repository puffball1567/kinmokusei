package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExceptionPublicGoAPIMatchesIndependentPackage(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "exceptions.otm")
	input := `
import go errors from "errors";

class RemoteException extends Exception {
  constructor(message: string) { super(message); }
}

function ThrowRemote(message: string): void {
  throw new RemoteException(message);
}

function RethrowRemote(message: string): void {
  try {
    ThrowRemote(message);
  } catch (_: RemoteException) {
    throw;
  }
}

function ThrowGeneric(message: string): void {
  throw errors.New(message);
}

function ThrowNil(): void {
  throw nil;
}

function RawPanic(): int {
  const values: int[] = [];
  return values[0];
}
`
	if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "exceptions")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v\n%s", err, diagnostics, generated)
	}
	for _, expected := range []string{
		"type Exception struct", "func NewException", "func (this *Exception) Error() string",
		"type RemoteException struct", "func NewRemoteException", "func UpcastRemoteExceptionToException",
		"func ThrowRemote", "func RethrowRemote", "func ThrowGeneric", "func ThrowNil", "func RawPanic",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated public exception API does not contain %q:\n%s", expected, generated)
		}
	}
	for _, forbidden := range []string{temporary, source, "github.com/puffball1567/onsentamago"} {
		if strings.Contains(string(generated), forbidden) {
			t.Errorf("publishable generated Go contains local/compiler-only path %q:\n%s", forbidden, generated)
		}
	}

	referenceSource := `package reference

import "errors"

type exception interface { OnsenTamagoExceptionError() error }
type thrown struct { err error }
func (value thrown) OnsenTamagoExceptionError() error { return value.err }
func throw(err error) { panic(thrown{err: err}) }

type Exception struct { Message string }
func NewException(message string) *Exception { return &Exception{Message: message} }
func (value *Exception) Error() string { return value.Message }

type RemoteException struct { Exception }
func NewRemoteException(message string) *RemoteException { return &RemoteException{Exception: Exception{Message: message}} }
func UpcastRemoteExceptionToException(value *RemoteException) *Exception { if value == nil { return nil }; return &value.Exception }

func ThrowRemote(message string) { throw(NewRemoteException(message)) }
func RethrowRemote(message string) {
  defer func() {
    recovered := recover()
    if _, ok := recovered.(exception); !ok { panic(recovered) }
    panic(recovered)
  }()
  ThrowRemote(message)
}
func ThrowGeneric(message string) { throw(errors.New(message)) }
func ThrowNil() { throw(nil) }
func RawPanic() int { values := []int{}; return values[0] }
`

	testSource := `package exceptions_test

import (
  "sync"
  "testing"
  exceptions "example.com/exceptions"
  reference "example.com/exceptions/reference"
)

type exceptionMarker interface { OnsenTamagoExceptionError() error }
type classifier func(error) (string, string)

func observe(call func(), classify classifier) (kind, message string) {
  kind = "normal"
  defer func() {
    recovered := recover()
    if recovered == nil { return }
    marked, ok := recovered.(exceptionMarker)
    if !ok { kind = "panic"; message = ""; return }
    kind, message = classify(marked.OnsenTamagoExceptionError())
  }()
  call()
  return kind, message
}

func classifyGenerated(err error) (string, string) {
  if err == nil { return "nil", "" }
  if value, ok := err.(*exceptions.RemoteException); ok { return "remote", value.Message }
  return "error", err.Error()
}
func classifyReference(err error) (string, string) {
  if err == nil { return "nil", "" }
  if value, ok := err.(*reference.RemoteException); ok { return "remote", value.Message }
  return "error", err.Error()
}

func TestPublicExceptionAPI(t *testing.T) {
  generatedRemote := exceptions.NewRemoteException("created")
  referenceRemote := reference.NewRemoteException("created")
  if got, want := generatedRemote.Error(), referenceRemote.Error(); got != want { t.Errorf("remote Error = %q, Go = %q", got, want) }
  if got, want := generatedRemote.Message, referenceRemote.Message; got != want { t.Errorf("remote Message = %q, Go = %q", got, want) }
  generatedBase := exceptions.UpcastRemoteExceptionToException(generatedRemote)
  referenceBase := reference.UpcastRemoteExceptionToException(referenceRemote)
  if got, want := generatedBase.Error(), referenceBase.Error(); got != want { t.Errorf("base Error = %q, Go = %q", got, want) }
  if exceptions.UpcastRemoteExceptionToException(nil) != nil || reference.UpcastRemoteExceptionToException(nil) != nil { t.Error("nil upcast was not preserved") }

  cases := []struct { name string; generated, expected func() }{
    {"remote", func() { exceptions.ThrowRemote("remote") }, func() { reference.ThrowRemote("remote") }},
    {"rethrow", func() { exceptions.RethrowRemote("again") }, func() { reference.RethrowRemote("again") }},
    {"generic", func() { exceptions.ThrowGeneric("generic") }, func() { reference.ThrowGeneric("generic") }},
    {"nil", exceptions.ThrowNil, reference.ThrowNil},
    {"raw panic", func() { _ = exceptions.RawPanic() }, func() { _ = reference.RawPanic() }},
  }
  for _, test := range cases {
    gotKind, gotMessage := observe(test.generated, classifyGenerated)
    wantKind, wantMessage := observe(test.expected, classifyReference)
    if gotKind != wantKind || gotMessage != wantMessage { t.Errorf("%s = (%q, %q), Go = (%q, %q)", test.name, gotKind, gotMessage, wantKind, wantMessage) }
  }
}

func TestPublicExceptionAPIConcurrent(t *testing.T) {
  const workers = 64
  var wait sync.WaitGroup
  failures := make(chan string, workers)
  for index := 0; index < workers; index++ {
    wait.Add(1)
    go func() {
      defer wait.Done()
      gotKind, gotMessage := observe(func() { exceptions.RethrowRemote("parallel") }, classifyGenerated)
      wantKind, wantMessage := observe(func() { reference.RethrowRemote("parallel") }, classifyReference)
      if gotKind != wantKind || gotMessage != wantMessage { failures <- gotKind + ":" + gotMessage }
    }()
  }
  wait.Wait()
  close(failures)
  for failure := range failures { t.Errorf("concurrent result = %q", failure) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "example.com/exceptions", generated, referenceSource, testSource)
}
