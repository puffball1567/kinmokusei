package compiler

import (
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

struct Report { public names: string[]; public count: int }
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
}
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "consoleexample")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
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
  actual := captureStdout(t, func() {
    g.WriteExamples(func(index int) int {
      actualCalls = append(actualCalls, index)
      return index + 40
    })
  })
  expected := r.WriteExamples(func(index int) int {
    expectedCalls = append(expectedCalls, index)
    return index + 40
  })
  if actual != expected || !slices.Equal(actualCalls, expectedCalls) {
    t.Fatalf("stdout %q, calls %v; want %q, calls %v", actual, actualCalls, expected, expectedCalls)
  }
}

func captureStdout(t *testing.T, run func()) string {
  t.Helper()
  reader, writer, err := os.Pipe()
  if err != nil { t.Fatal(err) }
  defer reader.Close()
  defer writer.Close()
  original := os.Stdout
  os.Stdout = writer
  defer func() { os.Stdout = original }()
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
			function use(values: string[]): void { x.log(values); line(values) }`, ""},
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
