package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func availableRelease(version, goos, arch string) release {
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	archive := "keika_" + strings.TrimPrefix(version, "v") + "_" + goos + "_" + arch + extension
	r := release{Tag: version}
	for _, name := range []string{archive, "SHA256SUMS"} {
		r.Assets = append(r.Assets, asset{Name: name, URL: "https://github.com/" + repository + "/releases/download/" + version + "/" + name, Size: 10})
	}
	return r
}

func releaseListingFixture(t *testing.T, respond func(page int) (string, int)) (updater, *int) {
	t.Helper()
	requests := 0
	u := updater{goos: "linux", goarch: "amd64"}
	u.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != "GET" || req.URL.Scheme != "https" || req.URL.Host != "api.github.com" || req.URL.Path != "/repos/"+repository+"/releases" || req.URL.Query().Get("per_page") != strconv.Itoa(releaseListPageSize) {
			t.Fatalf("unexpected listing request: %s", req.URL)
		}
		page, err := strconv.Atoi(req.URL.Query().Get("page"))
		if err != nil || page != requests {
			t.Fatalf("page: %s", req.URL)
		}
		body, status := respond(page)
		return &http.Response{StatusCode: status, ContentLength: -1, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	u.verify = func(context.Context, string, string) error { t.Fatal("listing executed a compiler"); return nil }
	u.rename = func(string, string) error { t.Fatal("listing changed an installation"); return nil }
	return u, &requests
}

func listingJSON(t *testing.T, values []release) string {
	t.Helper()
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAvailableVersionsFiltersAndSorts(t *testing.T) {
	t.Parallel()
	stable := availableRelease("v0.4.10", "linux", "amd64")
	draft := availableRelease("v0.4.11", "linux", "amd64")
	draft.Draft = true
	prerelease := availableRelease("v0.4.12", "linux", "amd64")
	prerelease.Prerelease = true
	missingSum := availableRelease("v0.4.7", "linux", "amd64")
	missingSum.Assets = missingSum.Assets[:1]
	values := []release{availableRelease("v0.4.9", "linux", "amd64"), stable, draft, prerelease, missingSum, availableRelease("v0.5.0", "darwin", "arm64"), {Tag: "v1.0.0-rc.1"}, {Tag: "../bad"}, {Tag: "v18446744073709551616.0.0"}, stable}
	u, requests := releaseListingFixture(t, func(int) (string, int) { return listingJSON(t, values), 200 })
	got, err := u.availableVersions(context.Background())
	if err != nil || !reflect.DeepEqual(got, []string{"v0.4.10", "v0.4.9"}) || *requests != 1 {
		t.Fatalf("versions=%v requests=%d err=%v", got, *requests, err)
	}
}

func TestAvailableVersionsPaginationAndDeduplication(t *testing.T) {
	t.Parallel()
	var first []release
	for i := 0; i < releaseListPageSize; i++ {
		first = append(first, availableRelease(fmt.Sprintf("v0.0.%d", i), "linux", "amd64"))
	}
	u, requests := releaseListingFixture(t, func(page int) (string, int) {
		if page == 1 {
			return listingJSON(t, first), 200
		}
		return listingJSON(t, []release{first[0], availableRelease("v0.0.30", "linux", "amd64")}), 200
	})
	got, err := u.availableVersions(context.Background())
	if err != nil || *requests != 2 || len(got) != 31 || got[0] != "v0.0.30" || got[30] != "v0.0.0" {
		t.Fatalf("versions=%v requests=%d err=%v", got, *requests, err)
	}
}

func TestAvailableVersionsTargets(t *testing.T) {
	t.Parallel()
	for _, target := range []struct{ os, arch string }{{"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"windows", "amd64"}} {
		u, _ := releaseListingFixture(t, func(int) (string, int) {
			return listingJSON(t, []release{availableRelease("v0.4.10", target.os, target.arch)}), 200
		})
		u.goos, u.goarch = target.os, target.arch
		got, err := u.availableVersions(context.Background())
		if err != nil || !reflect.DeepEqual(got, []string{"v0.4.10"}) {
			t.Fatalf("%s/%s: %v %v", target.os, target.arch, got, err)
		}
	}
}

func TestAvailableVersionsRejectsInvalidMetadata(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		body   string
		status int
	}{{"null", 200}, {"{}", 200}, {"[", 200}, {"[] trailing", 200}, {"[]", 403}, {strings.Repeat(" ", int(maxMetadataBytes)+1), 200}} {
		u, _ := releaseListingFixture(t, func(int) (string, int) { return test.body, test.status })
		if got, err := u.availableVersions(context.Background()); err == nil || got != nil {
			t.Fatalf("invalid metadata accepted: %d bytes status=%d versions=%v err=%v", len(test.body), test.status, got, err)
		}
	}
	for _, damage := range []func(*release){func(r *release) { r.Assets[0].URL = "https://example.com/archive" }, func(r *release) { r.Assets[0].Size = 0 }, func(r *release) { r.Assets[1].Size = maxMetadataBytes + 1 }, func(r *release) { r.Assets = append(r.Assets, r.Assets[0]) }} {
		r := availableRelease("v0.4.10", "linux", "amd64")
		damage(&r)
		u, _ := releaseListingFixture(t, func(int) (string, int) { return listingJSON(t, []release{r}), 200 })
		if got, err := u.availableVersions(context.Background()); err == nil || got != nil {
			t.Fatalf("invalid asset accepted: %v %v", got, err)
		}
	}
}

func TestAvailableVersionsNeverReturnsPartialListing(t *testing.T) {
	t.Parallel()
	page := make([]release, releaseListPageSize)
	for i := range page {
		page[i] = availableRelease(fmt.Sprintf("v0.0.%d", i), "linux", "amd64")
	}
	u, _ := releaseListingFixture(t, func(n int) (string, int) {
		if n == 1 {
			return listingJSON(t, page), 200
		}
		return "failed", 500
	})
	if got, err := u.availableVersions(context.Background()); err == nil || got != nil {
		t.Fatalf("partial result: %v %v", got, err)
	}
	u, requests := releaseListingFixture(t, func(int) (string, int) { return listingJSON(t, page), 200 })
	if got, err := u.availableVersions(context.Background()); err == nil || got != nil || *requests != maxReleaseListPages {
		t.Fatalf("unbounded/partial listing: %v %v requests=%d", got, err, *requests)
	}
}

func TestAvailableVersionsCancellationAndUnsupportedTargets(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	u, requests := releaseListingFixture(t, func(int) (string, int) { t.Fatal("unexpected request"); return "[]", 200 })
	if _, err := u.availableVersions(ctx); !errors.Is(err, context.Canceled) || *requests != 0 {
		t.Fatalf("cancel: %v requests=%d", err, *requests)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	u, _ = releaseListingFixture(t, func(int) (string, int) { cancel(); return "[]", 200 })
	if _, err := u.availableVersions(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel during response: %v", err)
	}
	for _, target := range []struct{ os, arch string }{{"plan9", "amd64"}, {"linux", "386"}} {
		u, requests = releaseListingFixture(t, func(int) (string, int) { t.Fatal("unsupported target fetched metadata"); return "[]", 200 })
		u.goos, u.goarch = target.os, target.arch
		if _, err := u.availableVersions(context.Background()); err == nil || *requests != 0 {
			t.Fatalf("unsupported target: %v", err)
		}
	}
}
