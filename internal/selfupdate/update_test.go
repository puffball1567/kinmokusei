package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type archiveEntry struct {
	name, contents string
	kind           byte
}

func tarArchive(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var result bytes.Buffer
	gz := gzip.NewWriter(&result)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		kind := e.kind
		if kind == 0 {
			kind = tar.TypeReg
		}
		h := &tar.Header{Name: e.name, Typeflag: kind, Mode: 0o7777}
		if kind == tar.TypeReg || kind == tar.TypeRegA {
			h.Size = int64(len(e.contents))
		}
		if kind == tar.TypeSymlink || kind == tar.TypeLink {
			h.Linkname = "../../outside"
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size != 0 {
			if _, err := tw.Write([]byte(e.contents)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}

func bundleEntries(base string) []archiveEntry {
	var entries []archiveEntry
	for _, name := range []string{"keika", "README.md", "LICENSE", "THIRD_PARTY_NOTICES.md", "licenses/go/LICENSE", "licenses/go/INDEX.txt"} {
		entries = append(entries, archiveEntry{name: base + "/" + name, contents: "official " + name})
	}
	return entries
}

func updaterFixture(t *testing.T) (updater, release, map[string][]byte, *[]string) {
	t.Helper()
	return targetFixture(t, "linux", "amd64")
}

func targetFixture(t *testing.T, goos, goarch string) (updater, release, map[string][]byte, *[]string) {
	t.Helper()
	base := "keika_0.4.8_" + goos + "_" + goarch
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	archive := tarArchive(t, bundleEntries(base))
	hash := sha256.Sum256(archive)
	r := release{Tag: "v0.4.8"}
	data := map[string][]byte{}
	for name, content := range map[string][]byte{
		base + extension: archive,
		"SHA256SUMS":     []byte(fmt.Sprintf("%x  %s%s\n", hash, base, extension)),
	} {
		address := "https://github.com/" + repository + "/releases/download/" + r.Tag + "/" + name
		r.Assets = append(r.Assets, asset{Name: name, URL: address, Size: int64(len(content))})
		data[address] = content
	}
	metadata, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	data["https://api.github.com/repos/"+repository+"/releases/latest"] = metadata
	data["https://api.github.com/repos/"+repository+"/releases/tags/v0.4.8"] = metadata
	requests := []string{}
	u := updater{goos: goos, goarch: goarch, rename: os.Rename, verify: func(ctx context.Context, path, version string) error {
		got, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(got) != "official keika" || version != "v0.4.8" {
			return errors.New("unexpected compiler")
		}
		return ctx.Err()
	}}
	u.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.URL.String())
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		body, ok := data[req.URL.String()]
		if !ok {
			return nil, fmt.Errorf("unexpected request: %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header), Request: req}, nil
	})}
	return u, r, data, &requests
}

func installedFixture(t *testing.T, versioned bool) (Options, string, string) {
	t.Helper()
	root := t.TempDir()
	name := "keika"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	launcher := filepath.Join(root, "bin", name)
	binary := launcher
	if versioned {
		binary = filepath.Join(root, "opt", "kinmokusei", "0.4.7", name)
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("original compiler"), 0o755); err != nil {
		t.Fatal(err)
	}
	if versioned {
		if err := os.MkdirAll(filepath.Dir(launcher), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(binary, launcher); err != nil {
			t.Fatal(err)
		}
	}
	return Options{CurrentVersion: "v0.4.7", CommandPath: launcher, ExecutablePath: binary}, launcher, binary
}

func TestInstallOfficialRelease(t *testing.T) {
	t.Parallel()
	for _, versioned := range []bool{false, true} {
		t.Run(fmt.Sprint(versioned), func(t *testing.T) {
			u, _, _, requests := updaterFixture(t)
			options, launcher, binary := installedFixture(t, versioned)
			result, err := u.run(context.Background(), options)
			if err != nil || !result.Updated || result.Version != "v0.4.8" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			resolved, err := filepath.EvalSymlinks(launcher)
			if err != nil || resolved != result.ExecutablePath {
				t.Fatalf("launcher=%q err=%v", resolved, err)
			}
			for _, name := range []string{"keika", "LICENSE", "THIRD_PARTY_NOTICES.md", "licenses/go/LICENSE", "licenses/go/INDEX.txt"} {
				contents, err := os.ReadFile(filepath.Join(filepath.Dir(resolved), filepath.FromSlash(name)))
				if err != nil || string(contents) != "official "+name {
					t.Fatalf("missing release material %s: %v", name, err)
				}
			}
			if versioned {
				old, err := os.ReadFile(binary)
				if err != nil || string(old) != "original compiler" {
					t.Fatalf("old version changed: %v", err)
				}
				if filepath.Dir(resolved) != filepath.Join(filepath.Dir(filepath.Dir(binary)), "0.4.8") {
					t.Fatal("versioned layout was not preserved")
				}
			}
			if len(*requests) != 3 {
				t.Fatalf("requests=%v", *requests)
			}
			if _, err := os.Lstat(filepath.Join(filepath.Dir(launcher), ".keika-update.lock")); !os.IsNotExist(err) {
				t.Fatal("update lock left behind")
			}
		})
	}
}

func TestChecksAndVersionSelection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, current, requested, goos, arch, want string
		check, available                           bool
	}{
		{"check", "v0.4.7", "", "linux", "amd64", "", true, true},
		{"same", "v0.4.8", "", "linux", "amd64", "", false, false},
		{"newer patch", "v0.4.9", "", "linux", "amd64", "", false, false},
		{"newer minor", "v0.5.0", "", "linux", "amd64", "", false, false},
		{"newer major", "v1.0.0", "", "linux", "amd64", "", false, false},
		{"explicit downgrade", "v0.4.9", "v0.4.8", "linux", "amd64", "refusing to downgrade", false, false},
		{"explicit", "v0.4.7", "v0.4.8", "linux", "amd64", "", true, true},
		{"invalid tag", "v0.4.7", "../main", "linux", "amd64", "stable release tag", false, false},
		{"devel", "devel", "", "linux", "amd64", "development", false, false},
		{"devel check", "devel", "", "linux", "amd64", "", true, true},
		{"devel selected", "devel", "v0.4.8", "linux", "amd64", "", true, true},
		{"unsupported arch", "v0.4.7", "", "linux", "riscv64", "has no asset", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			u, _, _, requests := updaterFixture(t)
			u.goos = test.goos
			u.goarch = test.arch
			result, err := u.run(context.Background(), Options{CurrentVersion: test.current, Version: test.requested, Check: test.check, CommandPath: "must-not-touch", ExecutablePath: "must-not-touch"})
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("err=%v", err)
				}
				return
			}
			if err != nil || result.Available != test.available || result.Updated {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if len(*requests) != 1 {
				t.Fatalf("check/no-op fetched artifacts: %v", *requests)
			}
		})
	}
}

func TestInstallFailurePreservesCompiler(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"checksum", "missing checksum", "archive", "missing notices", "version", "switch", "publish", "changed launcher", "lock", "existing release", "mismatched executable", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			u, r, data, _ := updaterFixture(t)
			options, launcher, binary := installedFixture(t, true)
			base := "keika_0.4.8_linux_amd64"
			archiveURL := r.Assets[0].URL
			for _, a := range r.Assets {
				if a.Name == base+".tar.gz" {
					archiveURL = a.URL
				}
			}
			sumsURL := "https://github.com/" + repository + "/releases/download/v0.4.8/SHA256SUMS"
			ctx := context.Background()
			switch kind {
			case "checksum":
				data[archiveURL] = []byte("corrupt download")
			case "missing checksum":
				data[sumsURL] = []byte("unrelated\n")
			case "archive", "missing notices":
				contents := []byte("not an archive")
				if kind == "missing notices" {
					contents = tarArchive(t, []archiveEntry{{name: base + "/keika", contents: "official keika"}})
				}
				data[archiveURL] = contents
				hash := sha256.Sum256(contents)
				data[sumsURL] = []byte(fmt.Sprintf("%x  %s.tar.gz\n", hash, base))
			case "version":
				u.verify = func(context.Context, string, string) error { return errors.New("wrong version") }
			case "switch":
				u.rename = func(from, to string) error {
					if to == launcher {
						return errors.New("switch denied")
					}
					return os.Rename(from, to)
				}
			case "publish":
				u.rename = func(string, string) error { return errors.New("publish denied") }
			case "changed launcher":
				u.verify = func(context.Context, string, string) error {
					other := filepath.Join(filepath.Dir(binary), "replacement-compiler")
					if err := os.WriteFile(other, []byte("original compiler"), 0o755); err != nil {
						return err
					}
					if err := os.Remove(launcher); err != nil {
						return err
					}
					return os.Symlink(other, launcher)
				}
			case "lock":
				if err := os.WriteFile(filepath.Join(filepath.Dir(launcher), ".keika-update.lock"), []byte("occupied"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "existing release":
				if err := os.Mkdir(filepath.Join(filepath.Dir(filepath.Dir(binary)), "0.4.8"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "mismatched executable":
				options.ExecutablePath = filepath.Join(t.TempDir(), "other")
				if err := os.WriteFile(options.ExecutablePath, []byte("other"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				u.verify = func(context.Context, string, string) error { cancel(); return nil }
			}
			if result, err := u.run(ctx, options); err == nil || result.Updated {
				t.Fatalf("unexpected success: %+v %v", result, err)
			}
			old, err := os.ReadFile(launcher)
			if err != nil || string(old) != "original compiler" {
				t.Fatalf("original compiler lost: %q %v", old, err)
			}
			if kind != "existing release" {
				if _, err := os.Lstat(filepath.Join(filepath.Dir(filepath.Dir(binary)), "0.4.8")); !os.IsNotExist(err) {
					t.Fatalf("failed update left a release behind: %v", err)
				}
			}
		})
	}
}

func TestReleaseTargets(t *testing.T) {
	t.Parallel()
	for _, target := range [][2]string{{"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"windows", "amd64"}} {
		t.Run(strings.Join(target[:], "/"), func(t *testing.T) {
			u, _, _, requests := targetFixture(t, target[0], target[1])
			if target[0] == "windows" {
				result, err := u.run(context.Background(), Options{CurrentVersion: "v0.4.7", Check: true})
				if err != nil || !result.Available || result.Updated || len(*requests) != 1 {
					t.Fatalf("check=%+v err=%v requests=%v", result, err, *requests)
				}
				if _, err := u.run(context.Background(), Options{CurrentVersion: "v0.4.7"}); err == nil || !strings.Contains(err.Error(), "not supported on windows") {
					t.Fatalf("err=%v", err)
				}
				return
			}
			options, _, _ := installedFixture(t, true)
			result, err := u.run(context.Background(), options)
			if err != nil || !result.Updated {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}
