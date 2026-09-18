package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/puffball1567/kinmokusei/internal/project"
)

func runNew(args []string) int {
	if len(args) == 0 || args[0] != "app" && args[0] != "library" {
		fmt.Fprintln(os.Stderr, "new requires a template: app or library")
		return 2
	}
	kind := args[0]
	flags := flag.NewFlagSet("new "+kind, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	module := flags.String("module", "", "module path; defaults to example.com/<name>")
	name := flags.String("name", "", "project name; defaults to destination directory name")
	license := flags.String("license", "", "library license identifier; defaults to UNLICENSED")
	if err := flags.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "new "+kind+" requires exactly one destination directory; options precede the directory")
		return 2
	}
	if kind != "library" && *license != "" {
		fmt.Fprintln(os.Stderr, "--license applies only to the library template")
		return 2
	}
	if err := project.NewProject(kind, flags.Arg(0), project.NewProjectOptions{Name: *name, Module: *module, License: *license}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "created %s project in %s\n", kind, flags.Arg(0))
	return 0
}
