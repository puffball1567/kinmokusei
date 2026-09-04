package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/compiler"
	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/project"
)

func captureRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	outputDirectory := t.TempDir()
	stdoutWriter, err := os.Create(filepath.Join(outputDirectory, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	stderrWriter, err := os.Create(filepath.Join(outputDirectory, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	status := run(args)
	os.Stdout, os.Stderr = oldStdout, oldStderr
	if err = stdoutWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err = stderrWriter.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, err := os.ReadFile(stdoutWriter.Name())
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.ReadFile(stderrWriter.Name())
	if err != nil {
		t.Fatal(err)
	}
	return status, string(stdout), string(stderr)
}

func TestCommandFailureMatrix(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStatus int
		wantError  string
	}{
		{"no command", nil, 2, "usage:"},
		{"unknown command", []string{"unknown"}, 2, "unknown command"},
		{"check missing source", []string{"check"}, 2, "requires at least one"},
		{"build missing source", []string{"build"}, 2, "requires at least one"},
		{"run missing source", []string{"run"}, 2, "requires at least one"},
		{"emit missing source", []string{"emit-go"}, 2, "requires at least one"},
		{"C ABI emit missing output", []string{"emit-c-abi"}, 2, "requires -o"},
		{"C ABI emit missing source", []string{"emit-c-abi", "-o", t.TempDir()}, 2, "requires at least one"},
		{"invalid check flag", []string{"check", "-unknown"}, 2, "flag provided but not defined"},
		{"invalid build flag", []string{"build", "-unknown"}, 2, "flag provided but not defined"},
		{"invalid run flag", []string{"run", "-unknown"}, 2, "flag provided but not defined"},
		{"invalid emit flag", []string{"emit-go", "-unknown"}, 2, "flag provided but not defined"},
		{"invalid C ABI emit flag", []string{"emit-c-abi", "-unknown"}, 2, "flag provided but not defined"},
		{"FFI missing subcommand", []string{"ffi"}, 2, "requires a subcommand"},
		{"FFI unknown subcommand", []string{"ffi", "unknown"}, 2, "unknown ffi subcommand"},
		{"FFI generate missing paths", []string{"ffi", "generate"}, 2, "requires --manifest and -o"},
		{"FFI generate positional argument", []string{"ffi", "generate", "extra"}, 2, "accepts no positional"},
		{"FFI generate invalid flag", []string{"ffi", "generate", "-unknown"}, 2, "flag provided but not defined"},
		{"abi missing subcommand", []string{"abi"}, 2, "requires a subcommand"},
		{"abi unknown subcommand", []string{"abi", "unknown"}, 2, "unknown abi subcommand"},
		{"abi check missing baseline", []string{"abi", "check"}, 2, "requires --baseline"},
		{"abi check missing source", []string{"abi", "check", "--baseline", filepath.Join(t.TempDir(), "baseline.json")}, 2, "requires at least one"},
		{"abi check invalid flag", []string{"abi", "check", "-unknown"}, 2, "flag provided but not defined"},
		{"interop missing subcommand", []string{"interop"}, 2, "requires a subcommand"},
		{"interop unknown subcommand", []string{"interop", "unknown"}, 2, "unknown interop subcommand"},
		{"interop audit missing package", []string{"interop", "audit"}, 2, "requires --stdlib or at least one"},
		{"interop audit invalid flag", []string{"interop", "audit", "-unknown"}, 2, "flag provided but not defined"},
		{"target invalid flag", []string{"target", "-unknown"}, 2, "flag provided but not defined"},
		{"target inaccessible project", []string{"target", filepath.Join(t.TempDir(), "missing")}, 2, "not accessible"},
		{"lsp missing stdio", []string{"lsp"}, 2, "requires --stdio"},
		{"lsp source argument", []string{"lsp", "--stdio", "main.km"}, 2, "accepts no source arguments"},
		{"invalid lsp flag", []string{"lsp", "-unknown"}, 2, "flag provided but not defined"},
		{"install missing kind", []string{"install"}, 2, "requires --go-module"},
		{"install missing module", []string{"install", "--go-module"}, 2, "requires <module>@<version>"},
		{"install malformed module", []string{"install", "--go-module", "example.com/library"}, 2, "<module>@<version>"},
		{"install invalid flag", []string{"install", "-unknown"}, 2, "flag provided but not defined"},
		{"install excess directories", []string{"install", "--go-module", "example.com/library@v1.0.0", ".", "."}, 2, "one optional project directory"},
		{"deps missing subcommand", []string{"deps"}, 2, "requires a subcommand"},
		{"deps unknown subcommand", []string{"deps", "unknown"}, 2, "unknown deps subcommand"},
		{"deps lock invalid flag", []string{"deps", "lock", "-unknown"}, 2, "flag provided but not defined"},
		{"deps check invalid flag", []string{"deps", "check", "-unknown"}, 2, "flag provided but not defined"},
		{"deps add missing dependency", []string{"deps", "add"}, 2, "requires a dependency"},
		{"deps add malformed dependency", []string{"deps", "add", "example.com/library"}, 2, "<module>@<version>"},
		{"deps remove missing dependency", []string{"deps", "remove"}, 2, "requires a dependency"},
		{"deps update missing dependency", []string{"deps", "update"}, 2, "requires a dependency"},
		{"deps update malformed dependency", []string{"deps", "update", "example.com/library"}, 2, "<module>@<version>"},
		{"deps add invalid flag", []string{"deps", "add", "-unknown"}, 2, "flag provided but not defined"},
		{"deps remove invalid flag", []string{"deps", "remove", "-unknown"}, 2, "flag provided but not defined"},
		{"deps update invalid flag", []string{"deps", "update", "-unknown"}, 2, "flag provided but not defined"},
		{"deps licenses invalid flag", []string{"deps", "licenses", "-unknown"}, 2, "flag provided but not defined"},
		{"deps excess directories", []string{"deps", "check", ".", "."}, 2, "at most one project directory"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, _, stderr := captureRun(t, test.args...)
			if status != test.wantStatus || !strings.Contains(stderr, test.wantError) {
				t.Fatalf("status = %d, stderr = %q; want status %d and %q", status, stderr, test.wantStatus, test.wantError)
			}
		})
	}
}

func TestVersionCommand(t *testing.T) {
	previous := product.Version
	product.Version = "v0.2.0-test"
	t.Cleanup(func() { product.Version = previous })
	for _, command := range []string{"version", "--version"} {
		status, stdout, stderr := captureRun(t, command)
		if status != 0 || stdout != product.CommandName+" v0.2.0-test\n" || stderr != "" {
			t.Fatalf("%s: status=%d stdout=%q stderr=%q", command, status, stdout, stderr)
		}
	}
}

func TestInstallGoModuleOfflineAndImport(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "application"
version = "0.1.0"
go-module = "example.com/application"
go-version = "1.23"
`
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/library\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "library.go"), []byte("package library\nfunc Value() int { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "LICENSE"), []byte("fixture license\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, stdout, stderr := captureRun(t, "deps", "lock", "--offline", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("initial lock: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, stdout, stderr := captureRun(t, "install", "--go-module", "--offline", "--replace", "./library", "example.com/library@v0.0.0", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("install: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}

	contents, err := os.ReadFile(filepath.Join(root, product.ProjectFileName))
	if err != nil || !strings.Contains(string(contents), `"example.com/library" = "v0.0.0"`) || !strings.Contains(string(contents), `"example.com/library" = "library"`) {
		t.Fatalf("installed manifest err=%v:\n%s", err, contents)
	}
	for _, path := range []string{filepath.Join(root, product.LockFileName), filepath.Join(product.DependencyDirectory(root), "go.mod"), filepath.Join(product.DependencyDirectory(root), "go.sum")} {
		if info, statErr := os.Stat(path); statErr != nil || info.IsDir() {
			t.Fatalf("dependency output %q: info=%v err=%v", path, info, statErr)
		}
	}

	source := filepath.Join(root, "main.km")
	if err = os.WriteFile(source, []byte(`import go library from "example.com/library"; function value(): int { return library.Value(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, stdout, stderr := captureRun(t, "check", source); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("check installed module: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, stdout, stderr := captureRun(t, "install", "--go-module", "--offline", "--replace", "./library", "example.com/library@v0.0.0", root); status != 1 || stdout != "" || !strings.Contains(stderr, "already exists") {
		t.Fatalf("duplicate install: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, stdout, stderr := captureRun(t, "deps", "check", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("check after rejected duplicate: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
}

func TestInstallGoModuleResolutionFailureRestoresProject(t *testing.T) {
	root := t.TempDir()
	manifest := `[project]
name = "application"
version = "0.1.0"
go-module = "example.com/application"
go-version = "1.23"
`
	manifestPath := filepath.Join(root, product.ProjectFileName)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, stdout, stderr := captureRun(t, "deps", "lock", "--offline", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("initial lock: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	paths := []string{
		manifestPath,
		filepath.Join(root, product.LockFileName),
		filepath.Join(product.DependencyDirectory(root), "go.mod"),
		filepath.Join(product.DependencyDirectory(root), "go.sum"),
	}
	before := make(map[string][]byte, len(paths))
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = contents
	}

	status, stdout, stderr := captureRun(t, "install", "--go-module", "--offline", "example.invalid/unavailable@v1.0.0", root)
	if status != 1 || stdout != "" || !strings.Contains(stderr, "original dependency state restored") {
		t.Fatalf("failed install: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	for _, path := range paths {
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before[path]) {
			t.Fatalf("failed install changed %s:\n--- before ---\n%s\n--- after ---\n%s", filepath.Base(path), before[path], after)
		}
	}
	if status, stdout, stderr = captureRun(t, "deps", "check", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("check after rollback: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
}

func TestIncomingCFFIGenerateCommand(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "binding.json")
	output := filepath.Join(root, "generated")
	contents := `{"schemaVersion":1,"package":"binding","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Value","symbol":"fixture_value","parameters":[],"result":"int32","convention":"direct"}]}`
	if err := os.WriteFile(manifest, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	status, stdout, stderr := captureRun(t, "ffi", "generate", "--manifest", manifest, "-o", output)
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	generated, err := os.ReadFile(filepath.Join(output, "generated_ffi.go"))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(generated); !strings.Contains(text, "package binding") || !strings.Contains(text, "func Value() int32") {
		t.Fatalf("generated C FFI =\n%s", generated)
	}
}

func TestGoInteropAuditCommandMatrix(t *testing.T) {
	status, stdout, stderr := captureRun(t, "interop", "audit", "fmt")
	if status != 0 || stderr != "" || !strings.Contains(stdout, "1/1 packages loaded") || !strings.Contains(stdout, "callables: safe") {
		t.Fatalf("text audit status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}

	status, stdout, stderr = captureRun(t, "interop", "audit", "--json", "fmt", "fmt")
	if status != 0 || stderr != "" {
		t.Fatalf("JSON audit status=%d stderr=%q", status, stderr)
	}
	var report compiler.GoInteropAuditReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON audit: %v\n%s", err, stdout)
	}
	if report.AttemptedPackages != 1 || report.LoadedPackages != 1 || report.Callables.Total == 0 || len(report.FailedPackages) != 0 {
		t.Fatalf("JSON audit report=%#v", report)
	}

	missing := "example.invalid/kinmokusei-audit-missing"
	status, stdout, stderr = captureRun(t, "interop", "audit", missing)
	if status != 1 || !strings.Contains(stdout, "failed packages:") || !strings.Contains(stdout, missing) || !strings.Contains(stderr, "audit incomplete") {
		t.Fatalf("failed audit status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	status, stdout, stderr = captureRun(t, "interop", "audit", "--allow-incomplete", "--json", missing)
	if status != 0 || stderr != "" {
		t.Fatalf("allowed incomplete audit status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || len(report.FailedPackages) != 1 {
		t.Fatalf("allowed incomplete JSON err=%v report=%#v", err, report)
	}
}

func TestDependencyCommandsOfflineMatrix(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "application"
version = "0.1.0"
go-module = "example.com/application"
go-version = "1.23"
`
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/library\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "library.go"), []byte("package library\nfunc Value() int { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "LICENSE"), []byte("fixture license\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, stdout, stderr := captureRun(t, "deps", "lock", "--offline", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("lock: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, stdout, stderr := captureRun(t, "deps", "add", "--offline", "--replace", "./library", "example.com/library@v0.0.0", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("add: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	for _, path := range []string{filepath.Join(root, product.LockFileName), filepath.Join(product.DependencyDirectory(root), "go.mod"), filepath.Join(product.DependencyDirectory(root), "go.sum")} {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			t.Fatalf("dependency output %q: info=%v err=%v", path, info, err)
		}
	}
	if status, stdout, stderr := captureRun(t, "deps", "check", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("check: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, stdout, stderr := captureRun(t, "deps", "update", "--offline", "example.com/library@v0.0.1", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("update: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	updated, err := os.ReadFile(filepath.Join(root, product.ProjectFileName))
	if err != nil || !strings.Contains(string(updated), `"example.com/library" = "v0.0.1"`) || !strings.Contains(string(updated), `"example.com/library" = "library"`) {
		t.Fatalf("updated manifest err=%v:\n%s", err, updated)
	}
	if status, stdout, stderr := captureRun(t, "deps", "check", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("check after update: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	status, stdout, stderr := captureRun(t, "deps", "licenses", "--strict", root)
	if status != 0 || stderr != "" || !strings.Contains(stdout, "example.com/library@v0.0.1\tLICENSE\tsha256:") {
		t.Fatalf("licenses: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, stdout, stderr := captureRun(t, "deps", "remove", "--offline", "example.com/library", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("remove: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, stdout, stderr := captureRun(t, "deps", "check", root); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("check after remove: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifest+"\n# stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := captureRun(t, "deps", "check", root); status != 1 || !strings.Contains(stderr, "does not match") {
		t.Fatalf("stale check: status=%d stderr=%q", status, stderr)
	}
}

func TestDependencyLicensesStrictUnknown(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "unknown-license"
version = "0.1.0"
go-module = "example.com/application"
go-version = "1.23"

[go.dependencies]
"example.com/library" = "v0.0.0"

[go.replacements]
"example.com/library" = "library"
`
	for path, contents := range map[string]string{
		filepath.Join(root, product.ProjectFileName): manifest,
		filepath.Join(library, "go.mod"):             "module example.com/library\n\ngo 1.23\n",
		filepath.Join(library, "library.go"):         "package library\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if status, _, stderr := captureRun(t, "deps", "lock", "--offline", root); status != 0 {
		t.Fatalf("lock status=%d stderr=%q", status, stderr)
	}
	status, stdout, stderr := captureRun(t, "deps", "licenses", root)
	if status != 0 || stderr != "" || stdout != "example.com/library@v0.0.0\tunknown\n" {
		t.Fatalf("licenses: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	status, stdout, stderr = captureRun(t, "deps", "licenses", "--strict", root)
	if status != 1 || stdout != "example.com/library@v0.0.0\tunknown\n" || !strings.Contains(stderr, "1 Go module(s)") {
		t.Fatalf("strict: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
}

func TestCrossTargetBuildAndRunMatrix(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	goos := "windows"
	magic := "MZ"
	extension := ".exe"
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		goos = "linux"
		magic = "\x7fELF"
		extension = ""
	}
	manifest := `[project]
name = "cross-target"
version = "0.1.0"
go-module = "example.com/cross-target"
go-version = "1.23"

[target]
goos = "` + goos + `"
goarch = "amd64"
cgo = "disabled"
tags = "kinmokusei_cross"

[go.dependencies]
"example.com/target-library" = "v0.0.0"

[go.replacements]
"example.com/target-library" = "library"
`
	source := filepath.Join(root, "main.km")
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/target-library\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "base.go"), []byte("package targetlibrary\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetSource := "//go:build " + goos + " && amd64 && kinmokusei_cross\n\npackage targetlibrary\nfunc TargetValue() int { return 42 }\n"
	if err := os.WriteFile(filepath.Join(library, "target_"+goos+"_amd64.go"), []byte(targetSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(`import go target from "example.com/target-library"; function main(): void { target.TargetValue(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	status, stdout, stderr := captureRun(t, "target", root)
	if status != 0 || stderr != "" || stdout != "GOOS="+goos+"\nGOARCH=amd64\nCGO_ENABLED=0\nTAGS=kinmokusei_cross\n" {
		t.Fatalf("target: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	output := filepath.Join(root, "cross"+extension)
	t.Setenv("GOOS", "notreal")
	t.Setenv("GOARCH", "notreal")
	t.Setenv("CGO_ENABLED", "1")
	status, stdout, stderr = captureRun(t, "build", "-o", output, source)
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("cross build: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	contents, err := os.ReadFile(output)
	if err != nil || !strings.HasPrefix(string(contents), magic) {
		t.Fatalf("cross binary prefix=%q err=%v", contents[:min(len(contents), 4)], err)
	}
	status, stdout, stderr = captureRun(t, "run", source)
	if status != 1 || stdout != "" || !strings.Contains(stderr, "cannot run cross target") || !strings.Contains(stderr, "use build instead") {
		t.Fatalf("cross run: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
}

func TestHelpAliases(t *testing.T) {
	for _, argument := range []string{"help", "-h", "--help"} {
		t.Run(argument, func(t *testing.T) {
			status, stdout, stderr := captureRun(t, argument)
			if status != 0 || stdout != "" || !strings.Contains(stderr, "usage:") {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
			}
		})
	}
}

func TestUsageAndMissingSourceUseProductIdentity(t *testing.T) {
	status, _, usageText := captureRun(t, "help")
	if status != 0 || !strings.Contains(usageText, "usage: "+product.CommandName+" ") || !strings.Contains(usageText, "<source"+product.SourceExtension+">") {
		t.Fatalf("status=%d usage=%q", status, usageText)
	}
	for _, command := range []string{"check", "build", "run", "emit-go"} {
		t.Run(command, func(t *testing.T) {
			status, _, stderr := captureRun(t, command)
			if status != 2 || !strings.Contains(stderr, product.SourceExtension+" source file") {
				t.Fatalf("status=%d stderr=%q", status, stderr)
			}
		})
	}
}

func TestLSPStdioLifecycle(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	shutdown := `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	framed := func(message string) string {
		return "Content-Length: " + strconv.Itoa(len(message)) + "\r\n\r\n" + message
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.WriteString(framed(input) + framed(shutdown) + framed(exit)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	previousStdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = previousStdin
		_ = reader.Close()
	})
	status, stdout, stderr := captureRun(t, "lsp", "--stdio")
	if status != 0 || stderr != "" || !strings.Contains(stdout, `"name":"`+product.DisplayName+`"`) || !strings.Contains(stdout, `"id":2,"result":null`) {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
}

func TestCheckAndEmitGoMatrix(t *testing.T) {
	temp := t.TempDir()
	valid := filepath.Join(temp, "valid.km")
	invalid := filepath.Join(temp, "invalid.km")
	if err := os.WriteFile(valid, []byte(`interface Value { function get(): int; } class Item implements Value { public function get(): int { return 42; } } function answer(): Value { return new Item(); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalid, []byte(`function answer(): int { return "wrong"; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, stdout, stderr := captureRun(t, "check", valid); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("valid check: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, _, stderr := captureRun(t, "check", invalid); status != 1 || !strings.Contains(stderr, "cannot use string as int") {
		t.Fatalf("invalid check: status=%d stderr=%q", status, stderr)
	}
	if status, _, stderr := captureRun(t, "check", filepath.Join(temp, "missing.km")); status != 1 || stderr == "" {
		t.Fatalf("missing check: status=%d stderr=%q", status, stderr)
	}
	status, stdout, stderr := captureRun(t, "emit-go", "-package", "custom", valid)
	if status != 0 || stderr != "" || !strings.Contains(stdout, "package custom") || !strings.Contains(stdout, "type Value interface") {
		t.Fatalf("stdout emit: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	output := filepath.Join(temp, "generated.go")
	status, stdout, stderr = captureRun(t, "emit-go", "-o", output, valid)
	if status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("file emit: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	generated, err := os.ReadFile(output)
	if err != nil || !strings.Contains(string(generated), "package main") {
		t.Fatalf("generated=%q err=%v", generated, err)
	}
	if status, _, stderr = captureRun(t, "emit-go", "-o", temp, valid); status != 1 || stderr == "" {
		t.Fatalf("unwritable output: status=%d stderr=%q", status, stderr)
	}
}

func TestCheckJSONDiagnosticMatrix(t *testing.T) {
	temp := t.TempDir()
	valid := filepath.Join(temp, "valid.km")
	additional := filepath.Join(temp, "additional.km")
	lexical := filepath.Join(temp, "lexical.km")
	semantic := filepath.Join(temp, "semantic.km")
	syntax := filepath.Join(temp, "syntax.km")
	library := filepath.Join(temp, "library.km")
	entry := filepath.Join(temp, "entry.km")
	for path, source := range map[string]string{
		valid:      `function answer(): int { return 42; }`,
		additional: `function label(): string { return "ready"; }`,
		lexical:    `function broken(): void { @ }`,
		semantic:   `function first(): int { return "wrong"; } function second(): boolean { return 42; }`,
		syntax:     `function broken(values: int[]): int[] { return values[1:2; }`,
		library:    `function value(): int { return "wrong"; }`,
		entry:      `import { value } from "./library"; function answer(): int { return value(); }`,
	} {
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	decode := func(t *testing.T, arguments ...string) (int, checkReport) {
		t.Helper()
		status, stdout, stderr := captureRun(t, arguments...)
		if stderr != "" {
			t.Fatalf("%v wrote non-JSON stderr %q", arguments, stderr)
		}
		var report checkReport
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("%v produced invalid JSON: %v\n%s", arguments, err, stdout)
		}
		if report.Diagnostics == nil {
			t.Fatalf("%v encoded diagnostics as null", arguments)
		}
		return status, report
	}

	if status, report := decode(t, "check", "--json", valid, additional); status != 0 || !report.Valid || len(report.Diagnostics) != 0 || report.Error != "" {
		t.Fatalf("valid report: status=%d report=%#v", status, report)
	}

	status, report := decode(t, "check", "--json", lexical)
	if status != 1 || report.Valid || len(report.Diagnostics) == 0 || !strings.Contains(report.Diagnostics[0].Message, "unexpected character") {
		t.Fatalf("lexical report: status=%d report=%#v", status, report)
	}

	status, report = decode(t, "check", "--json", semantic)
	if status != 1 || report.Valid || report.Error != "" || len(report.Diagnostics) != 2 {
		t.Fatalf("semantic report: status=%d report=%#v", status, report)
	}
	for _, item := range report.Diagnostics {
		if item.Path != semantic || item.Message == "" || item.Start.Line != 1 || item.Start.Column <= 0 || item.End.Offset < item.Start.Offset {
			t.Errorf("incomplete semantic diagnostic: %#v", item)
		}
	}

	status, report = decode(t, "check", "--json", syntax)
	if status != 1 || report.Valid || len(report.Diagnostics) == 0 || !strings.Contains(report.Diagnostics[0].Message, "expected ']' after slice expression") {
		t.Fatalf("syntax report: status=%d report=%#v", status, report)
	}

	status, report = decode(t, "check", "--json", entry)
	if status != 1 || report.Valid || report.Error != "" || len(report.Diagnostics) == 0 || report.Diagnostics[0].Path != library {
		t.Fatalf("imported-module report: status=%d report=%#v", status, report)
	}

	missing := filepath.Join(temp, "missing.km")
	status, report = decode(t, "check", "--json", missing)
	if status != 1 || report.Valid || report.Error == "" || len(report.Diagnostics) != 0 {
		t.Fatalf("I/O report: status=%d report=%#v", status, report)
	}
}

func TestEmitCABIMatrix(t *testing.T) {
	temp := t.TempDir()
	valid := filepath.Join(temp, "valid.km")
	invalid := filepath.Join(temp, "invalid.km")
	if err := os.WriteFile(valid, []byte(`export c("kinmokusei_answer") function answer(): int32 { return 42; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalid, []byte(`export c("kinmokusei_answer") function answer(value: string): void {}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := captureRun(t, "emit-c-abi", valid); status != 2 || !strings.Contains(stderr, "requires -o") {
		t.Fatalf("missing output: status=%d stderr=%q", status, stderr)
	}
	output := filepath.Join(temp, "abi")
	if status, stdout, stderr := captureRun(t, "emit-c-abi", "-o", output, valid); status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("valid emit: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	for name, want := range map[string]string{
		"generated.go": "func answer() int32", "generated_cabi.go": "//export kinmokusei_answer", "kinmokusei_abi.h": "int32_t kinmokusei_answer",
		"kinmokusei_abi.json": `"symbol": "kinmokusei_answer"`,
	} {
		contents, err := os.ReadFile(filepath.Join(output, name))
		if err != nil || !strings.Contains(string(contents), want) {
			t.Fatalf("%s=%q err=%v want=%q", name, contents, err, want)
		}
	}
	invalidOutput := filepath.Join(temp, "invalid-abi")
	if status, _, stderr := captureRun(t, "emit-c-abi", "-o", invalidOutput, invalid); status != 1 || !strings.Contains(stderr, "unsupported type string") {
		t.Fatalf("invalid emit: status=%d stderr=%q", status, stderr)
	}
	if _, err := os.Stat(invalidOutput); !os.IsNotExist(err) {
		t.Fatalf("invalid compilation created output directory: %v", err)
	}
	fileOutput := filepath.Join(temp, "not-directory")
	if err := os.WriteFile(fileOutput, []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := captureRun(t, "emit-c-abi", "-o", fileOutput, valid); status != 1 || stderr == "" {
		t.Fatalf("file output: status=%d stderr=%q", status, stderr)
	}
}

func TestABICompatibilityCommandMatrix(t *testing.T) {
	temp := t.TempDir()
	base := filepath.Join(temp, "base.km")
	additive := filepath.Join(temp, "additive.km")
	breaking := filepath.Join(temp, "breaking.km")
	for path, source := range map[string]string{
		base:     `export c("kinmokusei_value") function value(input: int32): int32 { return input; }`,
		additive: `export c("kinmokusei_value") function value(input: int32): int32 { return input; } export c("kinmokusei_ping") function ping(): void {}`,
		breaking: `export c("kinmokusei_value") function value(input: int64): int32 { return int32(input); }`,
	} {
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(temp, "abi")
	if status, _, stderr := captureRun(t, "emit-c-abi", "-o", output, base); status != 0 {
		t.Fatalf("baseline emit status=%d stderr=%q", status, stderr)
	}
	manifest := filepath.Join(output, "kinmokusei_abi.json")
	if status, stdout, stderr := captureRun(t, "abi", "check", "--baseline", manifest, base); status != 0 || !strings.Contains(stdout, "exact fingerprint") || stderr != "" {
		t.Fatalf("exact: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, stdout, stderr := captureRun(t, "abi", "check", "--baseline", manifest, additive); status != 0 || !strings.Contains(stdout, "added symbols kinmokusei_ping") || stderr != "" {
		t.Fatalf("additive: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	if status, stdout, stderr := captureRun(t, "abi", "check", "--baseline", manifest, breaking); status != 1 || stdout != "" || !strings.Contains(stderr, `changed signature of C ABI symbol "kinmokusei_value"`) {
		t.Fatalf("breaking: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	malformed := filepath.Join(temp, "malformed.json")
	if err := os.WriteFile(malformed, []byte(`{"schemaVersion":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := captureRun(t, "abi", "check", "--baseline", malformed, base); status != 1 || !strings.Contains(stderr, "invalid baseline C ABI manifest") {
		t.Fatalf("malformed: status=%d stderr=%q", status, stderr)
	}
	if status, _, stderr := captureRun(t, "abi", "check", "--baseline", filepath.Join(temp, "missing.json"), base); status != 1 || !strings.Contains(stderr, "cannot read baseline") {
		t.Fatalf("missing: status=%d stderr=%q", status, stderr)
	}
}

func TestAllCompilationCommandsRejectInvalidProgram(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("GOCACHE", filepath.Join(temp, "go-cache"))
	source := filepath.Join(temp, "invalid.km")
	if err := os.WriteFile(source, []byte(`function answer(): int { return "wrong"; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"check", "emit-go", "build", "run"} {
		t.Run(command, func(t *testing.T) {
			status, stdout, stderr := captureRun(t, command, source)
			if status != 1 || stdout != "" || !strings.Contains(stderr, "cannot use string as int") {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
			}
		})
	}
}

func TestBuildAndRunPropagateGoFailures(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("GOCACHE", filepath.Join(temp, "go-cache"))
	source := filepath.Join(temp, "library.km")
	if err := os.WriteFile(source, []byte(`function answer(): int { return 42; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"build", "run"} {
		t.Run(command, func(t *testing.T) {
			status, _, stderr := captureRun(t, command, source)
			if status != 1 || stderr == "" {
				t.Fatalf("status=%d stderr=%q", status, stderr)
			}
		})
	}
}
