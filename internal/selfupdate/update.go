package selfupdate

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Options struct {
	CurrentVersion string
	Version        string
	Check          bool
	// CommandPath is the invoked launcher; ExecutablePath is the running binary.
	// Both default to process information when empty.
	CommandPath    string
	ExecutablePath string
}

type Result struct {
	CurrentVersion string
	Version        string
	Updated        bool
	Available      bool
	ExecutablePath string
	RequireVersion bool
	ManagerPath    string
}

type updater struct {
	client       *http.Client
	goos, goarch string
	verify       func(context.Context, string, string) error
	rename       func(string, string) error
}

func Run(ctx context.Context, options Options) (Result, error) {
	u := updater{client: officialClient(), goos: runtime.GOOS, goarch: runtime.GOARCH, verify: verifyExecutable, rename: os.Rename}
	return u.run(ctx, options)
}

func (u updater) run(ctx context.Context, options Options) (Result, error) {
	result := Result{CurrentVersion: options.CurrentVersion}
	resolved, err := u.resolve(ctx, options.Version)
	if err != nil {
		return result, err
	}
	result.Version = resolved.Tag
	latest, _ := versionParts(resolved.Tag)
	current, currentErr := versionParts(options.CurrentVersion)
	result.RequireVersion = currentErr != nil
	if currentErr == nil {
		if current == latest {
			return result, nil
		}
		for i := range current {
			if current[i] == latest[i] {
				continue
			}
			if current[i] > latest[i] {
				if options.Version != "" {
					return result, fmt.Errorf("refusing to downgrade %s to %s", options.CurrentVersion, resolved.Tag)
				}
				return result, nil
			}
			break
		}
	} else if options.Version == "" && !options.Check {
		return result, fmt.Errorf("development or unrecognized compiler version %q; select an official release with --version vX.Y.Z", options.CurrentVersion)
	}
	result.Available = true
	base := "keika_" + strings.TrimPrefix(resolved.Tag, "v") + "_" + u.goos + "_" + u.goarch
	extension := ".tar.gz"
	if u.goos == "windows" {
		extension = ".zip"
	}
	archive, err := resolved.findAsset(base+extension, maxArchiveBytes)
	if err != nil {
		return result, err
	}
	sums, err := resolved.findAsset("SHA256SUMS", maxMetadataBytes)
	if err != nil {
		return result, err
	}
	if options.Check {
		return result, nil
	}
	if u.goos != "linux" && u.goos != "darwin" {
		return result, fmt.Errorf("automatic replacement is not supported on %s; download the verified archive from https://github.com/%s/releases/tag/%s", u.goos, repository, resolved.Tag)
	}
	if options.CommandPath == "" {
		options.CommandPath = os.Args[0]
	}
	if options.ExecutablePath == "" {
		options.ExecutablePath, err = os.Executable()
		if err != nil {
			return result, err
		}
	}
	installed, err := u.install(ctx, options, resolved.Tag, base, archive, sums)
	if err != nil {
		return result, fmt.Errorf("update aborted; existing compiler was not replaced: %w", err)
	}
	result.ExecutablePath = installed
	result.Updated = true
	return result, nil
}

func verifyExecutable(ctx context.Context, executable, version string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// No shell is used. Execute only after the official archive checksum has
	// been verified and all archive paths and required notices were checked.
	command := exec.CommandContext(ctx, executable, "version")
	output, err := command.Output()
	if err != nil || strings.TrimSpace(string(output)) != "keika "+version {
		return fmt.Errorf("downloaded compiler failed version verification")
	}
	return nil
}
