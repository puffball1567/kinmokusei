package project

import (
	"bytes"
	"fmt"
	"go/build"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type BuildTarget struct {
	GOOS       string   `json:"goos"`
	GOARCH     string   `json:"goarch"`
	CGOEnabled bool     `json:"cgoEnabled"`
	Tags       []string `json:"tags"`
}

var (
	targetNamePattern = regexp.MustCompile(`^[a-z0-9_]+$`)
	buildTagPattern   = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)
	targetsOnce       sync.Once
	supportedTargets  map[string]bool
	targetsError      error
)

func (config TargetConfig) Validate() error {
	if config.GOOS != "" && !targetNamePattern.MatchString(config.GOOS) {
		return fmt.Errorf("target goos %q must contain only lowercase letters, digits or underscores", config.GOOS)
	}
	if config.GOARCH != "" && !targetNamePattern.MatchString(config.GOARCH) {
		return fmt.Errorf("target goarch %q must contain only lowercase letters, digits or underscores", config.GOARCH)
	}
	if config.CGO != "" && config.CGO != "auto" && config.CGO != "enabled" && config.CGO != "disabled" {
		return fmt.Errorf("target cgo %q must be auto, enabled or disabled", config.CGO)
	}
	return validateBuildTags(config.Tags)
}

func parseBuildTags(value string) ([]string, error) {
	if value == "" {
		return []string{}, nil
	}
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			return nil, fmt.Errorf("target tags contains an empty build tag")
		}
		if seen[tag] {
			return nil, fmt.Errorf("target tags contains duplicate build tag %q", tag)
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	if err := validateBuildTags(tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func validateBuildTags(tags []string) error {
	for index, tag := range tags {
		if !buildTagPattern.MatchString(tag) {
			return fmt.Errorf("target build tag %q must contain only letters, digits, underscores or dots", tag)
		}
		if index > 0 && tags[index-1] >= tag {
			return fmt.Errorf("target build tags must be uniquely sorted")
		}
	}
	return nil
}

func ResolveTarget(config TargetConfig) (BuildTarget, error) {
	if err := config.Validate(); err != nil {
		return BuildTarget{}, err
	}
	target := BuildTarget{GOOS: config.GOOS, GOARCH: config.GOARCH, Tags: append([]string{}, config.Tags...)}
	if target.GOOS == "" {
		target.GOOS = runtime.GOOS
	}
	if target.GOARCH == "" {
		target.GOARCH = runtime.GOARCH
	}
	if err := validateSupportedTarget(target.GOOS, target.GOARCH); err != nil {
		return BuildTarget{}, err
	}
	switch config.CGO {
	case "enabled":
		target.CGOEnabled = true
	case "disabled":
		target.CGOEnabled = false
	default:
		target.CGOEnabled = target.GOOS == runtime.GOOS && target.GOARCH == runtime.GOARCH && build.Default.CgoEnabled
	}
	return target, nil
}

func validateSupportedTarget(goos, goarch string) error {
	targetsOnce.Do(func() {
		command := exec.Command("go", "tool", "dist", "list")
		output, err := command.Output()
		if err != nil {
			targetsError = fmt.Errorf("cannot list Go build targets: %w", err)
			return
		}
		supportedTargets = map[string]bool{}
		for _, target := range bytes.Fields(output) {
			supportedTargets[string(target)] = true
		}
	})
	if targetsError != nil {
		return targetsError
	}
	name := goos + "/" + goarch
	if !supportedTargets[name] {
		return fmt.Errorf("unsupported Go build target %q", name)
	}
	return nil
}

func (target BuildTarget) Validate() error {
	if target.GOOS == "" || target.GOARCH == "" {
		return fmt.Errorf("locked build target must include goos and goarch")
	}
	if !targetNamePattern.MatchString(target.GOOS) || !targetNamePattern.MatchString(target.GOARCH) {
		return fmt.Errorf("locked build target contains an invalid goos or goarch")
	}
	if target.Tags == nil {
		return fmt.Errorf("locked build target is missing tags metadata")
	}
	return validateBuildTags(target.Tags)
}

func (target BuildTarget) Environment(environment []string) []string {
	environment = environmentWith(environment, "GOOS", target.GOOS)
	environment = environmentWith(environment, "GOARCH", target.GOARCH)
	cgo := "0"
	if target.CGOEnabled {
		cgo = "1"
	}
	return environmentWith(environment, "CGO_ENABLED", cgo)
}

func (target BuildTarget) GoBuildFlags() []string {
	if len(target.Tags) == 0 {
		return nil
	}
	return []string{"-tags=" + strings.Join(target.Tags, ",")}
}

func (target BuildTarget) IsNative() bool {
	return target.GOOS == runtime.GOOS && target.GOARCH == runtime.GOARCH
}
