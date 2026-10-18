package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxExpandedBytes int64 = 256 << 20

func extractArchive(archive io.Reader, destination, root string) error {
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("invalid release archive: %w", err)
	}
	defer gz.Close()
	expanded := &io.LimitedReader{R: gz, N: maxExpandedBytes + 1}
	reader := tar.NewReader(expanded)
	seen := map[string]bool{}
	var total int64
	for entries := 0; ; entries++ {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid release archive: %w", err)
		}
		if entries >= 2048 {
			return fmt.Errorf("release archive has too many entries")
		}
		name := strings.TrimSuffix(header.Name, "/")
		if name == "" || len(name) > 1024 || strings.ContainsAny(name, "\\:\x00") || path.Clean(name) != name || name != root && !strings.HasPrefix(name, root+"/") {
			return fmt.Errorf("unsafe release archive path %q", header.Name)
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("duplicate release archive path %q", name)
		}
		seen[key] = true
		target := filepath.Join(destination, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxExpandedBytes-total {
				return fmt.Errorf("expanded release archive exceeds size limit")
			}
			total += header.Size
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if name == root+"/keika" {
				mode = 0o755
			}
			file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("release archive contains unsupported entry %q", name)
		}
	}
	// Read through the gzip trailer too; tar's end marker can precede it.
	if _, err := io.Copy(io.Discard, expanded); err != nil {
		return fmt.Errorf("invalid release archive trailer: %w", err)
	}
	if expanded.N <= 0 {
		return fmt.Errorf("expanded release archive exceeds size limit")
	}
	for _, name := range []string{"keika", "README.md", "LICENSE", "THIRD_PARTY_NOTICES.md", "licenses/go/LICENSE", "licenses/go/INDEX.txt"} {
		info, err := os.Lstat(filepath.Join(destination, root, filepath.FromSlash(name)))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("release archive is missing regular file %s", name)
		}
	}
	return nil
}
