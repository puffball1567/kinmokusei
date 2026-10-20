package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/selfupdate"
)

func TestUseCommand(t *testing.T) {
	for _, test := range []struct {
		args   []string
		status int
		called bool
		want   string
	}{
		{nil, 2, false, "Usage: keika use"},
		{[]string{"--help"}, 0, false, "already verified"},
		{[]string{"--unknown"}, 2, false, "flag provided but not defined"},
		{[]string{"v0.4.8", "extra"}, 2, false, "Usage: keika use"},
		{[]string{"v0.4.8"}, 0, true, "Using keika v0.4.8"},
		{[]string{"--offline", "v0.4.8"}, 0, true, "Switch again, including from older compilers: manager vX.Y.Z"},
	} {
		called := false
		status, stdout, stderr := captureCommand(t, func() int {
			return runUseWith(test.args, func(ctx context.Context, options selfupdate.UseOptions) (selfupdate.Result, error) {
				called = true
				if options.Version != "v0.4.8" || options.CurrentVersion != product.VersionString() || options.Offline != (test.args[0] == "--offline") {
					t.Fatalf("options=%+v", options)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing deadline")
				}
				return selfupdate.Result{Version: options.Version, ManagerPath: "manager"}, nil
			})
		})
		if called != test.called || status != test.status || !strings.Contains(stdout+stderr, test.want) {
			t.Fatalf("args=%v status=%d stdout=%q stderr=%q", test.args, status, stdout, stderr)
		}
		if test.called && stderr != "" {
			t.Fatal(stderr)
		}
	}
	status, stdout, stderr := captureCommand(t, func() int {
		return runUseWith([]string{"v0.4.8"}, func(context.Context, selfupdate.UseOptions) (selfupdate.Result, error) {
			return selfupdate.Result{}, errors.New("corrupt cached release")
		})
	})
	if status != 1 || stdout != "" || !strings.Contains(stderr, "corrupt cached release") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	status, _, stderr = captureRun(t, "use", "--help")
	if status != 0 || !strings.Contains(stderr, "keika-use") {
		t.Fatalf("dispatch status=%d stderr=%q", status, stderr)
	}
}
