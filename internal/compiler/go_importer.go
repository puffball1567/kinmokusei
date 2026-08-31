package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/importer"
	goparser "go/parser"
	gotoken "go/token"
	gotypes "go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/puffball1567/onsentamago/internal/project"
)

type moduleGoImporter struct {
	directory   string
	exportFiles map[string]string
	loadErrors  map[string]error
	importer    gotypes.Importer
	target      *project.BuildTarget
}

type listedGoPackage struct {
	ImportPath     string
	Dir            string
	Export         string
	IgnoredGoFiles []string
	Error          *struct {
		Err string
	}
}

func newModuleGoImporter(directory string, target *project.BuildTarget) gotypes.Importer {
	resolver := &moduleGoImporter{
		directory:   directory,
		exportFiles: map[string]string{},
		loadErrors:  map[string]error{},
		target:      target,
	}
	resolver.importer = importer.ForCompiler(gotoken.NewFileSet(), "gc", resolver.lookup)
	return resolver
}

func (i *moduleGoImporter) Import(path string) (*gotypes.Package, error) {
	return i.importer.Import(path)
}

func (i *moduleGoImporter) lookup(path string) (io.ReadCloser, error) {
	if exportFile := i.exportFiles[path]; exportFile != "" {
		return os.Open(exportFile)
	}
	if err, attempted := i.loadErrors[path]; attempted {
		return nil, err
	}
	if err := i.load(path); err != nil {
		i.loadErrors[path] = err
		return nil, err
	}
	exportFile := i.exportFiles[path]
	if exportFile == "" {
		err := fmt.Errorf("Go package %q did not provide compiler export data", path)
		i.loadErrors[path] = err
		return nil, err
	}
	return os.Open(exportFile)
}

func (i *moduleGoImporter) load(path string) error {
	arguments := []string{"list", "-mod=readonly", "-deps", "-export", "-json", path}
	if usesVendorDirectory(i.directory) {
		arguments[1] = "-mod=vendor"
	}
	command := exec.Command("go", arguments...)
	command.Dir = i.directory
	command.Env = environmentWith("GOPROXY", "off")
	if i.target != nil {
		arguments = append([]string{"list"}, append(i.target.GoBuildFlags(), arguments[1:]...)...)
		command = exec.Command("go", arguments...)
		command.Dir = i.directory
		command.Env = i.target.Environment(environmentWith("GOPROXY", "off"))
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	commandErr := command.Run()

	decoder := json.NewDecoder(&stdout)
	var packageError error
	var requestedPackage *listedGoPackage
	for {
		var listed listedGoPackage
		if err := decoder.Decode(&listed); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("cannot decode go list output for %q: %w", path, err)
		}
		if listed.ImportPath != "" && listed.Export != "" {
			i.exportFiles[listed.ImportPath] = listed.Export
		}
		if listed.ImportPath == path && listed.Error != nil && listed.Error.Err != "" {
			copy := listed
			requestedPackage = &copy
			packageError = i.contextualLoadError(path, listed.Error.Err, requestedPackage)
		}
	}
	if packageError != nil {
		return packageError
	}
	if commandErr != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = commandErr.Error()
		}
		return i.contextualLoadError(path, message, requestedPackage)
	}
	return nil
}

func (i *moduleGoImporter) contextualLoadError(path, message string, listed *listedGoPackage) error {
	if i.target == nil {
		return fmt.Errorf("go list failed for %q: %s", path, message)
	}
	lower := strings.ToLower(message)
	if !i.target.CGOEnabled && strings.Contains(lower, "build constraints exclude all go files") {
		if i.packageHasIgnoredCgoFile(path, listed) {
			return fmt.Errorf(`Go package %q requires cgo for target %s/%s, but CGO_ENABLED=0; set [target] cgo = "enabled" and regenerate the dependency lock`, path, i.target.GOOS, i.target.GOARCH)
		}
		return fmt.Errorf("Go package %q has no files for target %s/%s and the locked build tags", path, i.target.GOOS, i.target.GOARCH)
	}
	if i.target.CGOEnabled && (strings.Contains(lower, "c compiler") || strings.Contains(lower, "gcc") || strings.Contains(lower, "clang")) && (strings.Contains(lower, "not found") || strings.Contains(lower, "executable file") || strings.Contains(lower, "no such file")) {
		return fmt.Errorf("Go package %q requires a working C toolchain for target %s/%s with CGO_ENABLED=1: %s", path, i.target.GOOS, i.target.GOARCH, message)
	}
	return fmt.Errorf("go list failed for %q on target %s/%s with CGO_ENABLED=%d: %s", path, i.target.GOOS, i.target.GOARCH, boolInt(i.target.CGOEnabled), message)
}

func (i *moduleGoImporter) packageHasIgnoredCgoFile(path string, listed *listedGoPackage) bool {
	if listed != nil && packageListingHasIgnoredCgoFile(*listed) {
		return true
	}
	arguments := []string{"list", "-mod=readonly", "-e", "-json", path}
	if usesVendorDirectory(i.directory) {
		arguments[1] = "-mod=vendor"
	}
	arguments = append([]string{"list"}, append(i.target.GoBuildFlags(), arguments[1:]...)...)
	command := exec.Command("go", arguments...)
	command.Dir = i.directory
	command.Env = i.target.Environment(environmentWith("GOPROXY", "off"))
	output, err := command.Output()
	if err != nil {
		return false
	}
	var discovered listedGoPackage
	if err = json.Unmarshal(output, &discovered); err != nil {
		return false
	}
	return packageListingHasIgnoredCgoFile(discovered)
}

func packageListingHasIgnoredCgoFile(listed listedGoPackage) bool {
	for _, name := range listed.IgnoredGoFiles {
		path := filepath.Join(listed.Dir, name)
		file, err := goparser.ParseFile(gotoken.NewFileSet(), path, nil, goparser.ImportsOnly)
		if err != nil {
			continue
		}
		for _, imported := range file.Imports {
			if imported.Path != nil && imported.Path.Value == `"C"` {
				return true
			}
		}
	}
	return false
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func environmentWith(name, value string) []string {
	prefix := name + "="
	environment := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, prefix) {
			environment = append(environment, item)
		}
	}
	return append(environment, prefix+value)
}

func usesVendorDirectory(directory string) bool {
	for current := directory; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "vendor", "modules.txt")); err == nil {
			return true
		}
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
	}
}
