package project

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func packageManifest(module string) string {
	return fmt.Sprintf("[project]\nname = \"fixture\"\nversion = \"0.1.0\"\ngo-module = %q\ngo-version = \"1.23\"\n[package]\nentry = \"index.km\"\nmin-kinmokusei = \"0.4.0\"\nbackend = \"go\"\nlicense = \"MIT\"\n", module)
}

func packageFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, contents := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSourcePackageLocalGraphAndLiveEdits(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "app")
	library := filepath.Join(base, "library")
	utility := filepath.Join(base, "utility")
	packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main") + "[dependencies]\n\"pkg.test/library\" = \"v0.1.0\"\n[replace]\n\"pkg.test/library\" = \"../library\"\n\"pkg.test/utility\" = \"../utility\"\n", "index.km": ""})
	packageFiles(t, library, map[string]string{"kinmokusei.toml": packageManifest("pkg.test/library") + "[dependencies]\n\"pkg.test/utility\" = \"v0.1.0\"\n[exports]\n\"sub\" = \"src/sub.km\"\n", "index.km": "export function value():int{return 1;}", "src/sub.km": "export function other():int{return 2;}"})
	packageFiles(t, utility, map[string]string{"kinmokusei.toml": packageManifest("pkg.test/utility"), "index.km": ""})
	lock, err := LockDependencies(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) != 2 {
		t.Fatalf("packages=%v", lock.Packages)
	}
	manifest, err := ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := ReadPackageGraph(manifest, lock)
	if err != nil {
		t.Fatal(err)
	}
	file, err := graph.ResolveImport(filepath.Join(root, "index.km"), "pkg.test/library/sub")
	if err != nil || !strings.HasSuffix(filepath.ToSlash(file), "library/src/sub.km") {
		t.Fatalf("file=%s err=%v", file, err)
	}
	if _, err = graph.ResolveImport(filepath.Join(root, "index.km"), "pkg.test/utility"); err == nil {
		t.Fatal("root imported undeclared transitive dependency")
	}
	if _, err = graph.ResolveImport(filepath.Join(library, "index.km"), "pkg.test/utility"); err != nil {
		t.Fatal(err)
	}
	if err = graph.ValidateRelativeImport(filepath.Join(library, "index.km"), filepath.Join(utility, "index.km")); err == nil {
		t.Fatal("relative import escaped package")
	}
	if graph.SourceIdentity(file) != "pkg.test/library/src/sub.km" {
		t.Fatal("unstable source identity")
	}
	packageFiles(t, library, map[string]string{"index.km": "export function value():int{return 3;}"})
	if err := CheckDependencies(root); err != nil {
		t.Fatal("live source edit requires relock", err)
	}
	packageFiles(t, library, map[string]string{"kinmokusei.toml": packageManifest("pkg.test/library")})
	if err := CheckDependencies(root); err == nil || !strings.Contains(err.Error(), "does not match lock") {
		t.Fatalf("changed manifest err=%v", err)
	}
}

func TestSourcePackageManifestDiagnostics(t *testing.T) {
	t.Parallel()
	valid := packageManifest("pkg.test/library")
	for _, test := range []struct{ old, new, want string }{
		{"index.km", "../index.km", "entry"}, {"index.km", "/index.km", "entry"}, {"index.km", "C:/index.km", "entry"},
		{"backend = \"go\"", "backend = \"other\"", "backend"}, {"license = \"MIT\"", "license = \"\"", "license"},
		{"min-kinmokusei = \"0.4.0\"", "min-kinmokusei = \"latest\"", "min-kinmokusei"},
	} {
		_, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(strings.Replace(valid, test.old, test.new, 1)))
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: %v", test.new, err)
		}
	}
	manifest, err := ParseManifest(filepath.Join(t.TempDir(), "kinmokusei.toml"), []byte(valid+"[exports]\n\"sub\" = \"src/sub.km\"\n[dependencies]\n\"pkg.test/dep\" = \"v1.0.0\"\n[replace]\n\"pkg.test/dep\" = \"../dep\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := RenderManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(manifest.Path, text); err != nil {
		t.Fatal(err)
	}
}

func TestSourcePackageGraphFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, change, want string }{
		{"version", strings.Replace(packageManifest("pkg.test/lib"), "0.1.0", "0.2.0", 1), "version"},
		{"identity", packageManifest("pkg.test/wrong"), "identity"},
		{"minimum", strings.Replace(packageManifest("pkg.test/lib"), "0.4.0", "99.0.0", 1), "requires Kinmokusei"},
		{"target", packageManifest("pkg.test/lib") + "[target]\ngoos = \"impossible\"\n", "incompatible with target"},
		{"cycle", packageManifest("pkg.test/lib") + "[dependencies]\n\"app.test/main\" = \"v0.1.0\"\n", "cycle"},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "app")
			lib := filepath.Join(base, "lib")
			packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main") + "[dependencies]\n\"pkg.test/lib\" = \"v0.1.0\"\n[replace]\n\"pkg.test/lib\" = \"../lib\"\n\"app.test/main\" = \".\"\n", "index.km": ""})
			packageFiles(t, lib, map[string]string{"kinmokusei.toml": test.change, "index.km": ""})
			_, err := LockDependencies(root, true)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want=%s", err, test.want)
			}
		})
	}
}

func TestSourcePackageDownloadLockFetchAndIntegrity(t *testing.T) {
	root := t.TempDir()
	proxy := t.TempDir()
	cache := filepath.Join(t.TempDir(), "cache")
	t.Setenv("GOPROXY", localFileURL(proxy))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOMODCACHE", cache)
	t.Cleanup(func() {
		_ = filepath.WalkDir(cache, func(path string, entry os.DirEntry, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
	for _, module := range []string{"pkg.test/base", "pkg.test/library"} {
		manifest := packageManifest(module)
		if module == "pkg.test/library" {
			manifest += "[dependencies]\n\"pkg.test/base\" = \"v0.1.0\"\n"
		}
		writeProxyModule(t, proxy, module, "v0.1.0", "module "+module+"\n\ngo 1.23\n", map[string]string{"kinmokusei.toml": manifest, "index.km": "export function value():int{return 42;}", "LICENSE": "MIT fixture"})
	}
	packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main"), "index.km": ""})
	if err := AddAutoDependency(root, "pkg.test/library", "v0.1.0", "", false); err != nil {
		t.Fatal(err)
	}
	lock, err := ReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) != 2 || lock.Packages[1].Dependencies["pkg.test/base"] != "v0.1.0" {
		t.Fatalf("lock=%v", lock.Packages)
	}
	manifest, err := ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Imports["library"] = "pkg.test/library"
	if err := commitManifestAndLock(manifest, true); err != nil {
		t.Fatal(err)
	}
	aliasedLock, err := ReadLock(root)
	if err != nil || !reflect.DeepEqual(lock.Packages, aliasedLock.Packages) {
		t.Fatalf("alias changed canonical package versions/hashes: %v %v", aliasedLock.Packages, err)
	}
	checkAlias := func() {
		t.Helper()
		manifest, lock, err := ValidateLockedFiles(root)
		if err != nil {
			t.Fatal(err)
		}
		graph, err := ReadPackageGraph(manifest, lock)
		if err != nil {
			t.Fatal(err)
		}
		aliased, err := graph.ResolveImport(filepath.Join(root, "index.km"), "library")
		if err != nil || graph.SourceIdentity(aliased) != "pkg.test/library/index.km" {
			t.Fatalf("cached alias: %s %v", aliased, err)
		}
	}
	checkAlias()
	before, err := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckDependencies(root); err != nil {
		t.Fatal(err)
	}
	if err := AddAutoDependency(root, "missing.test/package", "v9.0.0", "", true); err == nil || !strings.Contains(err.Error(), "missing.test/package@v9.0.0") {
		t.Fatalf("cache miss=%v", err)
	}
	unchanged, _ := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
	if string(unchanged) != string(before) {
		t.Fatal("failed add changed lock")
	}
	newManifest := strings.Replace(packageManifest("pkg.test/library"), "0.1.0", "0.2.0", 1) + "[dependencies]\n\"pkg.test/base\" = \"v0.1.0\"\n"
	writeProxyModule(t, proxy, "pkg.test/library", "v0.2.0", "module pkg.test/library\n\ngo 1.23\n", map[string]string{"kinmokusei.toml": newManifest, "index.km": "export function value():int{return 43;}", "LICENSE": "MIT fixture"})
	if err := UpdateSourcePackages(root, "pkg.test/library", false); err != nil {
		t.Fatal(err)
	}
	updated, err := ReadLock(root)
	if err != nil || updated.Packages[1].Version != "v0.2.0" {
		t.Fatalf("upgrade=%v err=%v", updated.Packages, err)
	}
	checkAlias()
	if err := UpdateDependency(root, "pkg.test/library", "v0.1.0", true); err != nil {
		t.Fatal(err)
	}
	before, _ = os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
	if err := os.Remove(filepath.Join(root, ".kinmokusei", "deps", "go.mod")); err != nil {
		t.Fatal(err)
	}
	if err := FetchDependencies(root, true); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
	if string(before) != string(after) {
		t.Fatal("fetch rewrote lock")
	}
	checkAlias()
	file := filepath.Join(cache, "pkg.test", "library@v0.1.0", "index.km")
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckDependencies(root); err == nil || !strings.Contains(err.Error(), "does not match lock") {
		t.Fatalf("integrity error=%v", err)
	}
}

func TestSourcePackageVersionOrdering(t *testing.T) {
	t.Parallel()
	for _, pair := range [][2]string{{"0.4.0", "0.4.1"}, {"1.9.0", "1.10.0"}, {"1.0.0-alpha.9", "1.0.0-alpha.10"}, {"1.0.0-alpha", "1.0.0"}, {"1.0.0-1", "1.0.0-alpha"}, {"1.0.0-alpha", "1.0.0-alpha.1"}} {
		if comparePackageVersions(pair[0], pair[1]) >= 0 || comparePackageVersions(pair[1], pair[0]) <= 0 {
			t.Fatalf("order=%v", pair)
		}
	}
	if comparePackageVersions("v1.23.0+build", "1.23") != 0 {
		t.Fatal("equal version")
	}
}
