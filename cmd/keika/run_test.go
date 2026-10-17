package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
)

func TestRunPreservesProgramExitStatus(t *testing.T) {
	// captureRun changes process streams, so these cases must remain serial.
	root := t.TempDir()
	runTemporary := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, runTemporary)
	}
	for _, code := range []int{0, 3, 42, 125, 255} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			source := filepath.Join(root, "main.km")
			input := fmt.Sprintf(`import go fmt from "fmt";
import go os from "os";
function main():void {
 fmt.Println("program stdout");
 fmt.Fprintln(os.Stderr, "program stderr");
 os.Exit(%d);
}`, code)
			if err := os.WriteFile(source, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			status, stdout, stderr := captureRun(t, "run", source)
			if status != code || strings.TrimSpace(stdout) != "program stdout" || strings.TrimSpace(stderr) != "program stderr" {
				t.Fatalf("status=%d stdout=%q stderr=%q; want program exit %d and unmodified streams", status, stdout, stderr, code)
			}
			entries, err := os.ReadDir(runTemporary)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), product.CommandName+"-run-") {
					t.Fatalf("temporary run artifact remains: %s", entry.Name())
				}
			}
		})
	}
}

func TestRunBuildFailureRemainsCommandFailure(t *testing.T) {
	root := t.TempDir()
	runTemporary := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, runTemporary)
	}
	source := filepath.Join(root, "library.km")
	if err := os.WriteFile(source, []byte(`function other():void{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	status, stdout, stderr := captureRun(t, "run", source)
	if status != 1 || stdout != "" || !strings.Contains(stderr, "Go build failed:") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	entries, err := os.ReadDir(runTemporary)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), product.CommandName+"-run-") {
			t.Fatalf("failed build left a temporary executable directory: %s", entry.Name())
		}
	}
}

func TestRunForwardsStandardInput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "main.km")
	if err := os.WriteFile(source, []byte(`import go io from "io";
import go os from "os";
import go fmt from "fmt";
function main():void {
 const [data,err]=io.ReadAll(os.Stdin);
 if(err!==nil){os.Exit(1);return;}
 fmt.Println(len(data));
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(root, "stdin")
	if err := os.WriteFile(inputPath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	previous := os.Stdin
	os.Stdin = input
	defer func() { os.Stdin = previous }()
	status, stdout, stderr := captureRun(t, "run", source)
	if status != 0 || strings.TrimSpace(stdout) != "6" || stderr != "" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
}
