package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func TestPrereleaseLocalPackageCLI(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "app")
	library := filepath.Join(base, "library")
	for _, directory := range []string{root, library} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	app := `[project]
name="app"
version="0.1.0"
go-module="app.test/main"
go-version="1.23"
`
	if err := os.WriteFile(filepath.Join(root, "kinmokusei.toml"), []byte(app), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.km"), []byte(`import {value} from "library"; import go fmt from "fmt"; function main():void{fmt.Println(value());}`), 0o644); err != nil {
		t.Fatal(err)
	}
	setVersion := func(version string) {
		t.Helper()
		manifest := fmt.Sprintf("[project]\nname=\"library\"\nversion=%q\ngo-module=\"pkg.test/library\"\ngo-version=\"1.23\"\n[package]\nentry=\"index.km\"\nmin-kinmokusei=\"0.4.0\"\nbackend=\"go\"\nlicense=\"MIT\"\n", version)
		if err := os.WriteFile(filepath.Join(library, "kinmokusei.toml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(library, "index.km"), []byte(`export function value():int{return 42;}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "LICENSE"), []byte("MIT fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	})
	t.Setenv("GOPROXY", "off")
	setVersion("1.0.0-rc-1")
	for _, args := range [][]string{
		{"deps", "add", "--offline", "--replace", "../library", "pkg.test/library@v1.0.0-rc-1"},
		{"check"}, {"deps", "check"},
	} {
		if status, _, stderr := captureRun(t, args...); status != 0 {
			t.Fatalf("%v: %d %s", args, status, stderr)
		}
	}
	if status, stdout, stderr := captureRun(t, "run"); status != 0 || strings.TrimSpace(stdout) != "42" || stderr != "" {
		t.Fatalf("run=%d %q %s", status, stdout, stderr)
	}
	setVersion("1.0.0-rc-2")
	if status, _, stderr := captureRun(t, "deps", "update", "--offline", "pkg.test/library@v1.0.0-rc-2"); status != 0 {
		t.Fatalf("upgrade: %s", stderr)
	}
	setVersion("1.0.0-rc-1")
	if status, _, stderr := captureRun(t, "deps", "update", "--offline", "pkg.test/library"); status != 0 {
		t.Fatalf("local selection: %s", stderr)
	}
	lock, err := project.ReadLock(root)
	if err != nil || len(lock.Packages) != 1 || lock.Packages[0].Version != "v1.0.0-rc-1" {
		t.Fatalf("lock=%v err=%v", lock.Packages, err)
	}
	beforeManifest, _ := os.ReadFile("kinmokusei.toml")
	beforeLock, _ := os.ReadFile("kinmokusei.lock")
	if status, _, stderr := captureRun(t, "deps", "update", "--offline", "pkg.test/library@v1.0.0+build-1"); status == 0 || !strings.Contains(stderr, "canonical") {
		t.Fatalf("metadata status=%d %s", status, stderr)
	}
	afterManifest, _ := os.ReadFile("kinmokusei.toml")
	afterLock, _ := os.ReadFile("kinmokusei.lock")
	if string(beforeManifest) != string(afterManifest) || string(beforeLock) != string(afterLock) {
		t.Fatal("rejected metadata modified dependency state")
	}
}
