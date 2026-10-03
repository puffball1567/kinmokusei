package main

import (
	"archive/zip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

// A Kinmokusei source package and its cgo helper are distinct versioned Go
// modules. Consumers should need only the source dependency, not copied C
// files, a Go replacement, or an explicit helper dependency.
func TestDistributedNativePackageWorkflow(t *testing.T) {
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	if _, err := exec.LookPath(cc); err != nil {
		if os.Getenv("KINMOKUSEI_REQUIRE_C_TESTS") == "1" {
			t.Fatalf("required C compiler %q unavailable: %v", cc, err)
		}
		t.Skipf("C compiler %q unavailable", cc)
	}
	base := t.TempDir()
	proxy := filepath.Join(base, "proxy")
	app := filepath.Join(base, "app")
	ffi := filepath.Join(base, "ffi")
	if err := os.MkdirAll(ffi, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(ffi, "binding.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schemaVersion":1,"package":"ffi","header":"fixture.h","threadPolicy":"threadSafe","functions":[{"name":"Answer","symbol":"ray_answer","parameters":[],"result":"int32","convention":"direct"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, out, stderr := captureRun(t, "ffi", "generate", "--manifest", manifestPath, "-o", ffi); status != 0 || out != "" || stderr != "" {
		t.Fatalf("ffi generate: status=%d out=%s err=%s", status, out, stderr)
	}
	generated, err := os.ReadFile(filepath.Join(ffi, "generated_ffi.go"))
	if err != nil {
		t.Fatal(err)
	}
	const sourceModule = "binding.test/raylibkm"
	const nativeModule = sourceModule + "/native"
	writeNativeProxyModule(t, proxy, nativeModule, "v0.1.0", map[string]string{
		"ffi/generated_ffi.go": string(generated),
		"ffi/fixture.h":        "#include <stdint.h>\nint32_t ray_answer(void);\n",
		"ffi/fixture.c":        "#include \"fixture.h\"\nint32_t ray_answer(void) { return 42; }\n",
		"LICENSE":              "MIT fixture\n",
	})
	writeNativeProxyModule(t, proxy, sourceModule, "v0.1.0", map[string]string{
		"kinmokusei.toml": `[project]
name = "raylibkm"
version = "0.1.0"
go-module = "binding.test/raylibkm"
go-version = "1.23"
[package]
entry = "index.km"
min-kinmokusei = "0.4.0"
backend = "go"
license = "MIT"
[target]
cgo = "enabled"
[go.dependencies]
"binding.test/raylibkm/native" = "v0.1.0"
`,
		"index.km": `import go ffi from "binding.test/raylibkm/native/ffi"; export function Answer(): int32 { return ffi.Answer(); }`,
		"LICENSE":  "MIT fixture\n",
	})
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"kinmokusei.toml": `[project]
name = "app"
version = "0.1.0"
go-module = "app.test/main"
go-version = "1.23"
[target]
cgo = "enabled"
`,
		"main.km": `import { Answer } from "binding.test/raylibkm"; import go fmt from "fmt"; function main(): void { fmt.Println(Answer()); }`,
	} {
		if err := os.WriteFile(filepath.Join(app, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	moduleCache := filepath.Join(base, "module-cache")
	t.Setenv("GOMODCACHE", moduleCache)
	t.Cleanup(func() {
		_ = filepath.WalkDir(moduleCache, func(path string, entry os.DirEntry, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
	t.Setenv("GOCACHE", filepath.Join(base, "build-cache"))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	slashProxy := filepath.ToSlash(proxy)
	if filepath.VolumeName(proxy) != "" && !strings.HasPrefix(slashProxy, "/") {
		slashProxy = "/" + slashProxy
	}
	t.Setenv("GOPROXY", (&url.URL{Scheme: "file", Path: slashProxy}).String())
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(app); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	}()
	if status, out, stderr := captureRun(t, "deps", "add", sourceModule+"@v0.1.0"); status != 0 || out != "" || stderr != "" {
		t.Fatalf("deps add: status=%d out=%s err=%s", status, out, stderr)
	}
	manifestBefore, err := os.ReadFile("kinmokusei.toml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifestBefore), "[go.dependencies]") || strings.Contains(string(manifestBefore), "[go.replacements]") {
		t.Fatalf("consumer should not declare native helper: %s", manifestBefore)
	}
	lockBefore, err := os.ReadFile("kinmokusei.lock")
	if err != nil {
		t.Fatal(err)
	}
	lock, err := project.ReadLock(app)
	if err != nil {
		t.Fatal(err)
	}
	lockedNative := false
	for _, module := range lock.Modules {
		if module.Path == nativeModule && module.Version == "v0.1.0" && module.Sum != "" {
			lockedNative = true
		}
	}
	if !lockedNative {
		t.Fatalf("native helper missing from checked lock: %#v", lock.Modules)
	}
	t.Setenv("GOPROXY", "off")
	for _, args := range [][]string{{"check"}, {"deps", "check"}, {"emit-go"}, {"build", "-o", "app.out"}, {"run"}} {
		status, out, stderr := captureRun(t, args...)
		if status != 0 || stderr != "" {
			t.Fatalf("%v: status=%d out=%s err=%s", args, status, out, stderr)
		}
		if args[0] == "run" && strings.TrimSpace(out) != "42" {
			t.Fatalf("run output=%q", out)
		}
	}
	binary := filepath.Join(app, "app.out")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	output, err := exec.Command(binary).CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "42" {
		t.Fatalf("built binary: output=%s err=%v", output, err)
	}
	manifestAfter, err := os.ReadFile("kinmokusei.toml")
	if err != nil || string(manifestBefore) != string(manifestAfter) {
		t.Fatal("normal commands changed manifest", err)
	}
	lockAfter, err := os.ReadFile("kinmokusei.lock")
	if err != nil || string(lockBefore) != string(lockAfter) {
		t.Fatal("normal commands changed lock", err)
	}
	disabled := strings.Replace(string(manifestBefore), `cgo = "enabled"`, `cgo = "disabled"`, 1)
	if err := os.WriteFile("kinmokusei.toml", []byte(disabled), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := captureRun(t, "deps", "lock", "--offline"); status == 0 || !strings.Contains(stderr, "incompatible with target") {
		t.Fatalf("disabled cgo: status=%d err=%s", status, stderr)
	}
	lockAfter, err = os.ReadFile("kinmokusei.lock")
	if err != nil || string(lockBefore) != string(lockAfter) {
		t.Fatal("failed cgo lock changed lock", err)
	}
}

func writeNativeProxyModule(t *testing.T, proxy, module, version string, files map[string]string) {
	t.Helper()
	directory := filepath.Join(proxy, filepath.FromSlash(module), "@v")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	goMod := "module " + module + "\n\ngo 1.23\n"
	for name, contents := range map[string]string{
		version + ".info": `{"Version":"` + version + `","Time":"2026-01-01T00:00:00Z"}`,
		version + ".mod":  goMod,
		"list":            version + "\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	archive, err := os.Create(filepath.Join(directory, version+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	allFiles := map[string]string{"go.mod": goMod}
	for name, contents := range files {
		allFiles[name] = contents
	}
	for name, contents := range allFiles {
		entry, err := writer.Create(module + "@" + version + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
}
