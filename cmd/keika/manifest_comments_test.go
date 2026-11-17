package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDependencyCLIManifestComments(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	input := `# Owner: platform team
[project] # project settings
name="app" # display name
version="0.1.0"
go-module="app.test/main"
go-version="1.23"

[go.dependencies] # direct modules
# Keep version explanations here

[go.replacements] # local development
# End of manifest`
	path := filepath.Join(root, "kinmokusei.toml")
	for file, contents := range map[string]string{
		path:                                input,
		filepath.Join(library, "go.mod"):    "module pkg.test/native\n\ngo 1.23\n",
		filepath.Join(library, "native.go"): "package native\nfunc Value() int { return 42 }\n",
		filepath.Join(library, "LICENSE"):   "MIT fixture",
	} {
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	command := func(args ...string) {
		t.Helper()
		if status, out, stderr := captureRun(t, append(args, root)...); status != 0 {
			t.Fatalf("%v: status=%d out=%s err=%s", args, status, out, stderr)
		}
	}
	read := func() string {
		t.Helper()
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(input, "\n") {
			if !strings.Contains(string(contents), line) {
				t.Fatalf("CLI edit lost original line %q:\n%s", line, contents)
			}
		}
		return string(contents)
	}
	command("install", "--go-module", "--offline", "--replace", "./library", "pkg.test/native@v0.1.0")
	installed := read()
	installed = strings.Replace(installed, `"pkg.test/native" = "v0.1.0"`, "\"pkg.test/native\" = \"v0.1.0\"\t# installed version", 1)
	installed = strings.Replace(installed, `"pkg.test/native" = "library"`, "\"pkg.test/native\" = \"library\"\t# local path", 1)
	if err := os.WriteFile(path, []byte(installed), 0o644); err != nil {
		t.Fatal(err)
	}
	command("deps", "lock", "--offline")
	command("deps", "update", "--offline", "pkg.test/native@v0.2.0")
	updated := read()
	if !strings.Contains(updated, "\"pkg.test/native\" = \"v0.2.0\"\t# installed version") {
		t.Fatalf("version update lost inline comment: %s", updated)
	}
	command("deps", "remove", "--offline", "pkg.test/native")
	removed := read()
	if strings.Contains(removed, `"pkg.test/native"`) || !strings.Contains(removed, "# installed version") || !strings.Contains(removed, "# local path") {
		t.Fatalf("removal did not preserve inline documentation: %s", removed)
	}
	command("deps", "check")
}
