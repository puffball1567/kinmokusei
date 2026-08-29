package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"ontama.local/ontama/internal/compiler"
	"ontama.local/ontama/internal/diagnostic"
	"ontama.local/ontama/internal/lsp"
	"ontama.local/ontama/internal/product"
	"ontama.local/ontama/internal/project"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:])
	case "build":
		return runBuild(args[1:])
	case "run":
		return runGenerated(args[1:])
	case "emit-go":
		return runEmitGo(args[1:])
	case "emit-c-abi":
		return runEmitCABI(args[1:])
	case "ffi":
		return runFFI(args[1:])
	case "abi":
		return runABI(args[1:])
	case "interop":
		return runInterop(args[1:])
	case "lsp":
		return runLSP(args[1:])
	case "deps":
		return runDeps(args[1:])
	case "install":
		return runInstall(args[1:])
	case "target":
		return runTarget(args[1:])
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage()
		return 2
	}
}

func runInstall(args []string) int {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	goModule := flags.Bool("go-module", false, "add a versioned Go module dependency")
	offline := flags.Bool("offline", false, "resolve only from the existing Go module cache")
	replacement := flags.String("replace", "", "replace the Go module with a project-relative local directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*goModule {
		fmt.Fprintln(os.Stderr, "install currently requires --go-module; OnsenTamago source package installation is not implemented")
		return 2
	}
	arguments := flags.Args()
	if len(arguments) < 1 || len(arguments) > 2 {
		fmt.Fprintln(os.Stderr, "install --go-module requires <module>@<version> and accepts one optional project directory")
		return 2
	}
	root, ok := dependencyRoot(arguments[1:])
	if !ok {
		return 2
	}
	path, version, err := project.ParseDependencyArgument(arguments[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err = project.AddDependency(root, path, version, *replacement, *offline); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runTarget(args []string) int {
	flags := flag.NewFlagSet("target", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	root, ok := dependencyRoot(flags.Args())
	if !ok {
		return 2
	}
	if err := project.CheckDependencies(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_, lock, err := project.ValidateLockedFiles(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cgo := 0
	if lock.Target.CGOEnabled {
		cgo = 1
	}
	fmt.Fprintf(os.Stdout, "GOOS=%s\nGOARCH=%s\nCGO_ENABLED=%d\nTAGS=%s\n", lock.Target.GOOS, lock.Target.GOARCH, cgo, strings.Join(lock.Target.Tags, ","))
	return 0
}

func runDeps(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "deps requires a subcommand: lock, check, add, remove, update or licenses")
		return 2
	}
	switch args[0] {
	case "lock":
		flags := flag.NewFlagSet("deps lock", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		offline := flags.Bool("offline", false, "resolve only from the existing Go module cache")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		root, ok := dependencyRoot(flags.Args())
		if !ok {
			return 2
		}
		if _, err := project.LockDependencies(root, *offline); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "check":
		flags := flag.NewFlagSet("deps check", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		root, ok := dependencyRoot(flags.Args())
		if !ok {
			return 2
		}
		if err := project.CheckDependencies(root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "add":
		flags := flag.NewFlagSet("deps add", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		offline := flags.Bool("offline", false, "resolve only from the existing Go module cache")
		replacement := flags.String("replace", "", "replace the module with a project-relative local directory")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		dependency, root, ok := dependencyAndRoot("add", flags.Args())
		if !ok {
			return 2
		}
		path, version, err := project.ParseDependencyArgument(dependency)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if err = project.AddDependency(root, path, version, *replacement, *offline); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "remove":
		flags := flag.NewFlagSet("deps remove", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		offline := flags.Bool("offline", false, "resolve only from the existing Go module cache")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		dependency, root, ok := dependencyAndRoot("remove", flags.Args())
		if !ok {
			return 2
		}
		if err := project.RemoveDependency(root, dependency, *offline); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "update":
		flags := flag.NewFlagSet("deps update", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		offline := flags.Bool("offline", false, "resolve only from the existing Go module cache")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		dependency, root, ok := dependencyAndRoot("update", flags.Args())
		if !ok {
			return 2
		}
		path, version, err := project.ParseDependencyArgument(dependency)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if err = project.UpdateDependency(root, path, version, *offline); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "licenses":
		flags := flag.NewFlagSet("deps licenses", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		strict := flags.Bool("strict", false, "fail when a module has no detected license file")
		if err := flags.Parse(args[1:]); err != nil {
			return 2
		}
		root, ok := dependencyRoot(flags.Args())
		if !ok {
			return 2
		}
		licenses, err := project.DependencyLicenses(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		missing := 0
		for _, module := range licenses {
			identity := module.Path
			if module.Version != "" {
				identity += "@" + module.Version
			}
			if len(module.Files) == 0 {
				fmt.Fprintf(os.Stdout, "%s\tunknown\n", identity)
				missing++
				continue
			}
			for _, license := range module.Files {
				fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", identity, license.Path, license.SHA256)
			}
		}
		if *strict && missing != 0 {
			fmt.Fprintf(os.Stderr, "%d Go module(s) have no detected top-level LICENSE, LICENCE or COPYING file\n", missing)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown deps subcommand %q; expected lock, check, add, remove, update or licenses\n", args[0])
		return 2
	}
}

func dependencyAndRoot(command string, arguments []string) (string, string, bool) {
	if len(arguments) < 1 || len(arguments) > 2 {
		fmt.Fprintf(os.Stderr, "deps %s requires a dependency and accepts one optional project directory\n", command)
		return "", "", false
	}
	rootArguments := arguments[1:]
	root, ok := dependencyRoot(rootArguments)
	return arguments[0], root, ok
}

func dependencyRoot(arguments []string) (string, bool) {
	if len(arguments) > 1 {
		fmt.Fprintln(os.Stderr, "dependency commands accept at most one project directory")
		return "", false
	}
	root := "."
	if len(arguments) == 1 {
		root = arguments[0]
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return "", false
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "project directory %q is not accessible\n", root)
		return "", false
	}
	return absolute, true
}

func runLSP(args []string) int {
	flags := flag.NewFlagSet("lsp", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	stdio := flags.Bool("stdio", false, "serve Language Server Protocol over standard input/output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || !*stdio {
		fmt.Fprintln(os.Stderr, "lsp requires --stdio and accepts no source arguments")
		return 2
	}
	if err := lsp.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runBuild(args []string) int {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	output := flags.String("o", product.DefaultExecutable, "write the executable to this path")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "build requires at least one %s source file\n", product.SourceExtension)
		return 2
	}
	target, hasTarget, err := lockedTargetForSources(flags.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	generatedDirectory, diagnostics, err := compiler.WriteGeneratedModule(flags.Args(), "main")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(diagnostics) != 0 {
		diagnostic.Write(os.Stderr, diagnostics)
		return 1
	}
	absoluteOutput, err := filepath.Abs(*output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	arguments := []string{"build"}
	if hasTarget {
		arguments = append(arguments, target.GoBuildFlags()...)
	}
	arguments = append(arguments, "-mod=readonly", "-buildvcs=false", "-o", absoluteOutput, ".")
	command := exec.Command("go", arguments...)
	command.Dir = generatedDirectory
	if hasTarget {
		command.Env = target.Environment(command.Environ())
	}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err = command.Run(); err != nil {
		return 1
	}
	return 0
}

func runGenerated(args []string) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "run requires at least one %s source file\n", product.SourceExtension)
		return 2
	}
	target, hasTarget, err := lockedTargetForSources(flags.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if hasTarget && !target.IsNative() {
		fmt.Fprintf(os.Stderr, "cannot run cross target %s/%s on this host; use build instead\n", target.GOOS, target.GOARCH)
		return 1
	}
	generatedDirectory, diagnostics, err := compiler.WriteGeneratedModule(flags.Args(), "main")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(diagnostics) != 0 {
		diagnostic.Write(os.Stderr, diagnostics)
		return 1
	}
	arguments := []string{"run"}
	if hasTarget {
		arguments = append(arguments, target.GoBuildFlags()...)
	}
	arguments = append(arguments, "-mod=readonly", "-buildvcs=false", ".")
	command := exec.Command("go", arguments...)
	command.Dir = generatedDirectory
	if hasTarget {
		command.Env = target.Environment(command.Environ())
	}
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err = command.Run(); err != nil {
		return 1
	}
	return 0
}

func runCheck(args []string) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "write a machine-readable diagnostic report as JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "check requires at least one %s source file\n", product.SourceExtension)
		return 2
	}
	if err := validateProjectDependencies(flags.Args()); err != nil {
		return finishCheck(*jsonOutput, nil, err)
	}
	result, err := compiler.CheckFiles(flags.Args())
	if err != nil {
		return finishCheck(*jsonOutput, nil, err)
	}
	return finishCheck(*jsonOutput, result.Diagnostics, nil)
}

type checkReport struct {
	Valid       bool              `json:"valid"`
	Diagnostics []checkDiagnostic `json:"diagnostics"`
	Error       string            `json:"error,omitempty"`
}

type checkDiagnostic struct {
	Message string        `json:"message"`
	Path    string        `json:"path,omitempty"`
	Start   checkPosition `json:"start"`
	End     checkPosition `json:"end"`
}

type checkPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
	Offset int `json:"offset"`
}

func finishCheck(jsonOutput bool, diagnostics []diagnostic.Diagnostic, checkErr error) int {
	valid := checkErr == nil && len(diagnostics) == 0
	if !jsonOutput {
		if checkErr != nil {
			fmt.Fprintln(os.Stderr, checkErr)
		} else if len(diagnostics) != 0 {
			diagnostic.Write(os.Stderr, diagnostics)
		}
		if valid {
			return 0
		}
		return 1
	}

	report := checkReport{Valid: valid, Diagnostics: make([]checkDiagnostic, 0, len(diagnostics))}
	if checkErr != nil {
		report.Error = checkErr.Error()
	}
	for _, item := range diagnostics {
		report.Diagnostics = append(report.Diagnostics, checkDiagnostic{
			Message: item.Message,
			Path:    item.Span.Path,
			Start: checkPosition{
				Line: item.Span.Start.Line, Column: item.Span.Start.Column, Offset: item.Span.Start.Offset,
			},
			End: checkPosition{
				Line: item.Span.End.Line, Column: item.Span.End.Column, Offset: item.Span.End.Offset,
			},
		})
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	encoded = append(encoded, '\n')
	if _, err = os.Stdout.Write(encoded); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if valid {
		return 0
	}
	return 1
}

func runEmitGo(args []string) int {
	flags := flag.NewFlagSet("emit-go", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	output := flags.String("o", "", "write generated Go to this file instead of stdout")
	packageName := flags.String("package", "main", "generated Go package name")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "emit-go requires at least one %s source file\n", product.SourceExtension)
		return 2
	}
	if err := validateProjectDependencies(flags.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	generated, diagnostics, err := compiler.EmitGo(flags.Args(), *packageName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(diagnostics) != 0 {
		diagnostic.Write(os.Stderr, diagnostics)
		return 1
	}
	if *output == "" {
		_, err = os.Stdout.Write(generated)
	} else {
		err = os.WriteFile(*output, generated, 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runEmitCABI(args []string) int {
	flags := flag.NewFlagSet("emit-c-abi", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	output := flags.String("o", "", "write generated.go, generated_cabi.go, ontama_abi.h, and ontama_abi.json to this directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *output == "" {
		fmt.Fprintln(os.Stderr, "emit-c-abi requires -o with an output directory")
		return 2
	}
	if flags.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "emit-c-abi requires at least one %s source file\n", product.SourceExtension)
		return 2
	}
	if err := validateProjectDependencies(flags.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	artifacts, diagnostics, err := compiler.EmitCABI(flags.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(diagnostics) != 0 {
		diagnostic.Write(os.Stderr, diagnostics)
		return 1
	}
	if err = os.MkdirAll(*output, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for name, contents := range map[string][]byte{
		"generated.go": artifacts.GoSource, "generated_cabi.go": artifacts.Gateway,
		"ontama_abi.h": artifacts.Header, "ontama_abi.json": artifacts.Manifest,
	} {
		if err = os.WriteFile(filepath.Join(*output, name), contents, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}

func runFFI(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "ffi requires a subcommand: generate")
		return 2
	}
	if args[0] != "generate" {
		fmt.Fprintf(os.Stderr, "unknown ffi subcommand %q; expected generate\n", args[0])
		return 2
	}
	flags := flag.NewFlagSet("ffi generate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	manifestPath := flags.String("manifest", "", "read a checked incoming C FFI manifest")
	output := flags.String("o", "", "write generated_ffi.go to this directory")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "ffi generate accepts no positional arguments")
		return 2
	}
	if *manifestPath == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "ffi generate requires --manifest and -o")
		return 2
	}
	manifest, err := os.ReadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read C FFI manifest: %v\n", err)
		return 1
	}
	artifacts, err := compiler.GenerateCFFI(manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err = os.MkdirAll(*output, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err = os.WriteFile(filepath.Join(*output, "generated_ffi.go"), artifacts.Source, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runABI(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "abi requires a subcommand: check")
		return 2
	}
	if args[0] != "check" {
		fmt.Fprintf(os.Stderr, "unknown abi subcommand %q; expected check\n", args[0])
		return 2
	}
	flags := flag.NewFlagSet("abi check", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baselinePath := flags.String("baseline", "", "compare against this previously generated ontama_abi.json")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if *baselinePath == "" {
		fmt.Fprintln(os.Stderr, "abi check requires --baseline with an ontama_abi.json file")
		return 2
	}
	if flags.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "abi check requires at least one %s source file\n", product.SourceExtension)
		return 2
	}
	baseline, err := os.ReadFile(*baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read baseline C ABI manifest: %v\n", err)
		return 1
	}
	if err = validateProjectDependencies(flags.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	artifacts, diagnostics, err := compiler.EmitCABI(flags.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(diagnostics) != 0 {
		diagnostic.Write(os.Stderr, diagnostics)
		return 1
	}
	compatibility, err := compiler.CompareCABIManifests(baseline, artifacts.Manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !compatibility.Compatible() {
		for _, change := range compatibility.BreakingChanges {
			fmt.Fprintln(os.Stderr, "breaking C ABI change: "+change)
		}
		return 1
	}
	if compatibility.ExactFingerprint {
		fmt.Fprintf(os.Stdout, "C ABI compatible: exact fingerprint %s\n", artifacts.Fingerprint)
	} else {
		fmt.Fprintf(os.Stdout, "C ABI compatible: added symbols %s\n", strings.Join(compatibility.Additions, ", "))
	}
	return 0
}

func runInterop(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "interop requires a subcommand: audit")
		return 2
	}
	if args[0] != "audit" {
		fmt.Fprintf(os.Stderr, "unknown interop subcommand %q; expected audit\n", args[0])
		return 2
	}
	flags := flag.NewFlagSet("interop audit", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	standardLibrary := flags.Bool("stdlib", false, "audit all public packages reported by the active Go standard library")
	jsonOutput := flags.Bool("json", false, "write the complete machine-readable report as JSON")
	allowIncomplete := flags.Bool("allow-incomplete", false, "return success even when one or more packages cannot be loaded")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	paths := append([]string(nil), flags.Args()...)
	if *standardLibrary {
		standardPaths, err := compiler.StandardGoPackagePaths()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		paths = append(paths, standardPaths...)
	}
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "interop audit requires --stdlib or at least one Go package import path")
		return 2
	}
	report := compiler.AuditGoInteropPackages(paths, nil)
	if *jsonOutput {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		encoded = append(encoded, '\n')
		if _, err = os.Stdout.Write(encoded); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else {
		writeGoInteropAuditText(report)
	}
	if len(report.FailedPackages) != 0 && !*allowIncomplete {
		fmt.Fprintf(os.Stderr, "interop audit incomplete: %d of %d package(s) could not be loaded\n", len(report.FailedPackages), report.AttemptedPackages)
		return 1
	}
	return 0
}

func writeGoInteropAuditText(report compiler.GoInteropAuditReport) {
	fmt.Fprintf(os.Stdout, "Go interop API audit (%s): %d/%d packages loaded\n", report.GoVersion, report.LoadedPackages, report.AttemptedPackages)
	writeGoInteropAuditCount("callables", report.Callables)
	writeGoInteropAuditCount("values", report.Values)
	writeGoInteropAuditCount("types", report.Types)
	writeGoInteropAuditCount("overall", report.Overall)
	if len(report.Reasons) != 0 {
		type reasonCount struct {
			reason string
			count  int
		}
		reasons := make([]reasonCount, 0, len(report.Reasons))
		for reason, count := range report.Reasons {
			reasons = append(reasons, reasonCount{reason: reason, count: count})
		}
		sort.Slice(reasons, func(i, j int) bool {
			if reasons[i].count != reasons[j].count {
				return reasons[i].count > reasons[j].count
			}
			return reasons[i].reason < reasons[j].reason
		})
		fmt.Fprintln(os.Stdout, "reasons:")
		limit := len(reasons)
		if limit > 10 {
			limit = 10
		}
		for _, reason := range reasons[:limit] {
			fmt.Fprintf(os.Stdout, "  %d\t%s\n", reason.count, reason.reason)
		}
		if remaining := len(reasons) - limit; remaining != 0 {
			fmt.Fprintf(os.Stdout, "  ... %d more reason(s); use --json for all details\n", remaining)
		}
	}
	if len(report.FailedPackages) != 0 {
		fmt.Fprintln(os.Stdout, "failed packages:")
		limit := len(report.FailedPackages)
		if limit > 10 {
			limit = 10
		}
		for _, failure := range report.FailedPackages[:limit] {
			fmt.Fprintf(os.Stdout, "  %s: %s\n", failure.Path, failure.Error)
		}
		if remaining := len(report.FailedPackages) - limit; remaining != 0 {
			fmt.Fprintf(os.Stdout, "  ... %d more failure(s); use --json for all details\n", remaining)
		}
	}
}

func writeGoInteropAuditCount(label string, count compiler.GoInteropAuditCount) {
	safePercentage, availablePercentage := 0.0, 0.0
	if count.Total != 0 {
		safePercentage = float64(count.Supported) * 100 / float64(count.Total)
		availablePercentage = float64(count.Supported+count.RequiresUnsafe) * 100 / float64(count.Total)
	}
	fmt.Fprintf(os.Stdout, "%s: safe %d/%d (%.2f%%), with unsafe opt-in %d/%d (%.2f%%), unsupported %d\n",
		label, count.Supported, count.Total, safePercentage,
		count.Supported+count.RequiresUnsafe, count.Total, availablePercentage, count.Unsupported)
}

func validateProjectDependencies(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	root, found, err := project.FindRoot(paths[0])
	if err != nil || !found {
		return err
	}
	return project.CheckDependencies(root)
}

func lockedTargetForSources(paths []string) (project.BuildTarget, bool, error) {
	if len(paths) == 0 {
		return project.BuildTarget{}, false, nil
	}
	root, found, err := project.FindRoot(paths[0])
	if err != nil || !found {
		return project.BuildTarget{}, false, err
	}
	if err = project.CheckDependencies(root); err != nil {
		return project.BuildTarget{}, false, err
	}
	_, lock, err := project.ValidateLockedFiles(root)
	if err != nil {
		return project.BuildTarget{}, false, err
	}
	return lock.Target, true, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s <check|build|run|emit-go|emit-c-abi|ffi|abi|interop|lsp|install|deps|target> [options] <source%s>...\n", product.CommandName, product.SourceExtension)
}
