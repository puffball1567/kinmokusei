package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/selfupdate"
)

func versionsUsage() {
	fmt.Fprintln(os.Stderr, "Usage: keika versions [list] [--remote]")
	fmt.Fprintln(os.Stderr, "Default: read-only retained versions. --remote: official stable release metadata for this platform.")
	fmt.Fprintln(os.Stderr, "Independent manager: keika-use versions [list] [--remote]")
}

func runVersions(args []string) int {
	return runVersionsWith(args, selfupdate.ListVersions, selfupdate.ListAvailableVersions)
}

func runVersionsWith(args []string, list func(context.Context, selfupdate.Options) ([]selfupdate.InstalledVersion, error), remote func(context.Context) ([]string, error)) int {
	if len(args) > 0 && args[0] == "list" {
		args = args[1:]
	}
	flags := flag.NewFlagSet("versions", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = versionsUsage
	available := flags.Bool("remote", false, "fetch official stable releases with compiler/checksum assets for this platform")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if *available {
		versions, err := remote(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if len(versions) == 0 {
			fmt.Fprintln(os.Stdout, "No official stable release bundles for this platform.")
			return 0
		}
		fmt.Fprintln(os.Stdout, "VERSION")
		for _, version := range versions {
			fmt.Fprintln(os.Stdout, version)
		}
		return 0
	}
	entries, err := list(ctx, selfupdate.Options{CurrentVersion: product.VersionString()})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stdout, "No retained compiler versions.")
		return 0
	}
	fmt.Fprintln(os.Stdout, "VERSION\tBYTES\tSTATUS")
	for _, entry := range entries {
		status := []string{entry.Status}
		if entry.Active {
			status = append(status, "active")
		}
		if entry.Protected {
			status = append(status, "protected")
		}
		size := "unknown"
		if entry.Bytes >= 0 {
			size = strconv.FormatInt(entry.Bytes, 10)
		}
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", entry.Version, size, strings.Join(status, ", "))
	}
	return 0
}
