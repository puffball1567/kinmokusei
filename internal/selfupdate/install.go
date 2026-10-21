package selfupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func installationPaths(options Options) (launcher, binary, root string, original os.FileInfo, err error) {
	launcher, err = exec.LookPath(options.CommandPath)
	if err != nil {
		return
	}
	launcher, err = filepath.Abs(launcher)
	if err != nil {
		return
	}
	// Resolve the parent but keep the invoked symlink itself as the switch point.
	parent, pathErr := filepath.EvalSymlinks(filepath.Dir(launcher))
	if pathErr != nil {
		err = pathErr
		return
	}
	launcher = filepath.Join(parent, filepath.Base(launcher))
	binary, err = filepath.EvalSymlinks(launcher)
	if err != nil {
		return
	}
	physical, pathErr := filepath.EvalSymlinks(options.ExecutablePath)
	if pathErr != nil {
		err = pathErr
		return
	}
	actual, pathErr := os.Stat(physical)
	if pathErr != nil {
		err = pathErr
		return
	}
	invoked, pathErr := os.Stat(binary)
	if pathErr != nil {
		err = pathErr
		return
	}
	if !actual.Mode().IsRegular() || !os.SameFile(actual, invoked) {
		err = fmt.Errorf("invoked command does not identify the running compiler")
		return
	}
	original, err = os.Lstat(launcher)
	if err != nil {
		return
	}
	if !original.Mode().IsRegular() && original.Mode()&os.ModeSymlink == 0 {
		err = fmt.Errorf("compiler launcher is not a regular file or symlink")
		return
	}
	root = filepath.Join(parent, "keika-releases")
	// Preserve the existing versioned installation convention. In particular,
	// never replace the old binary inside its old version-labelled directory.
	versionDirectory := filepath.Dir(binary)
	if original.Mode()&os.ModeSymlink != 0 && filepath.Base(filepath.Dir(versionDirectory)) == "kinmokusei" && filepath.Base(versionDirectory) == strings.TrimPrefix(options.CurrentVersion, "v") {
		root = filepath.Dir(versionDirectory)
	}
	return
}

func (u updater) install(ctx context.Context, options Options, version, base string, archive, sums asset) (string, error) {
	launcher, _, root, original, err := installationPaths(options)
	if err != nil {
		return "", err
	}
	lockPath := filepath.Join(filepath.Dir(launcher), ".keika-update.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("cannot acquire update lock (check directory permissions or another update): %w", err)
	}
	defer os.Remove(lockPath)
	defer lock.Close()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("cannot create release directory (check permissions): %w", err)
	}
	final := filepath.Join(root, strings.TrimPrefix(version, "v"))
	// Exclusively reserve a new version directory. Never overwrite a user's
	// existing installation, symlink or partially managed release.
	if err := os.Mkdir(final, 0o755); err != nil {
		if os.IsExist(err) {
			info, statErr := os.Lstat(final)
			if statErr == nil && info.IsDir() {
				if cacheErr := validateReceipt(final, version, u.goos, u.goarch); cacheErr == nil {
					installed := filepath.Join(final, "keika")
					if err := u.verify(ctx, installed, version); err != nil {
						return "", err
					}
					if err := u.switchLauncher(ctx, launcher, installed, original); err != nil {
						return "", err
					}
					return installed, nil
				}
			}
		}
		return "", fmt.Errorf("cannot reserve release directory %s: %w", final, err)
	}
	committed := false
	defer func() {
		if !committed {
			os.RemoveAll(final)
		}
	}()
	staging, err := os.MkdirTemp(root, ".keika-update-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	bundle, err := u.downloadBundle(ctx, staging, version, base, archive, sums)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(bundle)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if err := u.rename(filepath.Join(bundle, entry.Name()), filepath.Join(final, entry.Name())); err != nil {
			return "", err
		}
	}
	installed := filepath.Join(final, "keika")
	if err := u.switchLauncher(ctx, launcher, installed, original); err != nil {
		return "", err
	}
	committed = true
	return installed, nil
}

func (u updater) downloadBundle(ctx context.Context, staging, version, base string, archive, sums asset) (string, error) {
	var contents strings.Builder
	if err := u.download(ctx, sums.URL, &contents, maxMetadataBytes); err != nil {
		return "", err
	}
	digest, err := archiveChecksum(contents.String(), archive.Name)
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(filepath.Join(staging, "download.tar.gz"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if err := u.download(ctx, archive.URL, io.MultiWriter(file, hash), maxArchiveBytes); err != nil {
		return "", err
	}
	if !bytes.Equal(digest, hash.Sum(nil)) {
		return "", fmt.Errorf("release archive SHA-256 checksum mismatch")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if err := extractArchive(file, staging, base); err != nil {
		return "", err
	}
	bundle := filepath.Join(staging, base)
	if err := u.verify(ctx, filepath.Join(bundle, "keika"), version); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := writeReceipt(bundle, version, u.goos, u.goarch); err != nil {
		return "", err
	}
	return bundle, nil
}

func (u updater) switchLauncher(ctx context.Context, launcher, installed string, original os.FileInfo) error {
	switchDirectory, err := os.MkdirTemp(filepath.Dir(launcher), ".keika-switch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(switchDirectory)
	link := filepath.Join(switchDirectory, "keika")
	if err := os.Symlink(installed, link); err != nil {
		return err
	}
	now, err := os.Lstat(launcher)
	if err != nil {
		return err
	}
	if !os.SameFile(original, now) || original.Size() != now.Size() || !original.ModTime().Equal(now.ModTime()) {
		return fmt.Errorf("compiler launcher changed during the update")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := u.rename(link, launcher); err != nil {
		return fmt.Errorf("cannot switch compiler launcher (check permissions): %w", err)
	}
	return nil
}
