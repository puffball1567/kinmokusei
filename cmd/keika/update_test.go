package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/selfupdate"
)

func TestUpdateUsage(t *testing.T) {
	for _, test := range []struct {
		args   []string
		status int
		want   string
	}{
		{[]string{"--help"}, 0, "select a stable official release"},
		{[]string{"-h"}, 0, "check for an update"},
		{[]string{"--unknown"}, 2, "flag provided but not defined"},
		{[]string{"--version"}, 2, "flag needs an argument"},
		{[]string{"v0.4.8"}, 2, "accepts no positional arguments"},
	} {
		status, stdout, stderr := captureCommand(t, func() int {
			return runUpdateWith(test.args, func(context.Context, selfupdate.Options) (selfupdate.Result, error) {
				t.Fatal("usage fetched an update")
				return selfupdate.Result{}, nil
			})
		})
		if status != test.status || stdout != "" || !strings.Contains(stderr, test.want) {
			t.Fatalf("args=%v status=%d stdout=%q stderr=%q", test.args, status, stdout, stderr)
		}
	}
	status, _, stderr := captureRun(t, "update", "--help")
	if status != 0 || !strings.Contains(stderr, "version") {
		t.Fatalf("dispatch status=%d stderr=%q", status, stderr)
	}
}

func TestUpdateCommandResults(t *testing.T) {
	for _, test := range []struct {
		name   string
		args   []string
		result selfupdate.Result
		err    error
		status int
		want   string
	}{
		{"updated", nil, selfupdate.Result{CurrentVersion: "v0.4.7", Version: "v0.4.8", Updated: true}, nil, 0, "Updated keika v0.4.7 -> v0.4.8"},
		{"available", []string{"--check", "--version", "v0.4.8"}, selfupdate.Result{CurrentVersion: "v0.4.7", Version: "v0.4.8", Available: true}, nil, 0, "v0.4.8 is available"},
		{"development check", []string{"--check"}, selfupdate.Result{CurrentVersion: "devel", Version: "v0.4.8", Available: true, RequireVersion: true}, nil, 0, "run keika update --version v0.4.8"},
		{"same", nil, selfupdate.Result{CurrentVersion: "v0.4.8", Version: "v0.4.8"}, nil, 0, "v0.4.8 is up to date"},
		{"failed", nil, selfupdate.Result{}, errors.New("checksum mismatch"), 1, "checksum mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			status, stdout, stderr := captureCommand(t, func() int {
				return runUpdateWith(test.args, func(ctx context.Context, options selfupdate.Options) (selfupdate.Result, error) {
					called = true
					if _, ok := ctx.Deadline(); !ok {
						t.Fatal("update has no deadline")
					}
					if options.CurrentVersion != product.VersionString() {
						t.Fatalf("current version=%q", options.CurrentVersion)
					}
					if test.name == "available" && (!options.Check || options.Version != "v0.4.8") {
						t.Fatalf("options=%+v", options)
					}
					return test.result, test.err
				})
			})
			if !called || status != test.status || !strings.Contains(stdout+stderr, test.want) {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
			}
			if test.err == nil && stderr != "" || test.err != nil && stdout != "" {
				t.Fatal("incorrect output stream")
			}
		})
	}
}
