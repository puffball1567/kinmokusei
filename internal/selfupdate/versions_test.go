package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func versionsFixture(t *testing.T) (updater, Options, string) {
	t.Helper()
	requireSwitchPlatform(t)
	u, _, _, _ := updaterFixture(t)
	// Listing must never execute a retained compiler or use the network.
	u.client = nil
	u.verify = func(context.Context, string, string) error { t.Fatal("executed cached compiler"); return nil }
	options, launcher, _ := installedFixture(t, false)
	root := filepath.Join(filepath.Dir(launcher), "keika-releases")
	for _, version := range []string{"0.4.8", "0.4.9", "0.4.10"} {
		directory := filepath.Join(root, version)
		writeFixtureBundle(t, directory)
		if err := writeReceipt(directory, "v"+version, "linux", "amd64"); err != nil {
			t.Fatal(err)
		}
	}
	return u, options, root
}

func TestListVersionsSortedAndReadOnly(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	u, options, root := versionsFixture(t)
	if err := os.Mkdir(filepath.Join(root, "keika-manager-ignored"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".keika-remove-ignored"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFixtureBundle(t, filepath.Join(root, "0.3.0"))
	if err := os.Mkdir(filepath.Join(root, "0.4.7"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "0.4.7", receiptName), []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "0.4.6")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "0.4.5"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := u.listVersions(context.Background(), options)
	if err != nil || len(entries) != 7 {
		t.Fatalf("entries=%+v err=%v", entries, err)
	}
	want := []string{"v0.4.10", "v0.4.9", "v0.4.8", "v0.4.7", "v0.4.6", "v0.4.5", "v0.3.0"}
	for i, entry := range entries {
		if entry.Version != want[i] {
			t.Fatalf("order=%+v", entries)
		}
		if i < 3 && (entry.Status != "managed" || entry.Bytes <= 0 || entry.Active || entry.Protected) {
			t.Fatalf("entry=%+v", entry)
		}
		if i >= 3 && i < 6 && entry.Status != "invalid" {
			t.Fatalf("entry=%+v", entry)
		}
		if i == 6 && entry.Status != "unmanaged" {
			t.Fatalf("entry=%+v", entry)
		}
	}
	after, _ := os.ReadDir(root)
	if len(before) != len(after) {
		t.Fatal("listing wrote a file")
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(options.CommandPath), ".keika-update.lock")); !os.IsNotExist(err) {
		t.Fatalf("listing created lock: %v", err)
	}
}

func TestListVersionsActiveAndInvalidInventory(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	u, options, root := versionsFixture(t)
	active := filepath.Join(root, "0.4.9", "keika")
	if err := os.Remove(options.CommandPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(active, options.CommandPath); err != nil {
		t.Fatal(err)
	}
	options.ExecutablePath = active
	if err := os.Mkdir(filepath.Join(root, "0.4.8", "user-empty-directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(active, filepath.Join(root, "0.4.10", "link")); err != nil {
		t.Fatal(err)
	}
	entries, err := u.listVersions(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Version == "v0.4.9" {
			if !entry.Active || !entry.Protected || entry.Status != "managed" {
				t.Fatalf("entry=%+v", entry)
			}
		} else if entry.Status != "invalid" {
			t.Fatalf("entry=%+v", entry)
		}
	}
}

func TestVersionsPathAndContextFailures(t *testing.T) {
	t.Parallel()
	u, _, _, _ := updaterFixture(t)
	u.goos = "windows"
	if _, err := u.listVersions(context.Background(), Options{}); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := u.listVersions(ctx, Options{}); err != context.Canceled {
		t.Fatalf("err=%v", err)
	}
	if _, err := u.listVersions(context.Background(), Options{CommandPath: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("missing command accepted")
	}
}

func TestListMissingRootDoesNotCreateStorage(t *testing.T) {
	requireSwitchPlatform(t)
	t.Parallel()
	u, _, _, _ := updaterFixture(t)
	options, launcher, _ := installedFixture(t, false)
	entries, err := u.listVersions(context.Background(), options)
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries=%+v err=%v", entries, err)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(launcher), "keika-releases")); !os.IsNotExist(err) {
		t.Fatalf("storage created: %v", err)
	}
}
