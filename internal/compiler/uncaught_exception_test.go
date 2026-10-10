package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUncaughtExceptionFormattingMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "errors.km")
	// Import the exception definitions so their Go link names differ from the
	// source spelling. Diagnostics must display the original, dynamic class.
	dependency := `export class NotFound extends Exception { constructor(message: string) { super(message); } }
export class Detailed extends NotFound { constructor(message: string) { super(message); } }`
	if err := os.WriteFile(filepath.Join(root, "definitions.km"), []byte(dependency), 0o644); err != nil {
		t.Fatal(err)
	}
	input := `import { NotFound, Detailed } from "./definitions";
import go errors from "errors";
function Missing(): void { throw new NotFound("user 42 was not found"); }
function Child(): void { throw new Detailed("detail"); }
function Rethrow(): void { try { throw new Detailed("again"); } catch (err: Exception) { throw; } }
function Base(): void { throw new Exception("base"); }
function Plain(): void { throw errors.New("plain"); }
function Nil(): void { let value: error = nil; throw value; }
function Message(): bstring { return new NotFound("message").error(); }
function Caught(): bstring { try { Missing(); } catch (err: NotFound) { return err.message; } return "unreachable"; }
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "exceptionformat")
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
import "errors"
type Exception struct{ Name, Message string }
func (value *Exception) Error() string { return value.Message }
type thrown struct{ err error }
func (value thrown) Error() string { if value.err==nil{return "<nil>"}; if ex,ok:=value.err.(*Exception);ok{return ex.Name+": "+ex.Error()};return value.err.Error() }
func Missing(){panic(thrown{&Exception{Name:"NotFound",Message:"user 42 was not found"}})}
func Child(){panic(thrown{&Exception{Name:"Detailed",Message:"detail"}})}
func Rethrow(){defer func(){if caught:=recover();caught!=nil{panic(caught)}}();panic(thrown{&Exception{Name:"Detailed",Message:"again"}})}
func Base(){panic(thrown{&Exception{Name:"Exception",Message:"base"}})}
func Plain(){panic(thrown{errors.New("plain")})}
func Nil(){panic(thrown{nil})}
func Message()string{return (&Exception{Name:"NotFound",Message:"message"}).Error()}
func Caught()(message string){defer func(){if caught:=recover();caught!=nil{message=caught.(thrown).err.(*Exception).Message}}();Missing();return "unreachable"}
`
	tests := `package exceptionformat
import ("testing";"fmt";"exceptionformatfixture/reference")
func observe(call func())(caught bool,message string){defer func(){if value:=recover();value!=nil{caught=true;message=fmt.Sprint(value)}}();call();return}
func TestFormatting(t *testing.T){
 for _,pair:=range []struct{name string;got,want func()}{
 {"missing",Missing,reference.Missing},{"child",Child,reference.Child},{"rethrow",Rethrow,reference.Rethrow},
 {"base",Base,reference.Base},{"plain",Plain,reference.Plain},{"nil",Nil,reference.Nil},
 }{t.Run(pair.name,func(t *testing.T){gp,gm:=observe(pair.got);wp,wm:=observe(pair.want);if gp!=wp||gm!=wm{t.Fatalf("panic=(%v,%q),want=(%v,%q)",gp,gm,wp,wm)}})}
 if got,want:=Message(),reference.Message();got!=want{t.Fatalf("message=%q,want=%q",got,want)}
 if got,want:=Caught(),reference.Caught();got!=want{t.Fatalf("caught=%q,want=%q",got,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "exceptionformatfixture", generated, reference, tests)
}

func TestUncaughtExceptionExecutableOutput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "main.km")
	input := `class NotFound extends Exception { constructor(message: string) { super(message); } }
function main(): void { throw new NotFound("user 42 was not found"); }`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	directory, diagnostics, err := WriteGeneratedModule([]string{path}, "main")
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	binary := filepath.Join(root, "panic-app")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-buildvcs=false", "-p=2", "-o", binary, ".")
	build.Dir = directory
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	output, err := exec.Command(binary).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "panic: NotFound: user 42 was not found") {
		t.Fatalf("uncaught output: %v\n%s", err, output)
	}
	if strings.Contains(string(output), "panic: (main.__kinmokuseiThrown)") {
		t.Fatalf("internal wrapper printed:\n%s", output)
	}
}
