package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/selfupdate"
)

func runUse(args []string) int { return runUseWith(args, selfupdate.Use) }

func runUseWith(args []string, selectVersion func(context.Context, selfupdate.UseOptions) (selfupdate.Result, error)) int {
	if len(args) > 0 && args[0] == "versions" {
		return runVersions(args[1:])
	}
	flags := flag.NewFlagSet("use", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: keika use [--offline] vX.Y.Z (or keika-use [--offline] vX.Y.Z)")
		fmt.Fprintln(os.Stderr, "A stable numeric version X.Y.Z is also accepted; use versions for retained/official listings.")
		flags.PrintDefaults()
	}
	offline := flags.Bool("offline", false, "select only an already verified local release; do not access the network")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	version := flags.Arg(0)
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	result, err := selectVersion(ctx, selfupdate.UseOptions{Options: selfupdate.Options{CurrentVersion: product.VersionString(), Version: version}, Offline: *offline})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Using keika %s\n", result.Version)
	if result.ManagerPath != "" {
		fmt.Fprintf(os.Stdout, "Switch again, including from older compilers: %s vX.Y.Z\n", result.ManagerPath)
	}
	return 0
}
