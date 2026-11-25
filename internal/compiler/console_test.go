package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConsoleMatchesIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "console.km")
	input := `import { console, print, println as line } from "kinmokusei/console"

export function WriteExamples(next: (index: int) => int): void {
  console.log("答え", next(1), next(2), true)
  console.log()
  print()
  print("raw=", b"\xff")
  line("!")
  const values: interface{}[] = ["rest", 7, false]
  const log = console.log
  log(values...)
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
