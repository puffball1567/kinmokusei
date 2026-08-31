package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/puffball1567/onsentamago/internal/lexer"
	"github.com/puffball1567/onsentamago/internal/parser"
	"github.com/puffball1567/onsentamago/internal/product"
)

type listedModule struct {
	Path     string
	Version  string
	Sum      string
	GoModSum string
	Dir      string
	Main     bool
	Replace  *listedModule
}

type editedGoMod struct {
	Require []struct {
		Path    string
		Version string
	}
}

type ModuleLicenses struct {
	Path    string
	Version string
	Files   []LockedLicenseFile
}

func LockDependencies(root string, offline bool) (Lock, error) {
	manifest, err := ReadManifest(root)
	if err != nil {
		return Lock{}, err
	}
	if err = validateReplacementTargets(manifest); err != nil {
		return Lock{}, err
	}
	target, err := ResolveTarget(manifest.Target)
	if err != nil {
		return Lock{}, err
	}
	stateDirectory := filepath.Join(manifest.Root, product.StateDirectoryName)
	if err = os.MkdirAll(stateDirectory, 0o755); err != nil {
		return Lock{}, fmt.Errorf("cannot create dependency state directory: %w", err)
	}
	temporary, err := os.MkdirTemp(stateDirectory, ".deps-resolve-")
	if err != nil {
		return Lock{}, err
	}
	defer os.RemoveAll(temporary)
	goMod, err := RenderGoMod(manifest, temporary)
	if err != nil {
		return Lock{}, err
	}
	if err = os.WriteFile(filepath.Join(temporary, "go.mod"), goMod, 0o644); err != nil {
		return Lock{}, err
	}
	if err = writeDependencyProbe(manifest.Root, temporary); err != nil {
		return Lock{}, fmt.Errorf("cannot inspect project Go imports: %w", err)
	}
	if err = runGo(temporary, offline, target, "mod", "tidy"); err != nil {
		return Lock{}, fmt.Errorf("cannot resolve Go dependencies: %w", err)
	}
	requirements, err := readGoModRequirements(temporary, target)
	if err != nil {
		return Lock{}, fmt.Errorf("cannot read resolved Go requirements: %w", err)
	}
	goMod, err = RenderResolvedGoMod(manifest, requirements, temporary)
	if err != nil {
		return Lock{}, err
	}
	if err = os.WriteFile(filepath.Join(temporary, "go.mod"), goMod, 0o644); err != nil {
		return Lock{}, err
	}
	if err = runGo(temporary, offline, target, "mod", "download", "all"); err != nil {
		return Lock{}, fmt.Errorf("cannot materialize locked Go dependencies: %w", err)
	}
	modules, err := listModules(temporary, true, target)
	if err != nil {
		return Lock{}, fmt.Errorf("cannot lock Go module graph: %w", err)
	}
	locked, err := lockModules(manifest, modules)
	if err != nil {
		return Lock{}, err
	}
	if err = validateDirectDependencies(manifest, locked); err != nil {
		return Lock{}, err
	}
	goMod, err = os.ReadFile(filepath.Join(temporary, "go.mod"))
	if err != nil {
		return Lock{}, err
	}
	goSum, err := readOptional(filepath.Join(temporary, "go.sum"))
	if err != nil {
		return Lock{}, err
	}
	lock := manifest.NewLock(goMod, goSum, target, locked)
	dependencyDirectory := product.DependencyDirectory(manifest.Root)
	if err = os.MkdirAll(dependencyDirectory, 0o755); err != nil {
		return Lock{}, err
	}
	if err = writeAtomic(filepath.Join(dependencyDirectory, "go.mod"), goMod, 0o644); err != nil {
		return Lock{}, err
	}
	if err = writeAtomic(filepath.Join(dependencyDirectory, "go.sum"), goSum, 0o644); err != nil {
		return Lock{}, err
	}
	if err = WriteLock(manifest.Root, lock); err != nil {
		return Lock{}, err
	}
	return lock, nil
}

func ValidateLockedFiles(root string) (Manifest, Lock, error) {
	manifest, err := ReadManifest(root)
	if err != nil {
		return Manifest{}, Lock{}, err
	}
	lock, err := ReadLock(manifest.Root)
	if err != nil {
		if os.IsNotExist(unwrapPathError(err)) {
			return Manifest{}, Lock{}, fmt.Errorf("dependency lock is missing; run %s deps lock", product.CommandName)
		}
		return Manifest{}, Lock{}, err
	}
	if lock.ManifestHash != Hash(manifest.Contents) {
		return Manifest{}, Lock{}, fmt.Errorf("dependency lock does not match %s; run %s deps lock", product.ProjectFileName, product.CommandName)
	}
	if lock.GoVersion != manifest.Project.GoVersion {
		return Manifest{}, Lock{}, fmt.Errorf("dependency lock Go version %q does not match manifest %q", lock.GoVersion, manifest.Project.GoVersion)
	}
	target, err := ResolveTarget(manifest.Target)
	if err != nil {
		return Manifest{}, Lock{}, err
	}
	if !sameBuildTarget(lock.Target, target) {
		return Manifest{}, Lock{}, fmt.Errorf("dependency lock target %s/%s does not match resolved manifest target %s/%s; run %s deps lock", lock.Target.GOOS, lock.Target.GOARCH, target.GOOS, target.GOARCH, product.CommandName)
	}
	directory := product.DependencyDirectory(manifest.Root)
	goMod, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		return Manifest{}, Lock{}, fmt.Errorf("locked Go module is missing: %w; run %s deps lock", err, product.CommandName)
	}
	goSum, err := os.ReadFile(filepath.Join(directory, "go.sum"))
	if err != nil {
		return Manifest{}, Lock{}, fmt.Errorf("locked Go checksums are missing: %w; run %s deps lock", err, product.CommandName)
	}
	if Hash(goMod) != lock.GoModHash {
		return Manifest{}, Lock{}, fmt.Errorf("locked go.mod was modified; run %s deps lock", product.CommandName)
	}
	if Hash(goSum) != lock.GoSumHash {
		return Manifest{}, Lock{}, fmt.Errorf("locked go.sum was modified; run %s deps lock", product.CommandName)
	}
	return manifest, lock, nil
}

func writeDependencyProbe(root, directory string) error {
	imports := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && entry.Name() == product.StateDirectoryName {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != product.SourceExtension {
			return nil
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		tokens, _ := lexer.Lex(path, string(contents))
		program, _ := parser.Parse(tokens)
		for _, imported := range program.Imports {
			if imported.Go {
				imports[imported.Path] = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(imports))
	for path := range imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var source strings.Builder
	source.WriteString("package ontamadependencies\n")
	if len(paths) != 0 {
		source.WriteString("\nimport (\n")
		for _, path := range paths {
			fmt.Fprintf(&source, "\t_ %s\n", strconv.Quote(path))
		}
		source.WriteString(")\n")
	}
	return os.WriteFile(filepath.Join(directory, "ontama_dependencies.go"), []byte(source.String()), 0o644)
}

func readGoModRequirements(directory string, target BuildTarget) (map[string]string, error) {
	command := exec.Command("go", "mod", "edit", "-json")
	command.Dir = directory
	command.Env = target.Environment(command.Environ())
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	var edited editedGoMod
	if err = json.Unmarshal(output, &edited); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(edited.Require))
	for _, requirement := range edited.Require {
		result[requirement.Path] = requirement.Version
	}
	return result, nil
}

func CheckDependencies(root string) error {
	manifest, lock, err := ValidateLockedFiles(root)
	if err != nil {
		return err
	}
	modules, err := listModules(product.DependencyDirectory(manifest.Root), true, lock.Target)
	if err != nil {
		return fmt.Errorf("locked Go module graph is unavailable offline: %w", err)
	}
	actual, err := lockModules(manifest, modules)
	if err != nil {
		return err
	}
	if !slices.EqualFunc(actual, lock.Modules, func(left, right LockedModule) bool {
		return left.Path == right.Path && left.Version == right.Version && left.Sum == right.Sum && left.GoModSum == right.GoModSum && left.ReplacePath == right.ReplacePath && slices.Equal(left.LicenseFiles, right.LicenseFiles)
	}) {
		return fmt.Errorf("resolved Go module graph does not match %s; run %s deps lock", product.LockFileName, product.CommandName)
	}
	return nil
}

func listModules(directory string, offline bool, target BuildTarget) ([]listedModule, error) {
	command := exec.Command("go", "list", "-mod=readonly", "-m", "-json", "all")
	command.Dir = directory
	command.Env = target.Environment(command.Environ())
	if offline {
		command.Env = environmentWith(command.Env, "GOPROXY", "off")
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("go list failed: %s", message)
	}
	decoder := json.NewDecoder(&stdout)
	var modules []listedModule
	for {
		var module listedModule
		if err := decoder.Decode(&module); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("cannot decode go list output: %w", err)
		}
		modules = append(modules, module)
	}
	return modules, nil
}

func lockModules(manifest Manifest, modules []listedModule) ([]LockedModule, error) {
	result := make([]LockedModule, 0, len(modules))
	for _, module := range modules {
		if module.Main {
			continue
		}
		licenses, err := findLicenseFiles(module)
		if err != nil {
			return nil, fmt.Errorf("cannot record license metadata for Go module %q: %w", module.Path, err)
		}
		locked := LockedModule{Path: module.Path, Version: module.Version, Sum: module.Sum, GoModSum: module.GoModSum, LicenseFiles: licenses}
		if replacement, exists := manifest.Replacements[module.Path]; exists {
			locked.ReplacePath = filepath.ToSlash(filepath.Clean(filepath.FromSlash(replacement)))
		}
		result = append(result, locked)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		return result[i].Version < result[j].Version
	})
	return result, nil
}

const maximumLicenseFileSize = 4 << 20

func findLicenseFiles(module listedModule) ([]LockedLicenseFile, error) {
	directory := module.Dir
	if module.Replace != nil && module.Replace.Dir != "" {
		directory = module.Replace.Dir
	}
	result := []LockedLicenseFile{}
	if directory == "" {
		return result, nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !isLicenseFileName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("license file %q is a symbolic link", entry.Name())
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > maximumLicenseFileSize {
			return nil, fmt.Errorf("license file %q exceeds %d bytes", entry.Name(), maximumLicenseFileSize)
		}
		contents, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		result = append(result, LockedLicenseFile{Path: entry.Name(), SHA256: Hash(contents)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

func isLicenseFileName(name string) bool {
	upper := strings.ToUpper(name)
	for _, base := range []string{"LICENSE", "LICENCE", "COPYING"} {
		if upper == base || strings.HasPrefix(upper, base+".") || strings.HasPrefix(upper, base+"-") {
			return true
		}
	}
	return false
}

func DependencyLicenses(root string) ([]ModuleLicenses, error) {
	if err := CheckDependencies(root); err != nil {
		return nil, err
	}
	_, lock, err := ValidateLockedFiles(root)
	if err != nil {
		return nil, err
	}
	result := make([]ModuleLicenses, len(lock.Modules))
	for index, module := range lock.Modules {
		result[index] = ModuleLicenses{Path: module.Path, Version: module.Version, Files: append([]LockedLicenseFile(nil), module.LicenseFiles...)}
	}
	return result, nil
}

func validateDirectDependencies(manifest Manifest, modules []LockedModule) error {
	present := map[string]bool{}
	for _, module := range modules {
		present[module.Path] = true
	}
	for path := range manifest.Dependencies {
		if !present[path] {
			return fmt.Errorf("Go dependency %q is absent from the resolved module graph", path)
		}
	}
	return nil
}

func validateReplacementTargets(manifest Manifest) error {
	root, err := filepath.EvalSymlinks(manifest.Root)
	if err != nil {
		return fmt.Errorf("cannot resolve project root: %w", err)
	}
	for path, replacement := range manifest.Replacements {
		target, targetErr := filepath.EvalSymlinks(filepath.Join(manifest.Root, filepath.FromSlash(replacement)))
		if targetErr != nil {
			return fmt.Errorf("cannot resolve replacement %q at %q: %w", path, replacement, targetErr)
		}
		relative, relativeErr := filepath.Rel(root, target)
		if relativeErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("replacement %q resolves outside the project root", path)
		}
		info, statErr := os.Stat(target)
		if statErr != nil || !info.IsDir() {
			return fmt.Errorf("replacement %q does not resolve to a directory", path)
		}
	}
	return nil
}

func runGo(directory string, offline bool, target BuildTarget, arguments ...string) error {
	command := exec.Command("go", arguments...)
	command.Dir = directory
	command.Env = target.Environment(command.Environ())
	if offline {
		command.Env = environmentWith(command.Env, "GOPROXY", "off")
	}
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("go %s failed: %s", strings.Join(arguments, " "), message)
	}
	return nil
}

func environmentWith(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if !strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return append(result, prefix+value)
}

func readOptional(path string) ([]byte, error) {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []byte{}, nil
	}
	return contents, err
}

func unwrapPathError(err error) error {
	for err != nil {
		pathError, ok := err.(*os.PathError)
		if ok {
			return pathError
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return err
		}
		err = unwrapper.Unwrap()
	}
	return nil
}
