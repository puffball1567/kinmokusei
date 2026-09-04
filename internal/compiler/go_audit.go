package compiler

import (
	"bytes"
	"fmt"
	"go/importer"
	gotypes "go/types"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/sema"
)

type GoInteropAuditCount struct {
	Total          int `json:"total"`
	Supported      int `json:"supported"`
	RequiresUnsafe int `json:"requires_unsafe"`
	Unsupported    int `json:"unsupported"`
}

type GoInteropAuditItem struct {
	Package string                `json:"package"`
	Name    string                `json:"name"`
	Kind    string                `json:"kind"`
	Detail  string                `json:"detail"`
	Support sema.GoInteropSupport `json:"support"`
	Reason  string                `json:"reason,omitempty"`
}

type GoInteropPackageAudit struct {
	Path      string               `json:"path"`
	Overall   GoInteropAuditCount  `json:"overall"`
	Callables GoInteropAuditCount  `json:"callables"`
	Values    GoInteropAuditCount  `json:"values"`
	Types     GoInteropAuditCount  `json:"types"`
	Reasons   map[string]int       `json:"reasons,omitempty"`
	Items     []GoInteropAuditItem `json:"items"`
}

type GoInteropAuditFailure struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type GoInteropAuditReport struct {
	GoVersion         string                  `json:"go_version"`
	AttemptedPackages int                     `json:"attempted_packages"`
	LoadedPackages    int                     `json:"loaded_packages"`
	FailedPackages    []GoInteropAuditFailure `json:"failed_packages,omitempty"`
	Overall           GoInteropAuditCount     `json:"overall"`
	Callables         GoInteropAuditCount     `json:"callables"`
	Values            GoInteropAuditCount     `json:"values"`
	Types             GoInteropAuditCount     `json:"types"`
	Reasons           map[string]int          `json:"reasons,omitempty"`
	Packages          []GoInteropPackageAudit `json:"packages"`
}

// AuditGoInteropPackages measures the exported Go API surface accepted by the
// current direct-interop type conversion. Totals only include loaded packages;
// failures remain explicit so an incomplete environment cannot look complete.
func AuditGoInteropPackages(paths []string, goImporter gotypes.Importer) GoInteropAuditReport {
	if goImporter == nil {
		goImporter = importer.Default()
	}
	ordered := uniqueSortedStrings(paths)
	report := GoInteropAuditReport{
		GoVersion:         runtime.Version(),
		AttemptedPackages: len(ordered),
		Reasons:           map[string]int{},
	}
	for _, path := range ordered {
		packageInfo, err := goImporter.Import(path)
		if err != nil {
			report.FailedPackages = append(report.FailedPackages, GoInteropAuditFailure{Path: path, Error: err.Error()})
			continue
		}
		packageAudit := auditGoPackage(packageInfo)
		report.Packages = append(report.Packages, packageAudit)
		mergeGoInteropAuditCount(&report.Overall, packageAudit.Overall)
		mergeGoInteropAuditCount(&report.Callables, packageAudit.Callables)
		mergeGoInteropAuditCount(&report.Values, packageAudit.Values)
		mergeGoInteropAuditCount(&report.Types, packageAudit.Types)
		for reason, count := range packageAudit.Reasons {
			report.Reasons[reason] += count
		}
	}
	report.LoadedPackages = len(report.Packages)
	if len(report.Reasons) == 0 {
		report.Reasons = nil
	}
	return report
}

// StandardGoPackagePaths returns importable public packages reported by the
// active Go toolchain. Commands, internal implementation packages, and vendored
// copies are excluded because Kinmokusei users cannot import them as public API.
func StandardGoPackagePaths() ([]string, error) {
	command := exec.Command("go", "list", "-e", "-f", "{{.ImportPath}}", "std")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("cannot enumerate Go standard library packages: %s", message)
	}
	paths := make([]string, 0)
	for _, path := range strings.Fields(stdout.String()) {
		if isPublicStandardGoPackage(path) {
			paths = append(paths, path)
		}
	}
	paths = uniqueSortedStrings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("Go toolchain reported no public standard library packages")
	}
	return paths, nil
}

func auditGoPackage(packageInfo *gotypes.Package) GoInteropPackageAudit {
	result := GoInteropPackageAudit{Path: packageInfo.Path(), Reasons: map[string]int{}}
	seen := map[string]bool{}
	add := func(name, kind string, object gotypes.Object) {
		key := kind + "\x00" + name
		if seen[key] {
			return
		}
		seen[key] = true
		assessment := sema.AssessGoInteropObject(object)
		item := GoInteropAuditItem{
			Package: packageInfo.Path(), Name: name, Kind: kind,
			Detail:  gotypes.ObjectString(object, goAuditQualifier),
			Support: assessment.Support, Reason: assessment.Reason,
		}
		result.Items = append(result.Items, item)
		addGoInteropAssessment(&result.Overall, assessment.Support)
		switch kind {
		case "function", "method":
			addGoInteropAssessment(&result.Callables, assessment.Support)
		case "type":
			addGoInteropAssessment(&result.Types, assessment.Support)
		default:
			addGoInteropAssessment(&result.Values, assessment.Support)
		}
		if assessment.Reason != "" {
			result.Reasons[assessment.Reason]++
		}
	}

	for _, name := range packageInfo.Scope().Names() {
		object := packageInfo.Scope().Lookup(name)
		if object == nil || !object.Exported() {
			continue
		}
		switch typed := object.(type) {
		case *gotypes.Const:
			add(name, "constant", typed)
		case *gotypes.Func:
			add(name, "function", typed)
		case *gotypes.Var:
			add(name, "variable", typed)
		case *gotypes.TypeName:
			add(name, "type", typed)
			auditGoTypeMembers(name, typed, add)
		}
	}
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].Name != result.Items[j].Name {
			return result.Items[i].Name < result.Items[j].Name
		}
		return result.Items[i].Kind < result.Items[j].Kind
	})
	if len(result.Reasons) == 0 {
		result.Reasons = nil
	}
	return result
}

func auditGoTypeMembers(typeName string, declaration *gotypes.TypeName, add func(string, string, gotypes.Object)) {
	declaredType := declaration.Type()
	underlying := gotypes.Unalias(declaredType).Underlying()
	if structure, ok := underlying.(*gotypes.Struct); ok {
		for index := 0; index < structure.NumFields(); index++ {
			field := structure.Field(index)
			if field.Exported() {
				add(typeName+"."+field.Name(), "field", field)
			}
		}
	}
	methodSets := []*gotypes.MethodSet{gotypes.NewMethodSet(declaredType)}
	if _, pointer := declaredType.(*gotypes.Pointer); !pointer {
		methodSets = append(methodSets, gotypes.NewMethodSet(gotypes.NewPointer(declaredType)))
	}
	for _, methodSet := range methodSets {
		for index := 0; index < methodSet.Len(); index++ {
			method := methodSet.At(index).Obj()
			if method.Exported() {
				add(typeName+"."+method.Name(), "method", method)
			}
		}
	}
}

func addGoInteropAssessment(count *GoInteropAuditCount, support sema.GoInteropSupport) {
	count.Total++
	switch support {
	case sema.GoInteropSupported:
		count.Supported++
	case sema.GoInteropRequiresUnsafe:
		count.RequiresUnsafe++
	default:
		count.Unsupported++
	}
}

func mergeGoInteropAuditCount(target *GoInteropAuditCount, source GoInteropAuditCount) {
	target.Total += source.Total
	target.Supported += source.Supported
	target.RequiresUnsafe += source.RequiresUnsafe
	target.Unsupported += source.Unsupported
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func isPublicStandardGoPackage(path string) bool {
	return path != "" && path != "builtin" && path != "internal" &&
		!strings.HasPrefix(path, "cmd/") && !strings.HasPrefix(path, "internal/") &&
		!strings.HasPrefix(path, "vendor/") && !strings.HasSuffix(path, "/internal") &&
		!strings.Contains(path, "/internal/") && !strings.HasSuffix(path, "/vendor") &&
		!strings.Contains(path, "/vendor/")
}

func goAuditQualifier(packageInfo *gotypes.Package) string {
	if packageInfo == nil {
		return ""
	}
	return packageInfo.Path()
}
