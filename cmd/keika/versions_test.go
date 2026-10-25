package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/selfupdate"
)

func TestVersionsDispatchHelp(t *testing.T) {
	for _, args := range [][]string{{"versions", "--help"}, {"versions", "list", "--help"}, {"use", "versions", "--help"}} {
		status, _, stderr := captureRun(t, args...)
		if status != 0 || !strings.Contains(stderr, "versions") {
			t.Fatalf("%v: %d %q", args, status, stderr)
		}
	}
}

func TestVersionsLocalAndRemote(t *testing.T) {
	for _, test := range []struct {
		args        []string
		remote      bool
		want        string
		empty, fail bool
		status      int
	}{
		{nil, false, "v0.4.10\t123\tmanaged, active, protected", false, false, 0},
		{[]string{"list"}, false, "No retained compiler versions.", true, false, 0},
		{[]string{"--remote"}, true, "v0.4.10\nv0.4.9", false, false, 0},
		{[]string{"list", "--remote"}, true, "No official stable release bundles", true, false, 0},
		{[]string{"--remote"}, true, "metadata unavailable", false, true, 1},
		{nil, false, "metadata unavailable", false, true, 1},
	} {
		localCalls, remoteCalls := 0, 0
		status, stdout, stderr := captureCommand(t, func() int {
			return runVersionsWith(test.args, func(ctx context.Context, options selfupdate.Options) ([]selfupdate.InstalledVersion, error) {
				localCalls++
				if options.CurrentVersion != product.VersionString() {
					t.Fatal(options)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("no deadline")
				}
				if test.fail {
					return nil, errors.New("metadata unavailable")
				}
				if test.empty {
					return nil, nil
				}
				return []selfupdate.InstalledVersion{{Version: "v0.4.10", Bytes: 123, Status: "managed", Active: true, Protected: true}, {Version: "v0.4.9", Bytes: -1, Status: "invalid"}}, nil
			}, func(ctx context.Context) ([]string, error) {
				remoteCalls++
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("no deadline")
				}
				if test.fail {
					return nil, errors.New("metadata unavailable")
				}
				if test.empty {
					return nil, nil
				}
				return []string{"v0.4.10", "v0.4.9"}, nil
			})
		})
		if status != test.status || !strings.Contains(stdout+stderr, test.want) || test.remote && (remoteCalls != 1 || localCalls != 0) || !test.remote && (localCalls != 1 || remoteCalls != 0) {
			t.Fatalf("%v: status=%d local=%d remote=%d stdout=%q stderr=%q", test.args, status, localCalls, remoteCalls, stdout, stderr)
		}
		if status != 0 && stdout != "" {
			t.Fatal("error printed a partial listing", stdout)
		}
	}
}

func TestVersionsInvalidArgumentsDoNoWork(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"list", "extra"}, {"--unknown"}, {"--remote", "extra"}, {"list", "--unknown"}} {
		status, stdout, _ := captureCommand(t, func() int {
			return runVersionsWith(args, func(context.Context, selfupdate.Options) ([]selfupdate.InstalledVersion, error) {
				t.Fatal("invalid arguments inspected local storage")
				return nil, nil
			}, func(context.Context) ([]string, error) {
				t.Fatal("invalid arguments accessed network")
				return nil, nil
			})
		})
		if status != 2 || stdout != "" {
			t.Fatalf("%v: %d %q", args, status, stdout)
		}
	}
}
