package selfupdate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

type UseOptions struct {
	Options
	Offline bool
}

// Use selects an exact stable release, including older releases. Unlike Run,
// it reuses verified local bundles and preserves an independent switch command.
func Use(ctx context.Context, options UseOptions) (Result, error) {
	u := updater{client: officialClient(), goos: runtime.GOOS, goarch: runtime.GOARCH, verify: verifyExecutable, rename: os.Rename}
	return u.use(ctx, options)
}

func (u updater) use(ctx context.Context, options UseOptions) (result Result, err error) {
	result = Result{CurrentVersion: options.CurrentVersion, Version: options.Version}
	if _, err = versionParts(options.Version); err != nil {
		return result, err
	}
	if u.goos != "linux" && u.goos != "darwin" {
		return result, fmt.Errorf("automatic version switching is not supported on %s; download the release archive from https://github.com/%s/releases/tag/%s", u.goos, repository, options.Version)
	}
	if options.CommandPath == "" {
		options.CommandPath = os.Args[0]
	}
	if options.ExecutablePath == "" {
		options.ExecutablePath, err = os.Executable()
		if err != nil {
			return result, err
		}
	}
	launcher, binary, root, original, err := useInstallationPaths(options.Options)
	if err != nil {
		return result, err
	}
	lockPath := filepath.Join(filepath.Dir(launcher), ".keika-update.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return result, fmt.Errorf("cannot acquire update lock (check directory permissions or another switch/update): %w", err)
	}
	defer os.Remove(lockPath)
	defer lock.Close()
	if err = os.MkdirAll(root, 0o755); err != nil {
		return result, err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return result, fmt.Errorf("release root must be a real directory")
	}
	final := filepath.Join(root, strings.TrimPrefix(options.Version, "v"))
	info, statErr := os.Lstat(final)
	if statErr != nil && !os.IsNotExist(statErr) {
		return result, statErr
	}
	exists := statErr == nil
	if exists && !info.IsDir() {
		return result, fmt.Errorf("release path is not a real directory")
	}
	created, committed := false, false
	defer func() {
		if created && !committed {
			os.RemoveAll(final)
		}
	}()
	if exists {
		err = validateReceipt(final, options.Version, u.goos, u.goarch)
	}
	if !exists || err != nil {
		// Never repair a modified managed bundle or replace an existing receipt.
		if exists {
			if _, receiptErr := os.Lstat(filepath.Join(final, receiptName)); !os.IsNotExist(receiptErr) {
				return result, fmt.Errorf("cached release failed validation; no files were replaced: %w", err)
			}
		}
		if options.Offline {
			return result, fmt.Errorf("verified cache is missing for %s; run keika use %s without --offline to fetch or verify it", options.Version, options.Version)
		}
		resolved, resolveErr := u.resolve(ctx, options.Version)
		if resolveErr != nil {
			return result, resolveErr
		}
		base := "keika_" + strings.TrimPrefix(options.Version, "v") + "_" + u.goos + "_" + u.goarch
		archive, assetErr := resolved.findAsset(base+".tar.gz", maxArchiveBytes)
		if assetErr != nil {
			return result, assetErr
		}
		sums, assetErr := resolved.findAsset("SHA256SUMS", maxMetadataBytes)
		if assetErr != nil {
			return result, assetErr
		}
		staging, stageErr := os.MkdirTemp(root, ".keika-use-")
		if stageErr != nil {
			return result, stageErr
		}
		defer os.RemoveAll(staging)
		bundle, downloadErr := u.downloadBundle(ctx, staging, options.Version, base, archive, sums)
		if downloadErr != nil {
			return result, downloadErr
		}
		if exists {
			local, localErr := bundleFiles(final)
			official, officialErr := bundleFiles(bundle)
			if localErr != nil || officialErr != nil || !reflect.DeepEqual(local, official) {
				return result, fmt.Errorf("existing release does not match the official bundle; no files were replaced")
			}
			if err = writeReceipt(final, options.Version, u.goos, u.goarch); err != nil {
				return result, err
			}
		} else {
			if err = os.Mkdir(final, 0o755); err != nil {
				return result, err
			}
			created = true
			entries, readErr := os.ReadDir(bundle)
			if readErr != nil {
				return result, readErr
			}
			for _, entry := range entries {
				if err = u.rename(filepath.Join(bundle, entry.Name()), filepath.Join(final, entry.Name())); err != nil {
					return result, err
				}
			}
		}
	}
	installed := filepath.Join(final, "keika")
	if err = validateReceipt(final, options.Version, u.goos, u.goarch); err != nil {
		return result, err
	}
	if err = u.verify(ctx, installed, options.Version); err != nil {
		return result, err
	}
	// Preserve management before replacing a possibly unversioned launcher.
	helper, finishManager, helperErr := u.ensureUseHelper(options.ExecutablePath, launcher, root)
	if helperErr != nil {
		return result, helperErr
	}
	defer func() {
		finishManager(committed)
	}()
	active, activeErr := os.Stat(binary)
	selected, selectedErr := os.Stat(installed)
	if activeErr != nil || selectedErr != nil {
		return result, fmt.Errorf("cannot inspect the current or selected compiler")
	}
	if !os.SameFile(active, selected) {
		if err = u.switchLauncher(ctx, launcher, installed, original); err != nil {
			return result, err
		}
		result.Updated = true
	} else if err = ctx.Err(); err != nil {
		return result, err
	}
	committed = true
	result.ExecutablePath, result.ManagerPath = installed, helper
	return result, nil
}
