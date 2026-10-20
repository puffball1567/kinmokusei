package selfupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func requireSwitchPlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("automatic switching requires Linux/macOS")
	}
}

func TestUseDowngradeAndOfflineSwitch(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	u, _, _, requests := updaterFixture(t)
	options, launcher, _ := installedFixture(t, false)
	options.CurrentVersion = "v0.4.9"
	options.Version = "v0.4.8"
	result, err := u.use(context.Background(), UseOptions{Options: options})
	if err != nil || !result.Updated || result.ManagerPath != filepath.Join(filepath.Dir(launcher), "keika-use") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(*requests) != 3 {
		t.Fatalf("requests=%v", *requests)
	}
	manager, err := filepath.EvalSymlinks(result.ManagerPath)
	if err != nil {
		t.Fatal(err)
	}
	if original, err := os.ReadFile(manager); err != nil || string(original) != "original compiler" {
		t.Fatalf("manager was not preserved: %v", err)
	}
	// Select an arbitrary old compiler, then use the independent manager to get
	// back without any network request or command support in that old compiler.
	old := filepath.Join(t.TempDir(), "old")
	if err := os.WriteFile(old, []byte("legacy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(launcher); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(old, launcher); err != nil {
		t.Fatal(err)
	}
	options.CommandPath, options.ExecutablePath = result.ManagerPath, manager
	before := len(*requests)
	for attempt := 0; attempt < 2; attempt++ {
		selected, err := u.use(context.Background(), UseOptions{Options: options, Offline: true})
		if err != nil || selected.Updated != (attempt == 0) || selected.ExecutablePath != result.ExecutablePath {
			t.Fatalf("selected=%+v err=%v", selected, err)
		}
	}
	if len(*requests) != before {
		t.Fatalf("offline selection accessed network: %v", *requests)
	}
	if err := validateReceipt(filepath.Dir(result.ExecutablePath), "v0.4.8", "linux", "amd64"); err != nil {
		t.Fatal(err)
	}
}

func TestUseAdoptsOnlyIdenticalOfficialLegacyBundle(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "identical", true: "modified"}[mismatch], func(t *testing.T) {
			u, _, _, _ := updaterFixture(t)
			options, launcher, _ := installedFixture(t, false)
			options.Version = "v0.4.8"
			final := filepath.Join(filepath.Dir(launcher), "keika-releases", "0.4.8")
			writeFixtureBundle(t, final)
			if mismatch {
				if err := os.WriteFile(filepath.Join(final, "keika"), []byte("user compiler"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			result, err := u.use(context.Background(), UseOptions{Options: options})
			if mismatch {
				if err == nil || result.Updated || !strings.Contains(err.Error(), "does not match") {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				if data, _ := os.ReadFile(filepath.Join(final, "keika")); string(data) != "user compiler" {
					t.Fatal("existing files replaced")
				}
			} else if err != nil || !result.Updated {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func writeFixtureBundle(t *testing.T, root string) {
	t.Helper()
	for _, entry := range bundleEntries("bundle") {
		path := filepath.Join(root, strings.TrimPrefix(entry.name, "bundle/"))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(entry.contents), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUseFailuresPreserveLauncherAndCache(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	for _, kind := range []string{"offline miss", "checksum", "verify", "switch", "publish", "lock", "helper collision", "changed launcher", "cancelled", "corrupt cache", "target symlink", "root symlink", "receipt symlink", "wrong target", "missing notice", "extra symlink"} {
		t.Run(kind, func(t *testing.T) {
			u, r, data, requests := updaterFixture(t)
			options, launcher, _ := installedFixture(t, false)
			options.Version = "v0.4.8"
			selected := UseOptions{Options: options}
			root := filepath.Join(filepath.Dir(launcher), "keika-releases")
			final := filepath.Join(root, "0.4.8")
			ctx := context.Background()
			switch kind {
			case "offline miss":
				selected.Offline = true
			case "checksum":
				for _, asset := range r.Assets {
					if strings.HasSuffix(asset.Name, ".tar.gz") {
						data[asset.URL] = []byte("bad")
					}
				}
			case "verify":
				u.verify = func(context.Context, string, string) error { return errors.New("bad compiler") }
			case "switch":
				u.rename = func(from, to string) error {
					if to == launcher {
						return errors.New("cannot switch")
					}
					return os.Rename(from, to)
				}
			case "publish":
				u.rename = func(string, string) error { return errors.New("cannot publish") }
			case "lock":
				if err := os.WriteFile(filepath.Join(filepath.Dir(launcher), ".keika-update.lock"), []byte("held"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "helper collision":
				if err := os.WriteFile(filepath.Join(filepath.Dir(launcher), "keika-use"), []byte("user file"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "changed launcher":
				u.verify = func(context.Context, string, string) error {
					return os.WriteFile(launcher, []byte("external edit"), 0o755)
				}
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				u.verify = func(context.Context, string, string) error { cancel(); return nil }
			case "target symlink":
				if err := os.MkdirAll(root, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), final); err != nil {
					t.Fatal(err)
				}
			case "root symlink":
				if err := os.Symlink(t.TempDir(), root); err != nil {
					t.Fatal(err)
				}
			default:
				writeFixtureBundle(t, final)
				if err := writeReceipt(final, "v0.4.8", "linux", "amd64"); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "corrupt cache":
					if err := os.WriteFile(filepath.Join(final, "keika"), []byte("bad"), 0o755); err != nil {
						t.Fatal(err)
					}
				case "receipt symlink":
					if err := os.Remove(filepath.Join(final, receiptName)); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(launcher, filepath.Join(final, receiptName)); err != nil {
						t.Fatal(err)
					}
				case "wrong target":
					if err := os.Remove(filepath.Join(final, receiptName)); err != nil {
						t.Fatal(err)
					}
					if err := writeReceipt(final, "v0.4.8", "darwin", "arm64"); err != nil {
						t.Fatal(err)
					}
				case "missing notice":
					if err := os.Remove(filepath.Join(final, "LICENSE")); err != nil {
						t.Fatal(err)
					}
				case "extra symlink":
					if err := os.Symlink(launcher, filepath.Join(final, "extra")); err != nil {
						t.Fatal(err)
					}
				}
			}
			result, err := u.use(ctx, selected)
			if err == nil || result.Updated {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			original, readErr := os.ReadFile(launcher)
			want := "original compiler"
			if kind == "changed launcher" {
				want = "external edit"
			}
			if readErr != nil || string(original) != want {
				t.Fatalf("launcher changed: %q %v", original, readErr)
			}
			if selected.Offline && len(*requests) != 0 {
				t.Fatal("offline request")
			}
			if kind != "helper collision" {
				if _, err := os.Lstat(filepath.Join(filepath.Dir(launcher), "keika-use")); !os.IsNotExist(err) {
					t.Fatalf("failed switch left helper: %v", err)
				}
			}
			entries, _ := os.ReadDir(root)
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "keika-manager-") || strings.HasPrefix(entry.Name(), ".keika-use-") {
					t.Fatalf("left staging/manager: %s", entry.Name())
				}
			}
		})
	}
}

func TestUseVersionAndPlatformValidation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ version, goos, want string }{{"", "linux", "stable release"}, {"../main", "linux", "stable release"}, {"v0.4.8-rc.1", "linux", "stable release"}, {"v0.4.8", "windows", "not supported"}} {
		u, _, _, requests := updaterFixture(t)
		u.goos = test.goos
		_, err := u.use(context.Background(), UseOptions{Options: Options{Version: test.version}})
		if err == nil || !strings.Contains(err.Error(), test.want) || len(*requests) != 0 {
			t.Fatalf("err=%v requests=%v", err, *requests)
		}
	}
}

func TestUseRefreshesManagerTransactionally(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "rollback"}[fail], func(t *testing.T) {
			u, _, _, _ := updaterFixture(t)
			options, launcher, _ := installedFixture(t, false)
			options.Version = "v0.4.8"
			initial, err := u.use(context.Background(), UseOptions{Options: options})
			if err != nil {
				t.Fatal(err)
			}
			previous, err := filepath.EvalSymlinks(initial.ManagerPath)
			if err != nil {
				t.Fatal(err)
			}
			note := filepath.Join(filepath.Dir(previous), "user-note")
			if err := os.WriteFile(note, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			newBinary := filepath.Join(t.TempDir(), "new-compiler")
			if err := os.WriteFile(newBinary, []byte("new manager implementation"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(launcher); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(newBinary, launcher); err != nil {
				t.Fatal(err)
			}
			options.ExecutablePath = newBinary
			if fail {
				u.rename = func(from, to string) error {
					if to == launcher {
						return errors.New("switch denied")
					}
					return os.Rename(from, to)
				}
			}
			_, err = u.use(context.Background(), UseOptions{Options: options, Offline: true})
			if (err != nil) != fail {
				t.Fatalf("err=%v", err)
			}
			manager, err := filepath.EvalSymlinks(initial.ManagerPath)
			if err != nil {
				t.Fatal(err)
			}
			want := "new manager implementation"
			if fail {
				want = "original compiler"
			}
			contents, err := os.ReadFile(manager)
			if err != nil || string(contents) != want {
				t.Fatalf("contents=%q err=%v", contents, err)
			}
			if !fail {
				if _, err := os.Lstat(previous); !os.IsNotExist(err) {
					t.Fatalf("old manager snapshot retained: %v", err)
				}
			}
			if data, err := os.ReadFile(note); err != nil || string(data) != "keep" {
				t.Fatalf("user file lost: %v", err)
			}
		})
	}
}

func TestUpdateReusesVerifiedCachedRelease(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	u, _, _, requests := updaterFixture(t)
	options, launcher, _ := installedFixture(t, false)
	root := filepath.Join(filepath.Dir(launcher), "keika-releases", "0.4.8")
	writeFixtureBundle(t, root)
	if err := writeReceipt(root, "v0.4.8", "linux", "amd64"); err != nil {
		t.Fatal(err)
	}
	result, err := u.run(context.Background(), options)
	if err != nil || !result.Updated || len(*requests) != 1 {
		t.Fatalf("result=%+v err=%v requests=%v", result, err, *requests)
	}
}

func TestUsePreservesVersionedLayout(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	u, _, _, _ := updaterFixture(t)
	options, launcher, original := installedFixture(t, true)
	options.Version = "v0.4.8"
	result, err := u.use(context.Background(), UseOptions{Options: options})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(filepath.Dir(original)), "0.4.8", "keika")
	if result.ExecutablePath != want {
		t.Fatalf("installed=%s want=%s", result.ExecutablePath, want)
	}
	if data, err := os.ReadFile(original); err != nil || string(data) != "original compiler" {
		t.Fatalf("old version lost: %v", err)
	}
	manager, err := filepath.EvalSymlinks(result.ManagerPath)
	if err != nil {
		t.Fatal(err)
	}
	options.CommandPath, options.ExecutablePath = result.ManagerPath, manager
	if _, err := u.use(context.Background(), UseOptions{Options: options, Offline: true}); err != nil {
		t.Fatal(err)
	}
	if selected, err := filepath.EvalSymlinks(launcher); err != nil || selected != want {
		t.Fatalf("selected=%s err=%v", selected, err)
	}
}
