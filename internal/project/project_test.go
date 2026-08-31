package project

import (
	"archive/zip"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/product"
)

func validManifest() string {
	return `[project]
name = "example"
version = "0.1.0"
go-module = "example.com/application"
go-version = "1.23"

[go.dependencies]
"example.com/zeta" = "v1.2.3"
"example.com/alpha" = "v0.0.0"

[go.replacements]
"example.com/alpha" = "./library"
`
}

func TestManifestSuccessAndDeterministicGoMod(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, product.ProjectFileName)
	input := strings.Replace(validManifest(), `"example.com/alpha" = "./library"`, `"example.com/alpha" = "./library#literal" # comment`, 1)
	manifest, err := ParseManifest(path, []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Project.Name != "example" || manifest.Dependencies["example.com/zeta"] != "v1.2.3" || manifest.Replacements["example.com/alpha"] != "./library#literal" {
		t.Fatalf("manifest = %#v", manifest)
	}
	moduleDirectory := filepath.Join(root, product.StateDirectoryName, "deps")
	generated, err := RenderGoMod(manifest, moduleDirectory)
	if err != nil {
		t.Fatal(err)
	}
	want := `module example.com/application

go 1.23

require (
	example.com/alpha v0.0.0
	example.com/zeta v1.2.3
)

replace example.com/alpha => ../../library#literal
`
	if string(generated) != want {
		t.Fatalf("generated go.mod:\n%s\nwant:\n%s", generated, want)
	}
}

func TestResolvedGoModKeepsDirectOrderAndCanonicalIndirectRequirements(t *testing.T) {
	root := t.TempDir()
	manifest, err := ParseManifest(filepath.Join(root, product.ProjectFileName), []byte(validManifest()))
	if err != nil {
		t.Fatal(err)
	}
	generated, err := RenderResolvedGoMod(manifest, map[string]string{
		"example.com/zeta":             "v1.2.3",
		"example.com/transitive-zeta":  "v2.0.0",
		"example.com/transitive-alpha": "v1.0.0",
	}, filepath.Join(root, product.StateDirectoryName, "deps"))
	if err != nil {
		t.Fatal(err)
	}
	want := `module example.com/application

go 1.23

require (
	example.com/alpha v0.0.0
	example.com/zeta v1.2.3
)

require (
	example.com/transitive-alpha v1.0.0 // indirect
	example.com/transitive-zeta v2.0.0 // indirect
)

replace example.com/alpha => ../../library
`
	if string(generated) != want {
		t.Fatalf("resolved go.mod:\n%s\nwant:\n%s", generated, want)
	}
}

func TestDependencyProbeCollectsOnlyParsedGoImports(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, product.StateDirectoryName)
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{
		filepath.Join(root, "main.otm"): `
// import go fake from "example.invalid/comment";
import go http from "net/http";
import go fiber from "github.com/gofiber/fiber/v3";
function main(): void {}
`,
		filepath.Join(state, "ignored.otm"): `import go ignored from "example.invalid/state";`,
		filepath.Join(root, "notes.txt"):    `import go ignored from "example.invalid/text";`,
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	destination := t.TempDir()
	if err := writeDependencyProbe(root, destination); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "ontama_dependencies.go"))
	if err != nil {
		t.Fatal(err)
	}
	want := `package ontamadependencies

import (
	_ "github.com/gofiber/fiber/v3"
	_ "net/http"
)
`
	if string(contents) != want {
		t.Fatalf("dependency probe:\n%s\nwant:\n%s", contents, want)
	}
}

func TestLockDependenciesMaterializesImportedTransitiveGraphForReadonlyBuilds(t *testing.T) {
	root := t.TempDir()
	proxy := filepath.Join(t.TempDir(), "proxy")
	writeProxyModule(t, proxy, "example.com/transitive", "v1.0.0", "module example.com/transitive\n\ngo 1.23\n", map[string]string{
		"transitive.go": "package transitive\nfunc Value() int { return 42 }\n",
		"LICENSE":       "transitive terms\n",
	})
	writeProxyModule(t, proxy, "example.com/direct", "v1.0.0", "module example.com/direct\n\ngo 1.23\n\nrequire example.com/transitive v1.0.0\n", map[string]string{
		"direct.go": "package direct\nimport \"example.com/transitive\"\nfunc Value() int { return transitive.Value() }\n",
		"LICENSE":   "direct terms\n",
	})
	manifest := `[project]
name = "transitive-lock"
version = "0.1.0"
go-module = "example.com/transitive-lock"
go-version = "1.23"

[go.dependencies]
"example.com/direct" = "v1.0.0"
`
	for path, contents := range map[string]string{
		filepath.Join(root, product.ProjectFileName): manifest,
		filepath.Join(root, "main.otm"):              `import go direct from "example.com/direct"; function main(): void { direct.Value(); }`,
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOPROXY", localFileURL(proxy))
	t.Setenv("GOSUMDB", "off")
	moduleCache := filepath.Join(t.TempDir(), "module-cache")
	t.Setenv("GOMODCACHE", moduleCache)
	t.Cleanup(func() {
		_ = filepath.WalkDir(moduleCache, func(path string, entry os.DirEntry, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "build-cache"))
	t.Setenv("GOTOOLCHAIN", "local")
	lock, err := LockDependencies(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Modules) != 2 || lock.Modules[0].Path != "example.com/direct" || lock.Modules[1].Path != "example.com/transitive" {
		t.Fatalf("locked modules=%#v", lock.Modules)
	}
	goMod, err := os.ReadFile(filepath.Join(product.DependencyDirectory(root), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"example.com/direct v1.0.0", "example.com/transitive v1.0.0 // indirect"} {
		if !strings.Contains(string(goMod), expected) {
			t.Errorf("locked go.mod misses %q:\n%s", expected, goMod)
		}
	}
	if err = CheckDependencies(root); err != nil {
		t.Fatal(err)
	}
}

func writeProxyModule(t *testing.T, proxy, module, version, goMod string, files map[string]string) {
	t.Helper()
	directory := filepath.Join(proxy, filepath.FromSlash(module), "@v")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
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
	prefix := module + "@" + version + "/"
	allFiles := make(map[string]string, len(files)+1)
	allFiles["go.mod"] = goMod
	for name, contents := range files {
		allFiles[name] = contents
	}
	for name, contents := range allFiles {
		entry, createErr := writer.Create(prefix + name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write([]byte(contents)); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = archive.Close(); err != nil {
		t.Fatal(err)
	}
}

func localFileURL(path string) string {
	slashPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	return (&url.URL{Scheme: "file", Path: slashPath}).String()
}

func TestManifestFailureMatrix(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"empty", "", "missing required keys"},
		{"before section", `name = "x"`, "before a section"},
		{"unknown section", "[unknown]\nname = \"x\"", "unknown section"},
		{"duplicate section", validManifest() + "\n[project]\n", "duplicate section"},
		{"unknown project key", strings.Replace(validManifest(), `name = "example"`, `unknown = "x"`, 1), "unknown project key"},
		{"duplicate project key", strings.Replace(validManifest(), `name = "example"`, "name = \"example\"\nname = \"again\"", 1), "duplicate project key"},
		{"unquoted value", strings.Replace(validManifest(), `version = "0.1.0"`, `version = 0.1.0`, 1), "quoted string"},
		{"unquoted module", strings.Replace(validManifest(), `"example.com/zeta" = "v1.2.3"`, `example.com/zeta = "v1.2.3"`, 1), "module paths must be quoted"},
		{"unterminated quote", strings.Replace(validManifest(), `name = "example"`, `name = "example`, 1), "unterminated quoted string"},
		{"invalid name", strings.Replace(validManifest(), `name = "example"`, `name = "bad name"`, 1), "project name"},
		{"incomplete version", strings.Replace(validManifest(), `version = "0.1.0"`, `version = "0.1"`, 1), "complete semantic version"},
		{"invalid Go version", strings.Replace(validManifest(), `go-version = "1.23"`, `go-version = "latest"`, 1), "go-version"},
		{"unknown target key", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[target]\nunknown = \"x\"\n\n[go.dependencies]", 1), "unknown target key"},
		{"duplicate target key", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[target]\ngoos = \"linux\"\ngoos = \"windows\"\n\n[go.dependencies]", 1), "duplicate target key"},
		{"invalid target os", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[target]\ngoos = \"Linux!\"\n\n[go.dependencies]", 1), "target goos"},
		{"invalid cgo mode", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[target]\ncgo = \"sometimes\"\n\n[go.dependencies]", 1), "target cgo"},
		{"empty target tag", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[target]\ntags = \"one,,two\"\n\n[go.dependencies]", 1), "empty build tag"},
		{"duplicate target tag", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[target]\ntags = \"one,one\"\n\n[go.dependencies]", 1), "duplicate build tag"},
		{"invalid target tag", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[target]\ntags = \"one-tag\"\n\n[go.dependencies]", 1), "target build tag"},
		{"unknown Go interop key", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[go.interop]\nunknown = \"allow\"\n\n[go.dependencies]", 1), "unknown Go interop key"},
		{"duplicate Go interop key", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[go.interop]\nunsafe = \"deny\"\nunsafe = \"allow\"\n\n[go.dependencies]", 1), "duplicate Go interop key"},
		{"invalid unsafe policy", strings.Replace(validManifest(), "\n[go.dependencies]", "\n[go.interop]\nunsafe = \"maybe\"\n\n[go.dependencies]", 1), "must be deny or allow"},
		{"self dependency", strings.Replace(validManifest(), `"example.com/zeta" = "v1.2.3"`, `"example.com/application" = "v1.2.3"`, 1), "cannot depend on itself"},
		{"invalid dependency version", strings.Replace(validManifest(), `"example.com/zeta" = "v1.2.3"`, `"example.com/zeta" = "latest"`, 1), "invalid complete version"},
		{"orphan replacement", strings.Replace(validManifest(), `"example.com/alpha" = "./library"`, `"example.com/missing" = "./library"`, 1), "no matching Go dependency"},
		{"absolute replacement", strings.Replace(validManifest(), `"example.com/alpha" = "./library"`, `"example.com/alpha" = "/tmp/library"`, 1), "project-relative"},
		{"Windows absolute replacement", strings.Replace(validManifest(), `"example.com/alpha" = "./library"`, `"example.com/alpha" = "C:/library"`, 1), "project-relative"},
		{"UNC replacement", strings.Replace(validManifest(), `"example.com/alpha" = "./library"`, `"example.com/alpha" = "\\\\server\\library"`, 1), "project-relative"},
		{"escaping replacement", strings.Replace(validManifest(), `"example.com/alpha" = "./library"`, `"example.com/alpha" = "../library"`, 1), "escapes the project root"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseManifest(filepath.Join(t.TempDir(), product.ProjectFileName), []byte(test.input))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

func TestGoInteropPolicyCanonicalization(t *testing.T) {
	input := strings.Replace(validManifest(), "\n[go.dependencies]", "\n[go.interop]\nunsafe = \"allow\"\n\n[go.dependencies]", 1)
	manifest, err := ParseManifest(filepath.Join(t.TempDir(), product.ProjectFileName), []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.AllowsUnsafeGoInterop() {
		t.Fatal("unsafe Go interop was not enabled")
	}
	canonical, err := RenderManifest(manifest)
	if err != nil || !strings.Contains(string(canonical), "[go.interop]\nunsafe = \"allow\"\n") {
		t.Fatalf("canonical=%s err=%v", canonical, err)
	}
	manifest.GoInterop.Unsafe = "deny"
	if manifest.AllowsUnsafeGoInterop() {
		t.Fatal("deny policy enabled unsafe Go interop")
	}
}

func TestTargetManifestCanonicalizationAndResolutionMatrix(t *testing.T) {
	input := strings.Replace(validManifest(), "\n[go.dependencies]", `
[target]
goos = "windows"
goarch = "amd64"
cgo = "disabled"
tags = "zeta, alpha"

[go.dependencies]`, 1)
	manifest, err := ParseManifest(filepath.Join(t.TempDir(), product.ProjectFileName), []byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(manifest.Target.Tags, ",") != "alpha,zeta" {
		t.Fatalf("tags=%v", manifest.Target.Tags)
	}
	canonical, err := RenderManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"[target]\n", `goos = "windows"`, `goarch = "amd64"`, `cgo = "disabled"`, `tags = "alpha,zeta"`} {
		if !strings.Contains(string(canonical), expected) {
			t.Fatalf("canonical manifest misses %q:\n%s", expected, canonical)
		}
	}
	target, err := ResolveTarget(manifest.Target)
	if err != nil || target.GOOS != "windows" || target.GOARCH != "amd64" || target.CGOEnabled {
		t.Fatalf("target=%#v err=%v", target, err)
	}
	defaultTarget, err := ResolveTarget(TargetConfig{})
	if err != nil || defaultTarget.GOOS != runtime.GOOS || defaultTarget.GOARCH != runtime.GOARCH || defaultTarget.Tags == nil {
		t.Fatalf("default target=%#v err=%v", defaultTarget, err)
	}
	crossAuto, err := ResolveTarget(TargetConfig{GOOS: "windows", GOARCH: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && crossAuto.CGOEnabled {
		t.Fatalf("cross auto target enabled cgo: %#v", crossAuto)
	}
	explicitCGO, err := ResolveTarget(TargetConfig{GOOS: "windows", GOARCH: "amd64", CGO: "enabled"})
	if err != nil || !explicitCGO.CGOEnabled {
		t.Fatalf("explicit cgo target=%#v err=%v", explicitCGO, err)
	}
	if _, err = ResolveTarget(TargetConfig{GOOS: "notreal", GOARCH: "notreal"}); err == nil || !strings.Contains(err.Error(), "unsupported Go build target") {
		t.Fatalf("unsupported target err=%v", err)
	}
}

func TestBuildTargetEnvironmentAndFlags(t *testing.T) {
	target := BuildTarget{GOOS: "linux", GOARCH: "arm64", CGOEnabled: false, Tags: []string{"alpha", "zeta"}}
	environment := target.Environment([]string{"PATH=/bin", "GOOS=wrong", "GOARCH=wrong", "CGO_ENABLED=1"})
	joined := strings.Join(environment, "\n")
	for _, expected := range []string{"PATH=/bin", "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("environment=%v misses %q", environment, expected)
		}
	}
	if strings.Contains(joined, "wrong") || strings.Contains(joined, "CGO_ENABLED=1") {
		t.Fatalf("environment retained overridden values: %v", environment)
	}
	if flags := target.GoBuildFlags(); len(flags) != 1 || flags[0] != "-tags=alpha,zeta" {
		t.Fatalf("flags=%v", flags)
	}
}

func TestLockRoundTripAndValidationMatrix(t *testing.T) {
	root := t.TempDir()
	manifest, err := ParseManifest(filepath.Join(root, product.ProjectFileName), []byte(validManifest()))
	if err != nil {
		t.Fatal(err)
	}
	target, err := ResolveTarget(manifest.Target)
	if err != nil {
		t.Fatal(err)
	}
	lock := manifest.NewLock([]byte("go.mod"), nil, target, []LockedModule{{Path: "example.com/zeta", Version: "v1.2.3"}, {Path: "example.com/alpha", Version: "v0.0.0", ReplacePath: "library"}})
	if err = WriteLock(root, lock); err != nil {
		t.Fatal(err)
	}
	read, err := ReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Modules) != 2 || read.Modules[0].Path != "example.com/alpha" || read.Modules[1].Path != "example.com/zeta" {
		t.Fatalf("lock modules = %#v", read.Modules)
	}
	contents, err := os.ReadFile(filepath.Join(root, product.LockFileName))
	if err != nil || !strings.HasSuffix(string(contents), "\n") {
		t.Fatalf("lock contents = %q, err=%v", contents, err)
	}
	for name, contents := range map[string]string{
		"unknown field": `{"lockVersion":3,"unknown":true}`,
		"wrong version": `{"lockVersion":2}`,
		"trailing":      string(contents) + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.WriteFile(filepath.Join(caseRoot, product.LockFileName), []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadLock(caseRoot); err == nil {
				t.Fatal("invalid lock was accepted")
			}
		})
	}
}

func TestLockLicenseMetadataValidationMatrix(t *testing.T) {
	digest := Hash([]byte("license"))
	base := Lock{
		LockVersion: LockVersion, ManifestHash: Hash(nil), GoVersion: "1.23", Target: BuildTarget{GOOS: "linux", GOARCH: "amd64", Tags: []string{}}, GoModHash: Hash(nil), GoSumHash: Hash(nil),
		Modules: []LockedModule{{Path: "example.com/library", Version: "v1.0.0", LicenseFiles: []LockedLicenseFile{{Path: "LICENSE", SHA256: digest}}}},
	}
	for _, test := range []struct {
		name   string
		mutate func(*Lock)
		want   string
	}{
		{"missing metadata", func(lock *Lock) { lock.Modules[0].LicenseFiles = nil }, "missing licenseFiles"},
		{"absolute path", func(lock *Lock) { lock.Modules[0].LicenseFiles[0].Path = "/LICENSE" }, "invalid path"},
		{"nested path", func(lock *Lock) { lock.Modules[0].LicenseFiles[0].Path = "legal/LICENSE" }, "invalid path"},
		{"invalid digest", func(lock *Lock) { lock.Modules[0].LicenseFiles[0].SHA256 = "sha256:no" }, "invalid SHA-256"},
		{"duplicate file", func(lock *Lock) {
			lock.Modules[0].LicenseFiles = append(lock.Modules[0].LicenseFiles, lock.Modules[0].LicenseFiles[0])
		}, "uniquely sorted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			lock := base
			lock.Modules = append([]LockedModule(nil), base.Modules...)
			lock.Modules[0].LicenseFiles = append([]LockedLicenseFile(nil), base.Modules[0].LicenseFiles...)
			test.mutate(&lock)
			if err := lock.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v, want %q", err, test.want)
			}
		})
	}
}

func TestLockTargetMetadataValidationMatrix(t *testing.T) {
	base := Lock{LockVersion: LockVersion, ManifestHash: Hash(nil), GoVersion: "1.23", Target: BuildTarget{GOOS: "linux", GOARCH: "amd64", Tags: []string{}}, GoModHash: Hash(nil), GoSumHash: Hash(nil), Modules: []LockedModule{}}
	for _, test := range []struct {
		name   string
		mutate func(*Lock)
		want   string
	}{
		{"missing goos", func(lock *Lock) { lock.Target.GOOS = "" }, "include goos"},
		{"invalid goarch", func(lock *Lock) { lock.Target.GOARCH = "AMD64!" }, "invalid goos or goarch"},
		{"missing tags", func(lock *Lock) { lock.Target.Tags = nil }, "missing tags metadata"},
		{"unsorted tags", func(lock *Lock) { lock.Target.Tags = []string{"zeta", "alpha"} }, "uniquely sorted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			lock := base
			lock.Target.Tags = append([]string(nil), base.Target.Tags...)
			test.mutate(&lock)
			if err := lock.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v, want %q", err, test.want)
			}
		})
	}
}

func TestLicenseFileNameMatrix(t *testing.T) {
	for _, name := range []string{"LICENSE", "license.txt", "LICENSE-MIT", "LICENCE.md", "COPYING", "copying.apache-2.0"} {
		if !isLicenseFileName(name) {
			t.Errorf("%q was not detected", name)
		}
	}
	for _, name := range []string{"README.md", "NOTICE", "UNLICENSED", "LICENSES", "my-license.txt", "LICENSE_directory/file"} {
		if isLicenseFileName(name) {
			t.Errorf("%q was unexpectedly detected", name)
		}
	}
}

func TestOfflineLicenseMetadataAndIntegrity(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "licenses"
version = "0.1.0"
go-module = "example.com/licenses"
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
		filepath.Join(library, "LICENSE-MIT"):        "MIT terms\n",
		filepath.Join(library, "COPYING.txt"):        "copying terms\n",
		filepath.Join(library, "NOTICE"):             "not classified as a license\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lock, err := LockDependencies(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Modules) != 1 || len(lock.Modules[0].LicenseFiles) != 2 {
		t.Fatalf("license metadata = %#v", lock.Modules)
	}
	want := []LockedLicenseFile{{Path: "COPYING.txt", SHA256: Hash([]byte("copying terms\n"))}, {Path: "LICENSE-MIT", SHA256: Hash([]byte("MIT terms\n"))}}
	for index := range want {
		if lock.Modules[0].LicenseFiles[index] != want[index] {
			t.Fatalf("licenseFiles[%d]=%#v, want %#v", index, lock.Modules[0].LicenseFiles[index], want[index])
		}
	}
	report, err := DependencyLicenses(root)
	if err != nil || len(report) != 1 || len(report[0].Files) != 2 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	if err = os.WriteFile(filepath.Join(library, "LICENSE-MIT"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = CheckDependencies(root); err == nil || !strings.Contains(err.Error(), "module graph does not match") {
		t.Fatalf("changed license err=%v", err)
	}
}

func TestLicenseMetadataFailureMatrix(t *testing.T) {
	root := t.TempDir()
	license := filepath.Join(root, "LICENSE")
	if err := os.Symlink(filepath.Join(root, "missing"), license); err != nil {
		t.Fatal(err)
	}
	if _, err := findLicenseFiles(listedModule{Path: "example.com/library", Dir: root}); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("symlink err=%v", err)
	}
	if err := os.Remove(license); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(license)
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Truncate(maximumLicenseFileSize + 1); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = findLicenseFiles(listedModule{Path: "example.com/library", Dir: root}); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized err=%v", err)
	}
}

func TestOfflineLocalDependencyLockAndIntegrityMatrix(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := strings.Replace(validManifest(), "\"example.com/zeta\" = \"v1.2.3\"\n", "", 1)
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/alpha\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "library.go"), []byte("package alpha\nfunc Value() int { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := LockDependencies(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Modules) != 1 || lock.Modules[0].Path != "example.com/alpha" || lock.Modules[0].ReplacePath != "library" {
		t.Fatalf("lock = %#v", lock)
	}
	if err = CheckDependencies(root); err != nil {
		t.Fatal(err)
	}
	goModPath := filepath.Join(product.DependencyDirectory(root), "go.mod")
	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(goModPath, append(goMod, []byte("\n// modified\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err = ValidateLockedFiles(root); err == nil || !strings.Contains(err.Error(), "go.mod was modified") {
		t.Fatalf("modified go.mod err = %v", err)
	}
	if _, err = LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifestText+"\n# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err = ValidateLockedFiles(root); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("stale manifest err = %v", err)
	}
}

func TestLockedTargetOverridesAmbientEnvironmentAndDetectsMismatch(t *testing.T) {
	root := t.TempDir()
	manifest := `[project]
name = "target-lock"
version = "0.1.0"
go-module = "example.com/target-lock"
go-version = "1.23"

[target]
cgo = "disabled"
tags = "zeta,alpha"
`
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := LockDependencies(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Target.GOOS != runtime.GOOS || lock.Target.GOARCH != runtime.GOARCH || lock.Target.CGOEnabled || strings.Join(lock.Target.Tags, ",") != "alpha,zeta" {
		t.Fatalf("locked target=%#v", lock.Target)
	}
	t.Setenv("GOOS", "notreal")
	t.Setenv("GOARCH", "notreal")
	t.Setenv("CGO_ENABLED", "1")
	if err = CheckDependencies(root); err != nil {
		t.Fatalf("ambient environment overrode lock: %v", err)
	}
	lock.Target.GOOS = "plan9"
	lock.Target.GOARCH = "amd64"
	if err = WriteLock(root, lock); err != nil {
		t.Fatal(err)
	}
	if err = CheckDependencies(root); err == nil || !strings.Contains(err.Error(), "lock target") {
		t.Fatalf("target mismatch err=%v", err)
	}
}

func TestDependencyResolutionFailureMatrix(t *testing.T) {
	t.Run("missing local replacement", func(t *testing.T) {
		root := t.TempDir()
		manifestText := strings.Replace(validManifest(), "\"example.com/zeta\" = \"v1.2.3\"\n", "", 1)
		if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifestText), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LockDependencies(root, true); err == nil || !strings.Contains(err.Error(), "cannot resolve replacement") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("symlink escapes root", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "go.mod"), []byte("module example.com/alpha\n\ngo 1.23\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "library")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		manifestText := strings.Replace(validManifest(), "\"example.com/zeta\" = \"v1.2.3\"\n", "", 1)
		if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifestText), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LockDependencies(root, true); err == nil || !strings.Contains(err.Error(), "outside the project root") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("offline module unavailable", func(t *testing.T) {
		root := t.TempDir()
		manifestText := `[project]
name = "offline"
version = "0.1.0"
go-module = "example.com/offline"
go-version = "1.23"

[go.dependencies]
"example.invalid/ontama-unavailable" = "v1.0.0"
`
		if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifestText), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LockDependencies(root, true); err == nil || !strings.Contains(err.Error(), "GOPROXY=off") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestDependencyIntegrityDetectsChecksumsAndGraphChanges(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := strings.Replace(validManifest(), "\"example.com/zeta\" = \"v1.2.3\"\n", "", 1)
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/alpha\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "library.go"), []byte("package alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	goSumPath := filepath.Join(product.DependencyDirectory(root), "go.sum")
	if err := os.WriteFile(goSumPath, []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ValidateLockedFiles(root); err == nil || !strings.Contains(err.Error(), "go.sum was modified") {
		t.Fatalf("modified go.sum err = %v", err)
	}
	if _, err := LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	lock, err := ReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	lock.Modules = nil
	if err = WriteLock(root, lock); err != nil {
		t.Fatal(err)
	}
	if err = CheckDependencies(root); err == nil || !strings.Contains(err.Error(), "module graph does not match") {
		t.Fatalf("changed graph err = %v", err)
	}
}

func TestDependencyAddUpdateRemoveOfflineRoundTrip(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestText := `[project]
name = "editing"
version = "0.1.0"
go-module = "example.com/editing"
go-version = "1.23"
`
	if err := os.WriteFile(filepath.Join(root, product.ProjectFileName), []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/library\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "library.go"), []byte("package library\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddDependency(root, "example.com/library", "v0.0.0", "./library", true); err != nil {
		t.Fatal(err)
	}
	if err := CheckDependencies(root); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, product.ProjectFileName))
	if err != nil || !strings.Contains(string(contents), "[go.dependencies]") || !strings.Contains(string(contents), `"example.com/library" = "v0.0.0"`) || !strings.Contains(string(contents), `"example.com/library" = "library"`) {
		t.Fatalf("added manifest err=%v:\n%s", err, contents)
	}
	beforeDuplicate := string(contents)
	if err = AddDependency(root, "example.com/library", "v0.0.1", "", true); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate err = %v", err)
	}
	contents, _ = os.ReadFile(filepath.Join(root, product.ProjectFileName))
	if string(contents) != beforeDuplicate {
		t.Fatal("duplicate add modified manifest")
	}
	if err = UpdateDependency(root, "example.com/library", "v0.0.1", true); err != nil {
		t.Fatal(err)
	}
	if err = CheckDependencies(root); err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(filepath.Join(root, product.ProjectFileName))
	if err != nil || !strings.Contains(string(contents), `"example.com/library" = "v0.0.1"`) || !strings.Contains(string(contents), `"example.com/library" = "library"`) {
		t.Fatalf("updated manifest err=%v:\n%s", err, contents)
	}
	beforeSameVersion := string(contents)
	if err = UpdateDependency(root, "example.com/library", "v0.0.1", true); err == nil || !strings.Contains(err.Error(), "already uses") {
		t.Fatalf("same version err = %v", err)
	}
	contents, _ = os.ReadFile(filepath.Join(root, product.ProjectFileName))
	if string(contents) != beforeSameVersion {
		t.Fatal("same-version update modified manifest")
	}
	if err = RemoveDependency(root, "example.com/library", true); err != nil {
		t.Fatal(err)
	}
	if err = CheckDependencies(root); err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(filepath.Join(root, product.ProjectFileName))
	if err != nil || strings.Contains(string(contents), "go.dependencies") || strings.Contains(string(contents), "go.replacements") {
		t.Fatalf("removed manifest err=%v:\n%s", err, contents)
	}
	if err = RemoveDependency(root, "example.com/library", true); err == nil || !strings.Contains(err.Error(), "is not declared") {
		t.Fatalf("missing remove err = %v", err)
	}
	if err = UpdateDependency(root, "example.com/library", "v0.0.2", true); err == nil || !strings.Contains(err.Error(), "is not declared") {
		t.Fatalf("missing update err = %v", err)
	}
}

func TestDependencyAddRollbackPreservesAllLockedState(t *testing.T) {
	root := t.TempDir()
	manifestText := `[project]
name = "rollback"
version = "0.1.0"
go-module = "example.com/rollback"
go-version = "1.23"
`
	manifestPath := filepath.Join(root, product.ProjectFileName)
	if err := os.WriteFile(manifestPath, []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LockDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	paths := []string{manifestPath, filepath.Join(root, product.LockFileName), filepath.Join(product.DependencyDirectory(root), "go.mod"), filepath.Join(product.DependencyDirectory(root), "go.sum")}
	before := map[string]string{}
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = string(contents)
	}
	err := AddDependency(root, "example.invalid/ontama-add-rollback", "v1.0.0", "", true)
	if err == nil || !strings.Contains(err.Error(), "original dependency state restored") {
		t.Fatalf("rollback err = %v", err)
	}
	for _, path := range paths {
		contents, readErr := os.ReadFile(path)
		if readErr != nil || string(contents) != before[path] {
			t.Fatalf("state %q changed after rollback: err=%v\n%s", path, readErr, contents)
		}
	}
}

func TestInitialDependencyAddRollbackRemovesGeneratedState(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, product.ProjectFileName)
	manifest := `[project]
name = "initial-rollback"
version = "0.1.0"
go-module = "example.com/initial-rollback"
go-version = "1.23"
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	err := AddDependency(root, "example.invalid/initial-rollback", "v1.0.0", "", true)
	if err == nil || !strings.Contains(err.Error(), "original dependency state restored") {
		t.Fatalf("rollback err=%v", err)
	}
	contents, readErr := os.ReadFile(manifestPath)
	if readErr != nil || string(contents) != manifest {
		t.Fatalf("manifest changed: err=%v\n%s", readErr, contents)
	}
	for _, path := range []string{filepath.Join(root, product.LockFileName), filepath.Join(root, product.StateDirectoryName)} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("generated state %q remains: %v", path, statErr)
		}
	}
}

func TestDependencyUpdateRollbackPreservesAllLockedState(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, product.ProjectFileName)
	manifestText := `[project]
name = "update-rollback"
version = "0.1.0"
go-module = "example.com/update-rollback"
go-version = "1.23"
`
	if err := os.WriteFile(manifestPath, []byte(manifestText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/library\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "library.go"), []byte("package library\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddDependency(root, "example.com/library", "v0.0.0", "library", true); err != nil {
		t.Fatal(err)
	}
	paths := []string{manifestPath, filepath.Join(root, product.LockFileName), filepath.Join(product.DependencyDirectory(root), "go.mod"), filepath.Join(product.DependencyDirectory(root), "go.sum")}
	before := map[string]string{}
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = string(contents)
	}
	missingLibrary := filepath.Join(root, "library-missing")
	if err := os.Rename(library, missingLibrary); err != nil {
		t.Fatal(err)
	}
	err := UpdateDependency(root, "example.com/library", "v0.0.1", true)
	if err == nil || !strings.Contains(err.Error(), "original dependency state restored") {
		t.Fatalf("rollback err = %v", err)
	}
	for _, path := range paths {
		contents, readErr := os.ReadFile(path)
		if readErr != nil || string(contents) != before[path] {
			t.Fatalf("state %q changed after update rollback: err=%v\n%s", path, readErr, contents)
		}
	}
}

func TestDependencyArgumentFailureMatrix(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"", "form"}, {"example.com/library", "form"}, {"@v1.0.0", "form"}, {"example.com/library@", "form"}, {"library@v1.0.0", "domain-like"}, {"example.com/library@latest", "complete"}, {"example.com/library@v1", "complete"},
	} {
		if _, _, err := ParseDependencyArgument(test.input); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("ParseDependencyArgument(%q) err=%v, want %q", test.input, err, test.want)
		}
	}
	path, version, err := ParseDependencyArgument("example.com/library@v1.2.3")
	if err != nil || path != "example.com/library" || version != "v1.2.3" {
		t.Fatalf("path=%q version=%q err=%v", path, version, err)
	}
}
