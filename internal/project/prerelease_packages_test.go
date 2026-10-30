package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrereleasePackagesDownloadLockAndOfflineCache(t *testing.T) {
	root, proxy := t.TempDir(), t.TempDir()
	cache := filepath.Join(t.TempDir(), "cache")
	t.Setenv("GOPROXY", localFileURL(proxy))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOMODCACHE", cache)
	t.Cleanup(func() {
		_ = filepath.WalkDir(cache, func(file string, _ os.DirEntry, err error) error {
			if err == nil {
				_ = os.Chmod(file, 0o700)
			}
			return nil
		})
	})
	manifestFor := func(module, version string) string {
		return strings.Replace(packageManifest(module), `version = "0.1.0"`, `version = "`+version+`"`, 1)
	}
	baseVersion := "v0.1.0-rc-1"
	libraryVersion := "v1.0.0-alpha.1-2"
	writeProxyModule(t, proxy, "native.test/helper", baseVersion, "module native.test/helper\n\ngo 1.23\n", map[string]string{"helper.go": "package helper\nconst N=7\n", "LICENSE": "MIT fixture"})
	writeProxyModule(t, proxy, "pkg.test/base", baseVersion, "module pkg.test/base\n\ngo 1.23\n", map[string]string{
		"kinmokusei.toml": manifestFor("pkg.test/base", strings.TrimPrefix(baseVersion, "v")), "index.km": "export function value():int{return 3;}", "LICENSE": "MIT fixture",
	})
	writeProxyModule(t, proxy, "pkg.test/library", libraryVersion, "module pkg.test/library\n\ngo 1.23\n", map[string]string{
		"kinmokusei.toml": manifestFor("pkg.test/library", strings.TrimPrefix(libraryVersion, "v")) + "[dependencies]\n\"pkg.test/base\"=\"" + baseVersion + "\"\n[go.dependencies]\n\"native.test/helper\"=\"" + baseVersion + "\"\n",
		"index.km":        `import {value} from "pkg.test/base"; import go helper from "native.test/helper"; export function read():int{return value()+helper.N;}`, "LICENSE": "MIT fixture",
	})
	packageFiles(t, root, map[string]string{"kinmokusei.toml": packageManifest("app.test/main"), "index.km": ""})
	if err := AddAutoDependency(root, "pkg.test/library", libraryVersion, "", false); err != nil {
		t.Fatal(err)
	}
	lock, err := ReadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) != 2 || lock.Packages[0].Version != baseVersion || lock.Packages[1].Version != libraryVersion {
		t.Fatalf("packages=%v", lock.Packages)
	}
	before, err := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
	if err != nil {
		t.Fatal(err)
	}
	foundGo := false
	for _, mod := range lock.Modules {
		if mod.Path == "native.test/helper" && mod.Version == baseVersion {
			foundGo = true
		}
	}
	if !foundGo {
		t.Fatalf("Go prerelease not locked: %v", lock.Modules)
	}
	validVersion := lock.Packages[0].Version
	lock.Packages[0].Version = "v0.1.0+build-1"
	if err := lock.Validate(); err == nil {
		t.Fatal("lock accepted a noncanonical source version")
	}
	lock.Packages[0].Version = validVersion
	t.Setenv("GOPROXY", "off")
	for _, check := range []func() error{func() error { return CheckDependencies(root) }, func() error { return FetchDependencies(root, true) }} {
		if err := check(); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(filepath.Join(root, "kinmokusei.lock"))
	if err != nil || string(before) != string(after) {
		t.Fatalf("lock changed: %v", err)
	}
}
