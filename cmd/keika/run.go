package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/project"
)

// go run maps every nonzero program exit to 1. Build a private executable and
// launch it directly so scripts observe the program's own exit status.
func runGeneratedModule(directory string, target project.BuildTarget, hasTarget bool) int {
	temporary, err := os.MkdirTemp("", product.CommandName+"-run-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(temporary)
	name := "program"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(temporary, name)
	environment := project.OfflineEnvironment(os.Environ())
	arguments := []string{"build"}
	if hasTarget {
		arguments = append(arguments, target.GoBuildFlags()...)
		environment = target.Environment(environment)
	}
	arguments = append(arguments, "-mod=readonly", "-buildvcs=false", "-o", executable, ".")
	build := exec.Command("go", arguments...)
	build.Dir, build.Env = directory, environment
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Go build failed: %v\n", err)
		return 1
	}
	command := exec.Command(executable)
	command.Dir, command.Env = directory, environment
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() >= 0 {
			return exit.ExitCode()
		}
		// Launch failures and signal termination do not carry a portable
		// program exit code; retain the command failure status and diagnostic.
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
