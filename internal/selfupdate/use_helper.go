package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const managerMetadata = "launcher.json"

type useManager struct {
	Format   int    `json:"format"`
	Launcher string `json:"launcher"`
	Root     string `json:"root"`
}

func readUseManager(helper, executable string) (useManager, error) {
	var manager useManager
	physical, err := filepath.EvalSymlinks(helper)
	if err != nil {
		return manager, err
	}
	physical, err = filepath.Abs(physical)
	if err != nil {
		return manager, err
	}
	if err = readSmallJSON(filepath.Join(filepath.Dir(physical), managerMetadata), &manager); err != nil {
		return manager, err
	}
	if manager.Format != 1 || !filepath.IsAbs(manager.Launcher) || !filepath.IsAbs(manager.Root) || filepath.Clean(manager.Launcher) != manager.Launcher || filepath.Clean(manager.Root) != manager.Root || filepath.Join(filepath.Dir(manager.Launcher), "keika-use") != helper || filepath.Dir(filepath.Dir(physical)) != manager.Root || !strings.HasPrefix(filepath.Base(filepath.Dir(physical)), "keika-manager-") || filepath.Base(physical) != "keika-use" {
		return manager, fmt.Errorf("invalid switch manager layout")
	}
	info, err := os.Lstat(physical)
	if err != nil || !info.Mode().IsRegular() {
		return manager, fmt.Errorf("switch manager is not a regular file")
	}
	if executable != "" {
		actual, statErr := os.Stat(executable)
		if statErr != nil || !os.SameFile(info, actual) {
			return manager, fmt.Errorf("invoked manager does not identify the running compiler")
		}
	}
	return manager, nil
}

func useInstallationPaths(options Options) (launcher, binary, root string, original os.FileInfo, err error) {
	invoked, err := exec.LookPath(options.CommandPath)
	if err != nil {
		return
	}
	invoked, err = filepath.Abs(invoked)
	if err != nil {
		return
	}
	if filepath.Base(invoked) != "keika-use" {
		return installationPaths(options)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(invoked))
	if err != nil {
		return
	}
	invoked = filepath.Join(parent, "keika-use")
	manager, err := readUseManager(invoked, options.ExecutablePath)
	if err != nil {
		return
	}
	launcher, root = manager.Launcher, manager.Root
	original, err = os.Lstat(launcher)
	if err != nil {
		return
	}
	if !original.Mode().IsRegular() && original.Mode()&os.ModeSymlink == 0 {
		err = fmt.Errorf("compiler launcher is not a regular file or symlink")
		return
	}
	binary, err = filepath.EvalSymlinks(launcher)
	if err != nil {
		return
	}
	info, err := os.Stat(binary)
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("selected compiler is not a regular file")
	}
	return
}

// The helper is a private snapshot of the current compiler, not of the selected
// (possibly pre-management) release. It survives both downgrade and rollback.
func (u updater) ensureUseHelper(executable, launcher, root string) (string, func(bool), error) {
	helper := filepath.Join(filepath.Dir(launcher), "keika-use")
	noop := func(bool) {}
	var previous string
	var original os.FileInfo
	if info, err := os.Lstat(helper); err == nil {
		manager, err := readUseManager(helper, "")
		if err != nil || manager.Launcher != launcher || manager.Root != root {
			return "", noop, fmt.Errorf("existing keika-use is not this installation's switch manager; it was not replaced")
		}
		previous, err = filepath.EvalSymlinks(helper)
		if err != nil {
			return "", noop, err
		}
		old, err := os.Stat(previous)
		if err != nil {
			return "", noop, err
		}
		running, err := os.Stat(executable)
		if err != nil {
			return "", noop, err
		}
		if os.SameFile(old, running) {
			return helper, noop, nil
		}
		// A native newer compiler refreshes management code instead of keeping
		// the original manager snapshot indefinitely after compiler upgrades.
		original = info
	} else if !os.IsNotExist(err) {
		return "", noop, err
	}
	directory, err := os.MkdirTemp(root, "keika-manager-")
	if err != nil {
		return "", noop, err
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(directory)
		}
	}()
	source, err := os.Open(executable)
	if err != nil {
		return "", noop, err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxExpandedBytes {
		return "", noop, fmt.Errorf("invalid manager executable")
	}
	target := filepath.Join(directory, "keika-use")
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return "", noop, err
	}
	n, copyErr := io.Copy(file, io.LimitReader(source, info.Size()+1))
	closeErr := file.Close()
	if copyErr != nil {
		return "", noop, copyErr
	}
	if closeErr != nil {
		return "", noop, closeErr
	}
	if n != info.Size() {
		return "", noop, fmt.Errorf("manager executable changed while copying")
	}
	data, err := json.Marshal(useManager{1, launcher, root})
	if err != nil {
		return "", noop, err
	}
	if err = os.WriteFile(filepath.Join(directory, managerMetadata), data, 0o600); err != nil {
		return "", noop, err
	}
	if original == nil {
		if err = os.Symlink(target, helper); err != nil {
			return "", noop, err
		}
	} else {
		link := filepath.Join(directory, "switch-link")
		if err = os.Symlink(target, link); err != nil {
			return "", noop, err
		}
		now, statErr := os.Lstat(helper)
		if statErr != nil || !os.SameFile(original, now) {
			return "", noop, fmt.Errorf("switch manager changed during selection")
		}
		if err = u.rename(link, helper); err != nil {
			return "", noop, err
		}
	}
	keep = true
	return helper, func(committed bool) {
		if committed {
			if previous != "" {
				// Remove only our two snapshot files, never recursively remove
				// unrelated files a user might have added to the directory.
				os.Remove(previous)
				os.Remove(filepath.Join(filepath.Dir(previous), managerMetadata))
				os.Remove(filepath.Dir(previous))
			}
			return
		}
		if link, err := os.Readlink(helper); err == nil && link == target {
			if previous == "" {
				os.Remove(helper)
			} else {
				restore := filepath.Join(directory, "restore-link")
				if err := os.Symlink(previous, restore); err != nil {
					return
				}
				if err := os.Rename(restore, helper); err != nil {
					return
				}
			}
		}
		os.RemoveAll(directory)
	}, nil
}
