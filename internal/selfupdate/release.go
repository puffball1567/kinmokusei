// Package selfupdate installs verified official compiler releases on explicit request.
package selfupdate

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const repository = "puffball1567/kinmokusei"
const maxArchiveBytes int64 = 64 << 20
const maxMetadataBytes int64 = 1 << 20

var releaseVersion = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func versionParts(version string) ([3]uint64, error) {
	var result [3]uint64
	match := releaseVersion.FindStringSubmatch(version)
	if match == nil {
		return result, fmt.Errorf("expected a stable release tag such as v0.4.7, got %q", version)
	}
	for i := range result {
		value, err := strconv.ParseUint(match[i+1], 10, 64)
		if err != nil {
			return result, fmt.Errorf("release version is too large: %q", version)
		}
		result[i] = value
	}
	return result, nil
}

type release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

func officialClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many release download redirects")
		}
		if !trustedURL(req.URL) {
			return fmt.Errorf("untrusted release redirect")
		}
		return nil
	}}
}

func trustedURL(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch u.Host {
	case "api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return true
	}
	return false
}

func (u updater) download(ctx context.Context, address string, dst io.Writer, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	if !trustedURL(req.URL) {
		return fmt.Errorf("untrusted release URL")
	}
	req.Header.Set("User-Agent", "keika-self-update")
	req.Header.Set("Accept", "application/octet-stream")
	if req.URL.Host == "api.github.com" {
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	response, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch official release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch official release: HTTP %d (check connectivity, release availability and GitHub rate limits)", response.StatusCode)
	}
	if response.ContentLength > limit {
		return fmt.Errorf("release download exceeds size limit")
	}
	n, err := io.Copy(dst, io.LimitReader(response.Body, limit+1))
	if err != nil {
		return fmt.Errorf("read release download: %w", err)
	}
	if n > limit {
		return fmt.Errorf("release download exceeds size limit")
	}
	return nil
}

func (u updater) resolve(ctx context.Context, requested string) (release, error) {
	endpoint := "https://api.github.com/repos/" + repository + "/releases/latest"
	if requested != "" {
		if _, err := versionParts(requested); err != nil {
			return release{}, err
		}
		endpoint = "https://api.github.com/repos/" + repository + "/releases/tags/" + requested
	}
	var data strings.Builder
	if err := u.download(ctx, endpoint, &data, maxMetadataBytes); err != nil {
		return release{}, err
	}
	var result release
	if err := json.Unmarshal([]byte(data.String()), &result); err != nil {
		return result, fmt.Errorf("invalid release metadata: %w", err)
	}
	if _, err := versionParts(result.Tag); err != nil {
		return result, err
	}
	if result.Draft || result.Prerelease || requested != "" && requested != result.Tag {
		return result, fmt.Errorf("release is not the requested stable release")
	}
	return result, nil
}

func (r release) findAsset(name string, limit int64) (asset, error) {
	var result asset
	found := false
	for _, candidate := range r.Assets {
		if candidate.Name != name {
			continue
		}
		if found {
			return asset{}, fmt.Errorf("release contains duplicate asset %q", name)
		}
		found = true
		result = candidate
	}
	if !found {
		return result, fmt.Errorf("official release %s has no asset %s", r.Tag, name)
	}
	expected := "https://github.com/" + repository + "/releases/download/" + r.Tag + "/" + name
	if result.URL != expected || result.Size <= 0 || result.Size > limit {
		return result, fmt.Errorf("invalid official release asset %q", name)
	}
	return result, nil
}

func archiveChecksum(contents, name string) ([]byte, error) {
	var digest []byte
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != name {
			continue
		}
		if digest != nil {
			return nil, fmt.Errorf("duplicate checksum for %s", name)
		}
		var err error
		digest, err = hex.DecodeString(fields[0])
		if err != nil || len(digest) != 32 {
			return nil, fmt.Errorf("invalid SHA-256 checksum for %s", name)
		}
	}
	if digest == nil {
		return nil, fmt.Errorf("missing SHA-256 checksum for %s", name)
	}
	return digest, nil
}
