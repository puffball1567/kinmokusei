package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseVersions(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"v0.4.8", "v1.0.0", "v12.34.56"} {
		if _, err := versionParts(v); err != nil {
			t.Error(err)
		}
	}
	for _, v := range []string{"", "main", "v01.2.3", "v1.0", "v1.2.3-rc.1", "v1.2.3+build", "v1.2.3/evil", "v18446744073709551616.0.0"} {
		if _, err := versionParts(v); err == nil {
			t.Errorf("accepted %q", v)
		}
	}
	if _, err := Run(context.Background(), Options{Version: "invalid"}); err == nil {
		t.Fatal("invalid tag accepted")
	}
}

func TestReleaseMetadataAndChecksums(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`invalid`, `{"tag_name":"v0.4.8","draft":true}`, `{"tag_name":"v0.4.8","prerelease":true}`, `{"tag_name":"v0.4.9"}`, `{"tag_name":"../evil"}`} {
		u, _, data, _ := updaterFixture(t)
		data["https://api.github.com/repos/"+repository+"/releases/tags/v0.4.8"] = []byte(body)
		if _, err := u.resolve(context.Background(), "v0.4.8"); err == nil {
			t.Errorf("accepted metadata %s", body)
		}
	}
	_, r, _, _ := updaterFixture(t)
	a, err := r.findAsset("SHA256SUMS", maxMetadataBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []release{
		{Tag: r.Tag},
		{Tag: r.Tag, Assets: []asset{a, a}},
		{Tag: r.Tag, Assets: []asset{{Name: a.Name, URL: "https://evil.invalid/file", Size: 1}}},
		{Tag: r.Tag, Assets: []asset{{Name: a.Name, URL: a.URL, Size: 0}}},
		{Tag: r.Tag, Assets: []asset{{Name: a.Name, URL: a.URL, Size: maxMetadataBytes + 1}}},
	} {
		if _, err := test.findAsset("SHA256SUMS", maxMetadataBytes); err == nil {
			t.Fatal("accepted invalid asset")
		}
	}
	line := strings.Repeat("ab", 32) + "  target.tar.gz\n"
	if _, err := archiveChecksum("unrelated\n"+line, "target.tar.gz"); err != nil {
		t.Fatal(err)
	}
	for _, contents := range []string{"", line + line, "gg  target.tar.gz\n", "ab  target.tar.gz\n"} {
		if _, err := archiveChecksum(contents, "target.tar.gz"); err == nil {
			t.Fatal("accepted invalid checksum list")
		}
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestDownloadPolicyAndFailures(t *testing.T) {
	t.Parallel()
	client := officialClient()
	for _, address := range []string{"http://github.com/file", "https://github.com.evil.invalid/file", "https://user@github.com/file", "https://github.com:443/file", "file:///etc/passwd"} {
		parsed, _ := url.Parse(address)
		if trustedURL(parsed) {
			t.Errorf("trusted %q", address)
		}
		if err := client.CheckRedirect(&http.Request{URL: parsed}, nil); err == nil {
			t.Errorf("redirect accepted %q", address)
		}
	}
	for _, host := range []string{"api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com"} {
		parsed, _ := url.Parse("https://" + host + "/file")
		if err := client.CheckRedirect(&http.Request{URL: parsed}, nil); err != nil {
			t.Error(err)
		}
		if err := client.CheckRedirect(&http.Request{URL: parsed}, make([]*http.Request, 10)); err == nil {
			t.Fatal("unbounded redirect chain")
		}
	}
	u, _, _, _ := updaterFixture(t)
	for _, address := range []string{"https://%xx", "https://evil.invalid/file"} {
		if err := u.download(context.Background(), address, io.Discard, 10); err == nil {
			t.Fatal("accepted invalid URL")
		}
	}
	for _, kind := range []string{"transport", "status", "content length", "stream limit", "read"} {
		t.Run(kind, func(t *testing.T) {
			u.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if kind == "transport" {
					return nil, errors.New("offline")
				}
				r := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("12345678901")), ContentLength: -1, Header: make(http.Header), Request: req}
				if kind == "status" {
					r.StatusCode = 404
				}
				if kind == "content length" {
					r.ContentLength = 11
				}
				if kind == "read" {
					r.Body = io.NopCloser(brokenReader{})
				}
				return r, nil
			})}
			if err := u.download(context.Background(), "https://github.com/file", io.Discard, 10); err == nil {
				t.Fatal("expected download error")
			}
		})
	}
}

func TestArchiveValidation(t *testing.T) {
	t.Parallel()
	const base = "keika_0.4.8_linux_amd64"
	for _, entry := range []archiveEntry{
		{name: "../outside", contents: "bad"},
		{name: "/outside", contents: "bad"},
		{name: base + "/../outside", contents: "bad"},
		{name: base + "/x\\outside", contents: "bad"},
		{name: base + "/C:outside", contents: "bad"},
		{name: base + "/link", kind: tar.TypeSymlink},
		{name: base + "/link", kind: tar.TypeLink},
		{name: base + "/fifo", kind: tar.TypeFifo},
	} {
		t.Run(entry.name, func(t *testing.T) {
			if err := extractArchive(bytes.NewReader(tarArchive(t, []archiveEntry{entry})), t.TempDir(), base); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
	for _, entries := range [][]archiveEntry{
		{{name: base + "/same", contents: "one"}, {name: base + "/SAME", contents: "two"}},
		{{name: base + "/same", contents: "one"}, {name: base + "/same", contents: "two"}},
		{{name: base + "/keika", contents: "compiler only"}},
	} {
		if err := extractArchive(bytes.NewReader(tarArchive(t, entries)), t.TempDir(), base); err == nil {
			t.Fatal("invalid bundle accepted")
		}
	}
	valid := tarArchive(t, append([]archiveEntry{{name: base + "/", kind: tar.TypeDir}}, bundleEntries(base)...))
	for _, data := range [][]byte{[]byte("bad gzip"), valid[:len(valid)/2], append([]byte(nil), valid...)} {
		if len(data) == len(valid) {
			data[len(data)-1] ^= 1
		}
		if err := extractArchive(bytes.NewReader(data), t.TempDir(), base); err == nil {
			t.Fatal("corrupt archive accepted")
		}
	}
	var large bytes.Buffer
	gz := gzip.NewWriter(&large)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: base + "/large", Mode: 0o644, Size: maxExpandedBytes + 1}); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractArchive(bytes.NewReader(large.Bytes()), t.TempDir(), base); err == nil {
		t.Fatal("oversized entry accepted")
	}
	destination := t.TempDir()
	if err := extractArchive(bytes.NewReader(valid), destination, base); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(destination, base, "keika"))
		if err != nil || info.Mode() != 0o755 {
			t.Fatalf("unsafe mode: %v %v", info, err)
		}
	}
}

func TestVerifyDownloadedExecutable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "main.go")
	binary := filepath.Join(root, "keika")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if err := os.WriteFile(source, []byte("package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"keika v0.4.8\")}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-o", binary, source).CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v %s", err, out)
	}
	if err := verifyExecutable(context.Background(), binary, "v0.4.8"); err != nil {
		t.Fatal(err)
	}
	if err := verifyExecutable(context.Background(), binary, "v0.4.9"); err == nil {
		t.Fatal("wrong version accepted")
	}
	if err := verifyExecutable(context.Background(), filepath.Join(root, "missing"), "v0.4.8"); err == nil {
		t.Fatal("missing binary accepted")
	}
}
