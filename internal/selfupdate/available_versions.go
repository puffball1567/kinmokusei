package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"strings"
)

const releaseListPageSize = 30
const maxReleaseListPages = 100

// ListAvailableVersions explicitly fetches official stable release metadata.
// It never downloads an archive, executes a compiler, or changes an installation.
// Asset contents still undergo full verification when use/update installs them.
func ListAvailableVersions(ctx context.Context) ([]string, error) {
	u := updater{client: officialClient(), goos: runtime.GOOS, goarch: runtime.GOARCH}
	return u.availableVersions(ctx)
}

func (u updater) availableVersions(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if u.goos != "linux" && u.goos != "darwin" && u.goos != "windows" {
		return nil, fmt.Errorf("release listing is not supported on %s", u.goos)
	}
	if u.goarch != "amd64" && u.goarch != "arm64" {
		return nil, fmt.Errorf("release listing is not supported on %s", u.goarch)
	}
	extension := ".tar.gz"
	if u.goos == "windows" {
		extension = ".zip"
	}
	seen := map[string]bool{}
	var versions []string
	for page := 1; page <= maxReleaseListPages; page++ {
		endpoint := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d&page=%d", repository, releaseListPageSize, page)
		var body strings.Builder
		if err := u.download(ctx, endpoint, &body, maxMetadataBytes); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(strings.TrimSpace(body.String()), "[") {
			return nil, fmt.Errorf("invalid release list metadata: expected an array")
		}
		var releases []release
		if err := json.Unmarshal([]byte(body.String()), &releases); err != nil {
			return nil, fmt.Errorf("invalid release list metadata: %w", err)
		}
		if len(releases) > releaseListPageSize {
			return nil, fmt.Errorf("release list exceeds page size")
		}
		for _, release := range releases {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if release.Draft || release.Prerelease {
				continue
			}
			if _, err := versionParts(release.Tag); err != nil {
				continue
			}
			archiveName := "keika_" + strings.TrimPrefix(release.Tag, "v") + "_" + u.goos + "_" + u.goarch + extension
			archive, sums := false, false
			for _, asset := range release.Assets {
				archive = archive || asset.Name == archiveName
				sums = sums || asset.Name == "SHA256SUMS"
			}
			if !archive || !sums {
				continue
			}
			if _, err := release.findAsset(archiveName, maxArchiveBytes); err != nil {
				return nil, err
			}
			if _, err := release.findAsset("SHA256SUMS", maxMetadataBytes); err != nil {
				return nil, err
			}
			if !seen[release.Tag] {
				seen[release.Tag] = true
				versions = append(versions, release.Tag)
			}
		}
		if len(releases) < releaseListPageSize {
			sort.Slice(versions, func(i, j int) bool { return versionNewer(versions[i], versions[j]) })
			return versions, nil
		}
	}
	return nil, fmt.Errorf("official release list exceeds %d pages; no partial listing was returned", maxReleaseListPages)
}

func versionNewer(left, right string) bool {
	a, _ := versionParts(left)
	b, _ := versionParts(right)
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
