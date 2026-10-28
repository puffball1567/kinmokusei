package compiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/cabi"
)

type CABICompatibility struct {
	ExactFingerprint bool
	Additions        []string
	TypeAdditions    []string
	BreakingChanges  []string
}

func (result CABICompatibility) Compatible() bool {
	return len(result.BreakingChanges) == 0
}

type cabiCompatibilityVersion = cabi.Version
type cabiCompatibilityStatus = cabi.Status
type cabiCompatibilityFunction = cabi.Function
type cabiCompatibilityManifest = cabi.Manifest
type cabiCompatibilityFingerprintInput = cabi.FingerprintInput

// CompareCABIManifests verifies both manifests before classifying exact,
// additive, and breaking ABI changes. Additions are compatible; removal or a
// signature/status/gateway contract change is breaking.
func CompareCABIManifests(baselineData, currentData []byte) (CABICompatibility, error) {
	baseline, err := parseCABICompatibilityManifest("baseline", baselineData)
	if err != nil {
		return CABICompatibility{}, err
	}
	current, err := parseCABICompatibilityManifest("current", currentData)
	if err != nil {
		return CABICompatibility{}, err
	}
	result := CABICompatibility{ExactFingerprint: baseline.Fingerprint == current.Fingerprint}
	if baseline.GatewayVersion.Major != current.GatewayVersion.Major {
		result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf(
			"gateway major version changed from %d to %d", baseline.GatewayVersion.Major, current.GatewayVersion.Major,
		))
	} else if current.GatewayVersion.Minor < baseline.GatewayVersion.Minor {
		result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf(
			"gateway minor version decreased from %d to %d", baseline.GatewayVersion.Minor, current.GatewayVersion.Minor,
		))
	}
	if baseline.Status != current.Status {
		result.BreakingChanges = append(result.BreakingChanges, "status code contract changed")
	}
	currentFunctions := make(map[string]cabiCompatibilityFunction, len(current.Functions))
	for _, function := range current.Functions {
		currentFunctions[function.Symbol] = function
	}
	baselineFunctions := make(map[string]bool, len(baseline.Functions))
	for _, function := range baseline.Functions {
		baselineFunctions[function.Symbol] = true
		candidate, exists := currentFunctions[function.Symbol]
		if !exists {
			result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf("removed C ABI symbol %q", function.Symbol))
			continue
		}
		if !sameCABIFunction(function, candidate) {
			result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf("changed signature of C ABI symbol %q", function.Symbol))
			continue
		}
		for index, parameter := range function.ParameterTypes {
			compareCABIType(&result, fmt.Sprintf("C ABI symbol %q parameter %d", function.Symbol, index+1), parameter, candidate.ParameterTypes[index])
		}
		compareCABIType(&result, fmt.Sprintf("C ABI symbol %q result", function.Symbol), function.ResultType, candidate.ResultType)
	}
	for _, function := range current.Functions {
		if !baselineFunctions[function.Symbol] {
			result.Additions = append(result.Additions, function.Symbol)
		}
	}
	sort.Strings(result.Additions)
	sort.Strings(result.TypeAdditions)
	sort.Strings(result.BreakingChanges)
	return result, nil
}

func sameCABIFunction(left, right cabiCompatibilityFunction) bool {
	return left.Symbol == right.Symbol && slices.Equal(left.Parameters, right.Parameters) && left.Result == right.Result && left.ResultTransport == right.ResultTransport
}

func parseCABICompatibilityManifest(label string, data []byte) (cabiCompatibilityManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest cabiCompatibilityManifest
	if err := decoder.Decode(&manifest); err != nil {
		return cabiCompatibilityManifest{}, fmt.Errorf("invalid %s C ABI manifest: %w", label, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return cabiCompatibilityManifest{}, fmt.Errorf("invalid %s C ABI manifest: multiple JSON values", label)
		}
		return cabiCompatibilityManifest{}, fmt.Errorf("invalid %s C ABI manifest: trailing data: %w", label, err)
	}
	if manifest.SchemaVersion == 1 {
		return cabiCompatibilityManifest{}, fmt.Errorf("%s C ABI manifest schema 1 lacks semantic type contracts; regenerate it from the original baseline sources with emit-c-abi before comparing", label)
	}
	if manifest.SchemaVersion != cabi.SchemaVersion {
		return cabiCompatibilityManifest{}, fmt.Errorf("unsupported %s C ABI manifest schema version %d; expected %d", label, manifest.SchemaVersion, cabi.SchemaVersion)
	}
	if manifest.GatewayVersion.Major < 1 || manifest.GatewayVersion.Minor < 0 {
		return cabiCompatibilityManifest{}, fmt.Errorf("invalid %s C ABI gateway version %d.%d", label, manifest.GatewayVersion.Major, manifest.GatewayVersion.Minor)
	}
	previous := ""
	for index, function := range manifest.Functions {
		if function.Symbol == "" || function.Result == "" || function.ResultTransport == "" {
			return cabiCompatibilityManifest{}, fmt.Errorf("invalid %s C ABI function at index %d: symbol, result, and resultTransport are required", label, index)
		}
		if index != 0 && function.Symbol <= previous {
			return cabiCompatibilityManifest{}, fmt.Errorf("invalid %s C ABI manifest: functions must have unique symbols in ascending order", label)
		}
		previous = function.Symbol
		if err := validateCABIFunctionTypes(function); err != nil {
			return cabiCompatibilityManifest{}, fmt.Errorf("invalid %s C ABI symbol %q: %w", label, function.Symbol, err)
		}
	}
	canonical, err := json.Marshal(cabiCompatibilityFingerprintInput{
		SchemaVersion: manifest.SchemaVersion, GatewayVersion: manifest.GatewayVersion, Status: manifest.Status, Functions: manifest.Functions,
	})
	if err != nil {
		return cabiCompatibilityManifest{}, fmt.Errorf("cannot verify %s C ABI fingerprint: %w", label, err)
	}
	digest := sha256.Sum256(canonical)
	want := "sha256:" + hex.EncodeToString(digest[:])
	if manifest.Fingerprint != want {
		return cabiCompatibilityManifest{}, fmt.Errorf("invalid %s C ABI manifest fingerprint: got %q, expected %q", label, manifest.Fingerprint, want)
	}
	return manifest, nil
}
