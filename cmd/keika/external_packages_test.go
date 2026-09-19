package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestExternalPackageCLIWorkflow(t *testing.T) {
	for _, alias := range []string{"", "command", "kinmokusei-cli", "fmt"} {
		name := alias
		if name == "" {
			name = "canonical"
		}
		t.Run(name, func(t *testing.T) {
			testExternalPackageCLIWorkflow(t, alias)
		})
	}
}

func testExternalPackageCLIWorkflow(t *testing.T, alias string) {
	base := t.TempDir()
	root := filepath.Join(base, "app")
	files := map[string]string{
		"app/kinmokusei.toml":              "[project]\nname = \"app\"\nversion = \"0.1.0\"\ngo-module = \"app.test/main\"\ngo-version = \"1.23\"\n[go.dependencies]\n\"go.test/helper\" = \"v0.0.0\"\n[go.replacements]\n\"go.test/helper\" = \"./helper\"\n",
		"app/helper/go.mod":                "module go.test/helper\n\ngo 1.23\n",
		"app/helper/helper.go":             "package helper\nfunc Message()string{return \"external command works\"}\n",
		"app/main.km":                      `import {Command} from "pkg.test/command";import go {Println} from "fmt";function main():void{Println(new Command().run());}`,
		"command/kinmokusei.toml":          "[project]\nname = \"command\"\nversion = \"0.1.0\"\ngo-module = \"pkg.test/command\"\ngo-version = \"1.23\"\n[package]\nentry = \"index.km\"\nmin-kinmokusei = \"0.4.0\"\nbackend = \"go\"\nlicense = \"MIT\"\n[go.dependencies]\n\"go.test/helper\" = \"v0.0.0\"\n",
		"command/index.km":                 `import go helper from "go.test/helper";export class Command{public function run():string{return helper.Message();}}`,
		"command/examples/unconfigured.km": `import go sample from "missing.test/sample";`,
	}
	if alias != "" {
		files["app/main.km"] = strings.Replace(files["app/main.km"], "pkg.test/command", alias, 1)
	}
	for name, contents := range files {
		file := filepath.Join(base, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	}()
	t.Setenv("GOPROXY", "off")
	add := []string{"deps", "add", "--offline", "--replace", "../command"}
	if alias != "" && alias != "command" {
		add = append(add, "--alias", alias)
	}
	add = append(add, "pkg.test/command@v0.1.0")
	for _, args := range [][]string{
		add,
		{"check"}, {"deps", "check"},
	} {
		status, out, stderr := captureRun(t, args...)
		if status != 0 || out != "" || stderr != "" {
			t.Fatalf("%v: status=%d out=%s err=%s", args, status, out, stderr)
		}
	}
	manifest, err := project.ReadManifest(root)
	if alias == "" {
		alias = "command"
	}
	if err != nil || len(manifest.Imports) != 1 || manifest.Imports[alias] != "pkg.test/command" {
		t.Fatalf("automatic alias=%v err=%v", manifest.Imports, err)
	}
	manifestBefore, err := os.ReadFile("kinmokusei.toml")
	if err != nil {
		t.Fatal(err)
	}
	lock, err := os.ReadFile("kinmokusei.lock")
	if err != nil {
		t.Fatal(err)
	}
	binary := "app.out"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	for _, args := range [][]string{{"run"}, {"emit-go"}, {"deps", "list"}, {"build", "-o", binary}} {
		status, out, stderr := captureRun(t, args...)
		if status != 0 || stderr != "" {
			t.Fatalf("%v: status=%d out=%s err=%s", args, status, out, stderr)
		}
		if args[0] == "run" && strings.TrimSpace(out) != "external command works" {
			t.Fatalf("run=%q", out)
		}
		if args[0] == "deps" && !strings.Contains(out, "kinmokusei\tpkg.test/command\tv0.1.0") {
			t.Fatalf("list=%s", out)
		}
	}
	output, err := exec.Command(filepath.Join(root, binary)).CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "external command works" {
		t.Fatalf("binary=%s err=%v", output, err)
	}
	if err := os.Remove(filepath.Join(root, ".kinmokusei", "deps", "go.mod")); err != nil {
		t.Fatal(err)
	}
	if status, out, stderr := captureRun(t, "deps", "fetch", "--offline"); status != 0 {
		t.Fatalf("fetch %d %s %s", status, out, stderr)
	}
	after, err := os.ReadFile("kinmokusei.lock")
	if err != nil || string(lock) != string(after) {
		t.Fatal("normal commands/fetch changed lock", err)
	}
	manifestAfter, err := os.ReadFile("kinmokusei.toml")
	if err != nil || string(manifestBefore) != string(manifestAfter) {
		t.Fatal("normal commands/fetch changed manifest", err)
	}
	if status, _, stderr := captureRun(t, "deps", "remove", "--offline", "pkg.test/command"); status != 0 {
		t.Fatalf("remove=%s", stderr)
	}
	if status, _, stderr := captureRun(t, "check"); status == 0 || !strings.Contains(stderr, "not declared") {
		t.Fatalf("undeclared import: status=%d %s", status, stderr)
	}
}

func TestExternalPackageDefaultSources(t *testing.T) {
	root := t.TempDir()
	contents := fmt.Sprintf("[project]\nname = \"lib\"\nversion = \"0.1.0\"\ngo-module = \"pkg.test/lib\"\ngo-version = \"1.23\"\n[package]\nentry = \"src/index.km\"\nmin-kinmokusei = \"0.4.0\"\nbackend = \"go\"\nlicense = \"MIT\"\n")
	if err := os.WriteFile(filepath.Join(root, "kinmokusei.toml"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	root, err = os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sources, err := projectSources(nil)
	if err != nil || len(sources) != 1 || sources[0] != filepath.Join(root, "src", "index.km") {
		t.Fatalf("sources=%v err=%v", sources, err)
	}
}
