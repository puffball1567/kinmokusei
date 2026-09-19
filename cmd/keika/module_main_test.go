package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDependencyMainDoesNotBecomeApplicationEntry(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "entry.km")
	library := filepath.Join(root, "library.km")
	if err := os.WriteFile(library, []byte(`import go {Println} from "fmt";const main=()=>{Println("dependency-main");};export function value():int{return 42;}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`import {value} from "./library";function unused():int{return value();}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, out, stderr := captureRun(t, "run", entry); status == 0 || strings.Contains(out, "dependency-main") || !strings.Contains(stderr, "main") {
		t.Fatalf("dependency main ran as application entry: status=%d out=%q err=%q", status, out, stderr)
	}
	if err := os.WriteFile(entry, []byte(`import {value} from "./library";import go {Println} from "fmt";const main=()=>{Println(value());};`), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, out, stderr := captureRun(t, "run", entry); status != 0 || strings.TrimSpace(out) != "42" || stderr != "" {
		t.Fatalf("application entry: status=%d out=%q err=%q", status, out, stderr)
	}
}
