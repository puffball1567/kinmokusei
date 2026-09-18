package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/product"
)

type NewProjectOptions struct {
	Name    string
	Module  string
	License string
}

// NewProject prepares a complete, offline-locked project before reserving its
// destination. Existing directories, files and symlinks are never overwritten.
func NewProject(kind, directory string, options NewProjectOptions) error {
	if directory == "" {
		return fmt.Errorf("new project requires a destination directory")
	}
	destination, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	files, err := projectTemplate(kind, filepath.Base(destination), options)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("project destination %q already exists; choose a new directory", directory)
	} else if !os.IsNotExist(err) {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(destination), ".keika-new-")
	if err != nil {
		return fmt.Errorf("cannot prepare project; destination parent must exist: %w", err)
	}
	defer os.RemoveAll(staging)
	for _, name := range sortedKeys(files) {
		if err := os.WriteFile(filepath.Join(staging, name), []byte(files[name]), 0o644); err != nil {
			return err
		}
	}
	if _, err := LockDependencies(staging, true); err != nil {
		return fmt.Errorf("cannot initialize project offline; destination was not created: %w", err)
	}
	return publishNewProject(staging, destination)
}

func projectTemplate(kind, directoryName string, options NewProjectOptions) (map[string]string, error) {
	if kind != "app" && kind != "library" {
		return nil, fmt.Errorf("unknown project template %q; expected app or library", kind)
	}
	if kind != "library" && options.License != "" {
		return nil, fmt.Errorf("--license applies only to the library template")
	}
	name := options.Name
	if name == "" {
		name = directoryName
	}
	module := options.Module
	if module == "" {
		module = "example.com/" + name
	}
	manifest := Manifest{Project: Project{Name: name, Version: "0.1.0", GoModule: module, GoVersion: "1.23"}}
	if kind == "library" {
		license := options.License
		if license == "" {
			license = "UNLICENSED"
		}
		manifest.Package = PackageConfig{Entry: "index.km", MinimumVersion: product.DevelopmentCompatibilityVersion, Backend: "go", License: license}
	}
	contents, err := RenderManifest(manifest)
	if err != nil {
		return nil, err
	}
	files := map[string]string{
		product.ProjectFileName: string(contents),
		".gitignore":            "/.kinmokusei/\n/keika.out\n/keika.out.exe\n",
	}
	readme := "# " + name + "\n\nA " + product.DisplayName + " " + kind + " project.\n\n"
	readme += "Requires Kinmokusei and Go 1.23 or later. The initial dependency lock is prepared offline.\n\n"
	if kind == "app" {
		files["main.km"] = "import go { Println } from \"fmt\"\n\nconst main = (): void => {\n  Println(" + strconv.Quote("Hello from "+name) + ")\n}\n"
		readme += "```sh\nkeika check\nkeika run\nkeika build\n```\n\n"
	} else {
		files["index.km"] = "export const greet = (name: string): string => {\n  return \"Hello, \" + name + \"!\"\n}\n"
		files["go.mod"] = "module " + module + "\n\ngo 1.23\n"
		readme += "```sh\nkeika check\nkeika emit-go -package library\n```\n\n"
		readme += "Consume this package with `import { greet } from " + strconv.Quote(module) + "`.\n"
		readme += "Before publishing, set the real repository module path in both kinmokusei.toml and go.mod.\n"
		readme += "Keep project version 0.1.0 aligned with the v0.1.0 tag. See the external-packages guide for local replacements.\n\n"
		readme += "The manifest records license metadata only; no license text is generated.\n"
		readme += "UNLICENSED is the default unless --license was supplied. Choose and provide the appropriate license before publishing.\n\n"
	}
	readme += "Commit kinmokusei.lock; .kinmokusei/ contains disposable generated state.\n"
	readme += "After cloning on the same target, run `keika deps fetch --offline` to restore the locked state.\n"
	readme += "After changing the manifest or target, run `keika deps lock --offline` explicitly.\n"
	files["README.md"] = readme
	return files, nil
}

// Exclusive creation is portable and does not replace an empty destination
// that appeared after preflight. If publication fails, leave partial output
// visible rather than deleting files another process might have created.
func publishNewProject(staging, destination string) error {
	if err := os.Mkdir(destination, 0o755); err != nil {
		return fmt.Errorf("cannot create project destination %q: %w", destination, err)
	}
	err := filepath.WalkDir(staging, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(staging, path)
		if err != nil || relative == "." {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.Mkdir(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected non-regular staged file %q", relative)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(contents)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	})
	if err != nil {
		return fmt.Errorf("project publication failed; partial output retained at %q: %w", destination, err)
	}
	return nil
}
