package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/selfupdate"
)

func runUpdate(args []string) int {
	return runUpdateWith(args, selfupdate.Run)
}

func runUpdateWith(args []string, update func(context.Context, selfupdate.Options) (selfupdate.Result, error)) int {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	version := flags.String("version", "", "select a stable official release tag (vX.Y.Z)")
	check := flags.Bool("check", false, "check for an update without changing the installation")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "update accepts no positional arguments; use --version vX.Y.Z")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result, err := update(ctx, selfupdate.Options{CurrentVersion: product.VersionString(), Version: *version, Check: *check})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	switch {
	case result.Updated:
		fmt.Fprintf(os.Stdout, "Updated keika %s -> %s\n", result.CurrentVersion, result.Version)
	case result.Available:
		next := "keika update"
		if *version != "" || result.RequireVersion {
			next += " --version " + result.Version
		}
		fmt.Fprintf(os.Stdout, "keika %s is available (current: %s); run %s\n", result.Version, result.CurrentVersion, next)
	default:
		fmt.Fprintf(os.Stdout, "keika %s is up to date\n", result.CurrentVersion)
	}
	return 0
}
