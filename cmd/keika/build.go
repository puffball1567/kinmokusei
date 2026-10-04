package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/attribution"
	"github.com/puffball1567/kinmokusei/internal/product"
	"github.com/puffball1567/kinmokusei/internal/project"
)

func buildWithAttribution(generatedDirectory, output string, target project.BuildTarget, hasTarget bool) (result error) {
	environment := project.OfflineEnvironment(os.Environ())
	var flags, tags []string
	if hasTarget {
		environment = target.Environment(environment)
		flags, tags = target.GoBuildFlags(), target.Tags
	}
	var sources []attribution.SourcePackage
	root := filepath.Dir(filepath.Dir(generatedDirectory))
	if _, err := os.Stat(filepath.Join(root, product.ProjectFileName)); err == nil {
		manifest, lock, err := project.ValidateLockedFiles(root)
		if err != nil {
			return err
		}
		graph, err := project.ReadPackageGraph(manifest, lock)
		if err != nil {
			return err
		}
		for _, pkg := range graph.Packages {
			sources = append(sources, attribution.SourcePackage{Path: pkg.Lock.Path, Version: pkg.Lock.Version, Dir: pkg.Directory})
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	bundle, err := attribution.Inspect(generatedDirectory, environment, flags, sources, tags)
	if err != nil {
		return err
	}
	// Stage beside the destination so publication uses same-filesystem renames.
	// Existing executables/notices are never touched by a failed query or build.
	stage, err := os.MkdirTemp(filepath.Dir(output), ".keika-build-")
	if err != nil {
		return err
	}
	defer func() {
		if _, retain := result.(*buildRecoveryError); !retain {
			_ = os.RemoveAll(stage)
		}
	}()
	stagedOutput := filepath.Join(stage, "executable")
	stagedNotices := filepath.Join(stage, "licenses")
	arguments := append([]string{"build"}, flags...)
	arguments = append(arguments, "-mod=readonly", "-buildvcs=false", "-o", stagedOutput, ".")
	command := exec.Command("go", arguments...)
	command.Dir, command.Env = generatedDirectory, environment
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("Go build failed: %w", err)
	}
	if err := bindBuildAttribution(&bundle, stagedOutput); err != nil {
		return err
	}
	if err := bundle.Write(stagedNotices); err != nil {
		return err
	}
	return publishBuild(stage, stagedOutput, stagedNotices, output)
}

func bindBuildAttribution(bundle *attribution.Bundle, executable string) error {
	info, err := buildinfo.ReadFile(executable)
	if err != nil {
		return err
	}
	if info.GoVersion != bundle.Index.Toolchain {
		return fmt.Errorf("Go toolchain changed during attribution collection")
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "GOOS":
			if setting.Value != bundle.Index.GOOS {
				return fmt.Errorf("Go target changed during attribution collection")
			}
		case "GOARCH":
			if setting.Value != bundle.Index.GOARCH {
				return fmt.Errorf("Go target changed during attribution collection")
			}
		case "CGO_ENABLED":
			if (setting.Value == "1") != bundle.Index.CGOEnabled {
				return fmt.Errorf("Go cgo policy changed during attribution collection")
			}
		case "-tags":
			bundle.Index.BuildTags = strings.Split(setting.Value, ",")
		}
	}
	file, err := os.Open(executable)
	if err != nil {
		return err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return err
	}
	bundle.Index.ExecutableSHA256 = hex.EncodeToString(digest.Sum(nil))
	return nil
}

type buildRecoveryError struct{ error }

func publishBuild(stage, stagedOutput, stagedNotices, output string) error {
	notices := output + "-licenses"
	if info, err := os.Lstat(output); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace non-regular build output")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Lstat(notices); err == nil {
		if err := verifyManagedNotices(notices); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	type publication struct {
		staged, destination, backup string
		saved, installed            bool
	}
	items := []publication{
		{staged: stagedNotices, destination: notices, backup: filepath.Join(stage, "old-licenses")},
		{staged: stagedOutput, destination: output, backup: filepath.Join(stage, "old-executable")},
	}
	rollback := func(cause error) error {
		for index := len(items) - 1; index >= 0; index-- {
			item := &items[index]
			if item.installed {
				if err := os.Rename(item.destination, item.staged); err != nil {
					return &buildRecoveryError{fmt.Errorf("build publication failed (%v); rollback failed: %w; retained staging directory %s", cause, err, stage)}
				}
			}
			if item.saved {
				if err := os.Rename(item.backup, item.destination); err != nil {
					return &buildRecoveryError{fmt.Errorf("build publication failed (%v); restoration failed: %w; retained staging directory %s", cause, err, stage)}
				}
			}
		}
		return cause
	}
	for index := range items {
		item := &items[index]
		if _, err := os.Lstat(item.destination); err == nil {
			if err := os.Rename(item.destination, item.backup); err != nil {
				return rollback(err)
			}
			item.saved = true
		} else if !os.IsNotExist(err) {
			return rollback(err)
		}
		if err := os.Rename(item.staged, item.destination); err != nil {
			return rollback(err)
		}
		item.installed = true
	}
	return nil
}

// Do not treat an arbitrary directory (or user-added/edited files) as disposable
// generated output. Refuse it and preserve it rather than removing user data.
func verifyManagedNotices(directory string) error {
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to replace non-directory attribution output")
	}
	contents, err := readManagedNotice(filepath.Join(directory, "INDEX.json"))
	if err != nil {
		return fmt.Errorf("refusing to replace unmanaged attribution directory: %w", err)
	}
	var index attribution.Index
	if err := json.Unmarshal(contents, &index); err != nil || index.Format != "kinmokusei-attribution-v1" {
		return fmt.Errorf("refusing to replace unmanaged attribution directory")
	}
	expected := map[string]string{"INDEX.json": ""}
	directories := map[string]bool{".": true}
	for _, file := range index.Files {
		if !fs.ValidPath(file.Path) || strings.Contains(file.Path, "\\") || len(file.SHA256) != sha256.Size*2 {
			return fmt.Errorf("invalid existing attribution index")
		}
		if _, ok := expected[file.Path]; ok {
			return fmt.Errorf("duplicate existing attribution index entry")
		}
		expected[file.Path] = file.SHA256
		for parent := filepath.Dir(filepath.FromSlash(file.Path)); parent != "."; parent = filepath.Dir(parent) {
			directories[filepath.ToSlash(parent)] = true
		}
	}
	err = filepath.WalkDir(directory, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			relative, err := filepath.Rel(directory, file)
			if err != nil {
				return err
			}
			if !directories[filepath.ToSlash(relative)] {
				return fmt.Errorf("refusing to replace modified attribution directory")
			}
			return nil
		}
		relative, err := filepath.Rel(directory, file)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		digest, ok := expected[relative]
		if !ok || !entry.Type().IsRegular() {
			return fmt.Errorf("refusing to replace modified attribution directory")
		}
		if digest != "" {
			contents, err := readManagedNotice(file)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(contents)
			if hex.EncodeToString(sum[:]) != digest {
				return fmt.Errorf("refusing to replace modified attribution file %s", relative)
			}
		}
		delete(expected, relative)
		return nil
	})
	if err != nil {
		return err
	}
	if len(expected) != 0 {
		return fmt.Errorf("refusing to replace incomplete attribution directory")
	}
	return nil
}

func readManagedNotice(file string) ([]byte, error) {
	const limit = 16 << 20
	info, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("invalid existing attribution file")
	}
	input, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	contents, err := io.ReadAll(io.LimitReader(input, limit+1))
	if err != nil {
		return nil, err
	}
	if len(contents) > limit {
		return nil, fmt.Errorf("oversized existing attribution file")
	}
	return contents, nil
}
