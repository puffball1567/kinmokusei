package selfupdate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUseManagerSurvivesLegacyCompilerInRealProcess(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	root := t.TempDir()
	// Spaces, quotes and Unicode must remain paths, not shell syntax.
	bin := filepath.Join(root, "quoted ' tools 金木犀")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(bin, "keika")
	if output, err := exec.Command("go", "build", "-o", launcher, "../../cmd/keika").CombinedOutput(); err != nil {
		t.Fatalf("compiler build: %v %s", err, output)
	}
	source := filepath.Join(root, "legacy.go")
	if err := os.WriteFile(source, []byte(`package main
import("fmt";"os")
var version string
func main(){if len(os.Args)!=2||os.Args[1]!="version"{fmt.Fprintln(os.Stderr,"legacy compiler has no use command");os.Exit(2)};fmt.Println("keika "+version)}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0.4.8", "0.4.9"} {
		directory := filepath.Join(bin, "keika-releases", version)
		writeFixtureBundle(t, directory)
		// Real released compilers embed their version, independent of whether
		// os.Executable reports an invoked symlink (Darwin) or its target (Linux).
		if output, err := exec.Command("go", "build", "-ldflags=-X main.version=v"+version, "-o", filepath.Join(directory, "keika"), source).CombinedOutput(); err != nil {
			t.Fatalf("legacy build: %v %s", err, output)
		}
		if err := writeReceipt(directory, "v"+version, runtime.GOOS, runtime.GOARCH); err != nil {
			t.Fatal(err)
		}
	}
	invoke := func(command string, args ...string) string {
		t.Helper()
		out, err := exec.CommandContext(context.Background(), command, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v %s", command, args, err, out)
		}
		return string(out)
	}
	if output := invoke(launcher, "use", "--offline", "v0.4.8"); !strings.Contains(output, "Using keika v0.4.8") {
		t.Fatal(output)
	}
	if output := invoke(launcher, "version"); strings.TrimSpace(output) != "keika v0.4.8" {
		t.Fatal(output)
	}
	helper := filepath.Join(bin, "keika-use")
	if output := invoke(helper, "--offline", "v0.4.9"); !strings.Contains(output, "Using keika v0.4.9") {
		t.Fatal(output)
	}
	if output := invoke(launcher, "version"); strings.TrimSpace(output) != "keika v0.4.9" {
		t.Fatal(output)
	}
	if output := invoke(helper, "--offline", "v0.4.8"); !strings.Contains(output, "Using keika v0.4.8") {
		t.Fatal(output)
	}
}
