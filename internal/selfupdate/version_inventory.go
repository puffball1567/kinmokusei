package selfupdate

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type versionFile struct {
	name string
	info os.FileInfo
}

func versionInventory(ctx context.Context, root string) ([]versionFile, int64, error) {
	var entries []versionFile
	var size int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(entries) >= 4096 {
			return fmt.Errorf("version directory has too many entries")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("version directory contains a symlink or special file")
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, versionFile{name, info})
		if info.Mode().IsRegular() {
			if info.Size() < 0 || info.Size() > maxExpandedBytes+maxMetadataBytes-size {
				return fmt.Errorf("version directory exceeds managed bundle size limit")
			}
			size += info.Size()
		}
		return nil
	})
	return entries, size, err
}

func ownedVersionInventory(ctx context.Context, directory, version, goos, arch string) ([]versionFile, int64, error) {
	if err := validateReceipt(directory, version, goos, arch); err != nil {
		return nil, 0, fmt.Errorf("release is not an unchanged managed bundle: %w", err)
	}
	var recorded receipt
	if err := readSmallJSON(filepath.Join(directory, receiptName), &recorded); err != nil {
		return nil, 0, err
	}
	allowedDirectories := map[string]bool{".": true}
	for name := range recorded.Files {
		for parent := filepath.Dir(filepath.FromSlash(name)); parent != "."; parent = filepath.Dir(parent) {
			allowedDirectories[parent] = true
		}
	}
	entries, size, err := versionInventory(ctx, directory)
	if err != nil {
		return nil, 0, err
	}
	for _, entry := range entries {
		if entry.info.IsDir() && !allowedDirectories[entry.name] {
			return nil, 0, fmt.Errorf("version directory contains unmanaged directory %q", entry.name)
		}
		if !entry.info.IsDir() && entry.name != receiptName && recorded.Files[filepath.ToSlash(entry.name)] == "" {
			return nil, 0, fmt.Errorf("version directory contains unmanaged file %q", entry.name)
		}
	}
	return entries, size, nil
}
