package compiler

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsoleMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "console.km")
	input := `import { console, print, println as line } from "kinmokusei/console"
import go errors from "errors"

struct Report { public names: string[]; public count: int }
struct BrokenJSON {
  public function MarshalJSON(): Result<byte[]> {
    return fail(errors.New("json: unsupported value: encountered a cycle via fake"))
  }
}
function reportParts(names: string[]): (string[], int) { return names, len(names) }
export function WriteExamples(next: (index: int) => int): void {
  console.log("答え", next(1), next(2), true)
  console.log()
  print()
  print("raw=", b"\xff")
  line("!")
  const values: interface{}[] = ["rest", 7, false]
  const log = console.log
  log(values...)
  const names: string[] = ["Ada", "雪"]
  console.log("names", names)
  const labels = makeMap<int, string>()
  labels[1] = "ready"
  print(labels)
  line(Report{names: names, count: 2})
  console.log(reportParts(names))
  console.log("count: %d", next(3))
  console.log("%s:%d %% %f", "雪", "0x10", "1.25px", "extra")
  console.log("%i %d %f", " -12tail", true, "+.25e+1x")
  console.log("%d %i %f", "oops", false, "Infinity")
  console.log("%d %d", uint64(18446744073709551615), 1e-7)
  console.log("%s %d %%", "missing")
  console.log("%% %s")
  console.log("%% %q", "extra")
  console.log("json: %j", Report{names: names, count: 2})
  const cyclic = makeMap<bstring, interface{}>()
  cyclic[b"self"] = cyclic
  console.log("cycle: %j", cyclic)
  console.log("callback: %j", next)
  console.log("marshal failure: %j", BrokenJSON{})
  console.log("%clabel: %j %s", "color: red", names, "done")
  console.log("%c %j", "ignored")
  console.log("report: %O", Report{names: names, count: 2})
  console.log("cycle: %o", cyclic)
  let literal = "literal %d"
  line(literal, 3)
}
export function WriteInspection(value: interface{}): void {
  console.log("%O | %o", value, value)
}
export function WriteDiagnostics(): void {
  console.warn("warning: %s (%d)", ["ready", "done"], 2)
  console.error("error: %s", "invalid input")
  console.warn("warnings: %j", ["ready", "done"])
  console.error("%cstatus: %j", "color: red", { status: "failed" })
  console.warn("warnings: %o", ["ready", "done"])
  console.error()
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "consoleexample")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	formatted, err := format.Source(generated)
	if err != nil || !bytes.Equal(generated, formatted) {
		t.Fatalf("console output must already be gofmt-formatted: %v", err)
	}
	reference := `package reference

import (
  "bytes"
  "fmt"
)

func WriteExamples(next func(int) int) string {
  var output bytes.Buffer
  fmt.Fprintln(&output, "答え", next(1), next(2), true)
  fmt.Fprintln(&output)
  fmt.Fprint(&output, "raw=", "\xff")
  fmt.Fprintln(&output, "!")
  fmt.Fprintln(&output, "rest", 7, false)
  names := []string{"Ada", "雪"}
  fmt.Fprintln(&output, "names", names)
  fmt.Fprint(&output, map[int]string{1: "ready"})
  fmt.Fprintln(&output, struct { Names []string; Count int }{names, 2})
  fmt.Fprintln(&output, names, len(names))
  fmt.Fprintf(&output, "count: %d\n", next(3))
  fmt.Fprintln(&output, "雪:16 % 1.25 extra")
  fmt.Fprintln(&output, "-12 1 2.5")
  fmt.Fprintln(&output, "NaN NaN Infinity")
  fmt.Fprintln(&output, "18446744073709551615 1e-7")
  output.WriteString("missing %d %\n")
  output.WriteString("%% %s\n")
  output.WriteString("% %q extra\n")
  output.WriteString("json: {\"names\":[\"Ada\",\"雪\"],\"count\":2}\n")
  output.WriteString("cycle: [Circular]\n")
  output.WriteString("callback: [Unserializable]\n")
  output.WriteString("marshal failure: [Unserializable]\n")
  output.WriteString("label: [\"Ada\",\"雪\"] done\n")
  output.WriteString(" %j\n")
  output.WriteString("report: { Names: [ \"Ada\", \"雪\" ], Count: 2 }\n")
  output.WriteString("cycle: Map { \"self\" => [Circular] }\n")
  output.WriteString("literal %d 3\n")
  return output.String()
}

func WriteDiagnostics() string {
  var output bytes.Buffer
  fmt.Fprintf(&output, "warning: %v (%d)\n", []string{"ready", "done"}, 2)
  fmt.Fprintf(&output, "error: %s\n", "invalid input")
  output.WriteString("warnings: [\"ready\",\"done\"]\n")
  output.WriteString("status: {\"status\":\"failed\"}\n")
  output.WriteString("warnings: [ \"ready\", \"done\" ]\n")
  fmt.Fprintln(&output)
  return output.String()
}
`
	comparison := `package consoleexample_test

import (
  "io"
  "os"
  "slices"
  "testing"

  g "console.test"
  r "console.test/reference"
)

func TestConsole(t *testing.T) {
  var actualCalls, expectedCalls []int
  var stderr string
  actual := captureOutput(t, &os.Stdout, func() {
    stderr = captureOutput(t, &os.Stderr, func() {
      g.WriteExamples(func(index int) int {
        actualCalls = append(actualCalls, index)
        return index + 40
      })
    })
  })
  expected := r.WriteExamples(func(index int) int {
    expectedCalls = append(expectedCalls, index)
    return index + 40
  })
  if actual != expected || stderr != "" || !slices.Equal(actualCalls, expectedCalls) {
    t.Fatalf("stdout %q, stderr %q, calls %v. Want stdout %q, empty stderr, calls %v", actual, stderr, actualCalls, expected, expectedCalls)
  }
}

func TestDiagnostics(t *testing.T) {
  var stderr string
  stdout := captureOutput(t, &os.Stdout, func() {
    stderr = captureOutput(t, &os.Stderr, g.WriteDiagnostics)
  })
  if want := r.WriteDiagnostics(); stdout != "" || stderr != want {
    t.Fatalf("stdout %q, stderr %q; want empty stdout and stderr %q", stdout, stderr, want)
  }
}

type node struct {
  Next *node
  hidden string
}

func (node) String() string { panic("inspection called String") }

func TestInspection(t *testing.T) {
  nested := []any{[]any{[]any{[]any{[]any{42}}}}}
  self := &node{hidden: "private"}
  self.Next = self
  cyclic := make([]any, 1)
  cyclic[0] = cyclic
  shared := []int{7}
  for _, tc := range []struct {
    name string
    value any
    compact, detailed string
  }{
    {"depth", nested, "[ [ [ [Array] ] ] ]", "[ [ [ [ [ 42 ] ] ] ] ]"},
    {"pointer cycle", self, "&{ Next: [Circular] }", "&{ Next: [Circular] }"},
    {"slice cycle", cyclic, "[ [Circular] ]", "[ [Circular] ]"},
    {"shared slice", []any{shared, shared}, "[ [ 7 ], [ 7 ] ]", "[ [ 7 ], [ 7 ] ]"},
    {"stable map", map[int]string{2: "second", 1: "first"},
      "Map { 1 => \"first\", 2 => \"second\" }", "Map { 1 => \"first\", 2 => \"second\" }"},
    {"fixed array", [2]bool{true, false}, "[ true, false ]", "[ true, false ]"},
    {"typed nil", (*node)(nil), "null", "null"},
    {"byte text", "\xff", "\"\\xff\"", "\"\\xff\""},
  } {
    t.Run(tc.name, func(t *testing.T) {
      output := captureOutput(t, &os.Stdout, func() { g.WriteInspection(tc.value) })
      if want := tc.compact + " | " + tc.detailed + "\n"; output != want {
        t.Fatalf("got %q; want %q", output, want)
      }
    })
  }
}

func captureOutput(t *testing.T, stream **os.File, run func()) string {
  t.Helper()
  reader, writer, err := os.Pipe()
  if err != nil { t.Fatal(err) }
  defer reader.Close()
  defer writer.Close()
  original := *stream
  *stream = writer
  defer func() { *stream = original }()
  run()
  if err := writer.Close(); err != nil { t.Fatal(err) }
  output, err := io.ReadAll(reader)
  if err != nil { t.Fatal(err) }
  return string(output)
}
`
	runGeneratedGoDifferentialTest(t, root, "console.test", generated, reference, comparison)
}

func TestConsoleBorrowRequiresResolvedStandardDeclaration(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source, diagnostic string
	}{
		{"re-exported aliases", `import { output as x, line } from "./bridge"
			function use(values: string[]): void { x.log(values); x.warn(values); x.error(values); line(values) }`, ""},
		{"multiple values", `import { console } from "kinmokusei/console"
			function values(): (string[], int) { return ["one"], 2 }
			function use(): void { console.log(values()) }`, ""},
		{"generic argument", `import { console } from "kinmokusei/console"
			function show<T>(value: T): void { console.log(value) }
			function use(values: string[]): void { show(values) }`, ""},
		{"user static method", `class Console { public static function log(...values: interface{}[]): void {} }
			function use(values: string[]): void { Console.log(values) }`, "cannot use string[] as interface{}"},
		{"local receiver shadows import", `import { console } from "kinmokusei/console"
			function use(values: string[]): void {
				const console = { log: (...args: interface{}[]): void => {} }
				console.log(values)
			}`, "cannot use string[] as interface{}"},
		{"function value", `import { console } from "kinmokusei/console"
			function use(values: string[]): void { const output = console.log; output(values) }`, "cannot use string[] as interface{}"},
		{"function shadows imported alias", `import { println as output } from "kinmokusei/console"
			function use(values: string[]): void {
				const output = (...args: interface{}[]): void => {}
				output(values)
			}`, "cannot use string[] as interface{}"},
		{"direct Go printer keeps its check", `import go fmt from "fmt"
			function use(values: string[]): void { fmt.Println(values) }`, "cannot use string[] as any"},
		{"nested call keeps its check", `import { console } from "kinmokusei/console"
			import go fmt from "fmt"
			function use(values: string[]): void { console.log(fmt.Sprint(values)) }`, "cannot use string[] as any"},
		{"spread requires matching element type", `import { console } from "kinmokusei/console"
			function use(values: string[]): void { console.log(values...) }`, "cannot use string[]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			entry := filepath.Join(root, "main.km")
			bridge := filepath.Join(root, "bridge.km")
			result, err := CheckFilesWithOverlay([]string{entry}, map[string]string{
				entry:  tc.source,
				bridge: `export { console as output, println as line } from "kinmokusei/console"`,
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.diagnostic == "" {
				if len(result.Diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %v", result.Diagnostics)
				}
				return
			}
			for _, diagnostic := range result.Diagnostics {
				if strings.Contains(diagnostic.Message, tc.diagnostic) {
					return
				}
			}
			t.Fatalf("expected %q, got %v", tc.diagnostic, result.Diagnostics)
		})
	}
}

func TestConsoleBorrowIgnoresUserFileWithVirtualPathSpelling(t *testing.T) {
	t.Parallel()
	path := filepath.FromSlash("@stdlib/kinmokusei/console/console.km")
	input := `function print(...values: interface{}[]): void {}
		function use(values: string[]): void { print(values) }`
	result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, "cannot use string[] as interface{}") {
			return
		}
	}
	t.Fatalf("user file acquired the console argument contract: %v", result.Diagnostics)
}
