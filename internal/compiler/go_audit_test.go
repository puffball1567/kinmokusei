package compiler

import (
	"fmt"
	"go/constant"
	gotypes "go/types"
	"strings"
	"testing"

	"github.com/puffball1567/onsentamago/internal/sema"
)

type auditImporter map[string]*gotypes.Package

func (i auditImporter) Import(path string) (*gotypes.Package, error) {
	packageInfo := i[path]
	if packageInfo == nil {
		return nil, fmt.Errorf("package %s is unavailable", path)
	}
	return packageInfo, nil
}

func TestAuditGoInteropPackagesMatrix(t *testing.T) {
	packageInfo := syntheticAuditPackage(t)
	report := AuditGoInteropPackages([]string{"missing/package", packageInfo.Path(), packageInfo.Path(), ""}, auditImporter{packageInfo.Path(): packageInfo})
	if report.AttemptedPackages != 2 || report.LoadedPackages != 1 || len(report.FailedPackages) != 1 {
		t.Fatalf("package counts = attempted %d loaded %d failed %#v", report.AttemptedPackages, report.LoadedPackages, report.FailedPackages)
	}
	if report.FailedPackages[0].Path != "missing/package" || !strings.Contains(report.FailedPackages[0].Error, "unavailable") {
		t.Fatalf("failure = %#v", report.FailedPackages[0])
	}
	assertAuditCount(t, "overall", report.Overall, GoInteropAuditCount{Total: 9, Supported: 6, RequiresUnsafe: 2, Unsupported: 1})
	assertAuditCount(t, "callables", report.Callables, GoInteropAuditCount{Total: 5, Supported: 3, RequiresUnsafe: 1, Unsupported: 1})
	assertAuditCount(t, "values", report.Values, GoInteropAuditCount{Total: 3, Supported: 2, RequiresUnsafe: 1})
	assertAuditCount(t, "types", report.Types, GoInteropAuditCount{Total: 1, Supported: 1})

	packageAudit := report.Packages[0]
	if packageAudit.Path != packageInfo.Path() || len(packageAudit.Items) != 9 {
		t.Fatalf("package audit = %#v", packageAudit)
	}
	items := map[string]GoInteropAuditItem{}
	for _, item := range packageAudit.Items {
		items[item.Kind+":"+item.Name] = item
	}
	checks := []struct {
		key     string
		support sema.GoInteropSupport
		reason  string
	}{
		{"function:Safe", sema.GoInteropSupported, ""},
		{"function:Unsafe", sema.GoInteropRequiresUnsafe, "unsafe.Pointer"},
		{"function:Anonymous", sema.GoInteropSupported, ""},
		{"field:Record.Value", sema.GoInteropSupported, ""},
		{"field:Record.Raw", sema.GoInteropRequiresUnsafe, "unsafe.Pointer"},
		{"method:Record.Inspect", sema.GoInteropUnsupported, "anonymous Go interface"},
		{"method:Record.Read", sema.GoInteropSupported, ""},
	}
	for _, check := range checks {
		item, ok := items[check.key]
		if !ok || item.Support != check.support || !strings.Contains(item.Reason, check.reason) {
			t.Errorf("item %q = %#v, want %q containing %q", check.key, item, check.support, check.reason)
		}
	}
	if _, exists := items["field:Record.hidden"]; exists {
		t.Fatal("unexported field was audited")
	}
}

func TestPublicStandardGoPackageFilterMatrix(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"fmt", true},
		{"net/http", true},
		{"unsafe", true},
		{"builtin", false},
		{"cmd/go", false},
		{"internal/abi", false},
		{"net/http/internal", false},
		{"vendor/golang.org/x/net", false},
		{"crypto/vendor/example", false},
		{"", false},
	}
	for _, test := range tests {
		if got := isPublicStandardGoPackage(test.path); got != test.want {
			t.Errorf("isPublicStandardGoPackage(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}

func syntheticAuditPackage(t *testing.T) *gotypes.Package {
	t.Helper()
	packageInfo := gotypes.NewPackage("example.com/audit", "audit")
	integer := gotypes.Typ[gotypes.Int]
	unsafePointer := gotypes.Typ[gotypes.UnsafePointer]
	anonymousStruct := gotypes.NewStruct([]*gotypes.Var{gotypes.NewField(0, packageInfo, "Value", integer, false)}, nil)
	anonymousMethod := gotypes.NewFunc(0, packageInfo, "Value", gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(), gotypes.NewTuple(), false))
	anonymousInterface := gotypes.NewInterfaceType([]*gotypes.Func{anonymousMethod}, nil).Complete()

	insert := func(object gotypes.Object) {
		if existing := packageInfo.Scope().Insert(object); existing != nil {
			t.Fatalf("duplicate synthetic object %s", object.Name())
		}
	}
	makeFunction := func(name string, parameters, results []gotypes.Type) *gotypes.Func {
		params := make([]*gotypes.Var, len(parameters))
		for index, parameter := range parameters {
			params[index] = gotypes.NewVar(0, packageInfo, "", parameter)
		}
		returns := make([]*gotypes.Var, len(results))
		for index, result := range results {
			returns[index] = gotypes.NewVar(0, packageInfo, "", result)
		}
		return gotypes.NewFunc(0, packageInfo, name, gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(params...), gotypes.NewTuple(returns...), false))
	}
	insert(makeFunction("Safe", []gotypes.Type{integer}, []gotypes.Type{integer}))
	insert(makeFunction("Unsafe", []gotypes.Type{unsafePointer}, nil))
	insert(makeFunction("Anonymous", []gotypes.Type{anonymousStruct}, nil))
	insert(gotypes.NewConst(0, packageInfo, "Answer", integer, constant.MakeInt64(42)))

	fields := []*gotypes.Var{
		gotypes.NewField(0, packageInfo, "Value", integer, false),
		gotypes.NewField(0, packageInfo, "Raw", unsafePointer, false),
		gotypes.NewField(0, packageInfo, "hidden", anonymousStruct, false),
	}
	typeObject := gotypes.NewTypeName(0, packageInfo, "Record", nil)
	named := gotypes.NewNamed(typeObject, gotypes.NewStruct(fields, nil), nil)
	insert(typeObject)
	valueReceiver := gotypes.NewVar(0, packageInfo, "record", named)
	named.AddMethod(gotypes.NewFunc(0, packageInfo, "Read", gotypes.NewSignatureType(valueReceiver, nil, nil, gotypes.NewTuple(), gotypes.NewTuple(gotypes.NewVar(0, packageInfo, "", integer)), false)))
	pointerReceiver := gotypes.NewVar(0, packageInfo, "record", gotypes.NewPointer(named))
	named.AddMethod(gotypes.NewFunc(0, packageInfo, "Inspect", gotypes.NewSignatureType(pointerReceiver, nil, nil, gotypes.NewTuple(), gotypes.NewTuple(gotypes.NewVar(0, packageInfo, "", anonymousInterface)), false)))
	return packageInfo
}

func assertAuditCount(t *testing.T, name string, got, want GoInteropAuditCount) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %#v, want %#v", name, got, want)
	}
}
