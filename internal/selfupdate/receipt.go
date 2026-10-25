package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
)

const receiptName = ".keika-release.json"

// A receipt detects changes to a previously verified local bundle. It is not
// a publisher signature or a defense against a user who can rewrite the cache.
type receipt struct {
	Format  int               `json:"format"`
	Version string            `json:"version"`
	GOOS    string            `json:"goos"`
	GOARCH  string            `json:"goarch"`
	Files   map[string]string `json:"files"`
}

func bundleFiles(root string) (map[string]string, error) {
	files := make(map[string]string)
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if name == receiptName {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxExpandedBytes-total || len(files) >= 2048 {
			return fmt.Errorf("invalid cached release entry %q", name)
		}
		total += info.Size()
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		n, copyErr := io.Copy(hash, io.LimitReader(file, info.Size()+1))
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if n != info.Size() {
			return fmt.Errorf("cached release changed while reading %q", name)
		}
		files[filepath.ToSlash(name)] = hex.EncodeToString(hash.Sum(nil))
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"keika", "README.md", "LICENSE", "THIRD_PARTY_NOTICES.md", "licenses/go/LICENSE", "licenses/go/INDEX.txt"} {
		if files[name] == "" {
			return nil, fmt.Errorf("cached release is missing %s", name)
		}
	}
	return files, nil
}

func writeReceipt(root, version, goos, arch string) error {
	files, err := bundleFiles(root)
	if err != nil {
		return err
	}
	data, err := json.Marshal(receipt{1, version, goos, arch, files})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(root, receiptName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			os.Remove(filepath.Join(root, receiptName))
		}
	}()
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	complete = closeErr == nil
	return closeErr
}

func readSmallJSON(path string, value any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxMetadataBytes {
		return fmt.Errorf("invalid local metadata")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, maxMetadataBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing local metadata")
	}
	return nil
}

func validateReceipt(root, version, goos, arch string) error {
	var recorded receipt
	if err := readSmallJSON(filepath.Join(root, receiptName), &recorded); err != nil {
		return err
	}
	if recorded.Format != 1 || recorded.Version != version || recorded.GOOS != goos || recorded.GOARCH != arch {
		return fmt.Errorf("cached release receipt does not match the requested version/target")
	}
	files, err := bundleFiles(root)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(files, recorded.Files) {
		return fmt.Errorf("cached release contents do not match its receipt")
	}
	return nil
}
