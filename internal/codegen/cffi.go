package codegen

import (
	"encoding/json"
	"fmt"
	"go/format"
	gotoken "go/token"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/product"
)

type CFFIOutput struct {
	Package string
	Source  []byte
}

type cffiManifest struct {
	SchemaVersion         int                        `json:"schemaVersion"`
	Package               string                     `json:"package"`
	Header                string                     `json:"header"`
	CFlags                []string                   `json:"cFlags"`
	LDFlags               []string                   `json:"ldFlags"`
	Targets               []cffiTarget               `json:"targets"`
	ThreadPolicy          string                     `json:"threadPolicy"`
	Functions             []cffiFunction             `json:"functions"`
	Handles               []cffiHandle               `json:"handles"`
	Structs               []cffiStruct               `json:"structs"`
	Enums                 []cffiEnum                 `json:"enums"`
	TaggedUnions          []cffiTaggedUnion          `json:"taggedUnions"`
	Callbacks             []cffiCallback             `json:"callbacks"`
	CallbackRegistrations []cffiCallbackRegistration `json:"callbackRegistrations"`
}

type cffiTarget struct {
	GOOS    string   `json:"goos"`
	GOARCH  string   `json:"goarch"`
	CFlags  []string `json:"cFlags"`
	LDFlags []string `json:"ldFlags"`
}

type cffiHandle struct {
	Name    string `json:"name"`
	CType   string `json:"cType"`
	Release string `json:"release"`
}

type cffiFunction struct {
	Name          string          `json:"name"`
	Symbol        string          `json:"symbol"`
	Parameters    []cffiParameter `json:"parameters"`
	Result        string          `json:"result"`
	ResultElement string          `json:"resultElement"`
	ResultRelease string          `json:"resultRelease"`
	Convention    string          `json:"convention"`
}

type cffiParameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type cffiStruct struct {
	Name   string      `json:"name"`
	CType  string      `json:"cType"`
	Fields []cffiField `json:"fields"`
}

type cffiField struct {
	Name  string `json:"name"`
	CName string `json:"cName"`
	Type  string `json:"type"`
}

type cffiEnum struct {
	Name       string          `json:"name"`
	CType      string          `json:"cType"`
	Underlying string          `json:"underlying"`
	Values     []cffiEnumValue `json:"values"`
}

type cffiEnumValue struct {
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
}

type cffiTaggedUnion struct {
	Name        string             `json:"name"`
	CType       string             `json:"cType"`
	Tag         cffiField          `json:"tag"`
	OverlaidTag bool               `json:"overlaidTag"`
	Variants    []cffiUnionVariant `json:"variants"`
}

type cffiUnionVariant struct {
	Name  string   `json:"name"`
	CName string   `json:"cName"`
	Type  string   `json:"type"`
	Tags  []string `json:"tags"`
}

type cffiCallback struct {
	Name          string          `json:"name"`
	Lifetime      string          `json:"lifetime"`
	Parameters    []cffiParameter `json:"parameters"`
	Result        string          `json:"result"`
	ResultElement string          `json:"resultElement"`
}

type cffiCallbackRegistration struct {
	Name       string          `json:"name"`
	Callback   string          `json:"callback"`
	Parameters []cffiParameter `json:"parameters"`
	Register   string          `json:"register"`
	Unregister string          `json:"unregister"`
}

type cffiScalar struct {
	goType    string
	cgoType   string
	zero      string
	fromC     func(string) string
	toC       func(string) string
	canResult bool
}

var cffiIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var cffiReservedGoNames = map[string]bool{
	"C": true, "fmt": true, "sync": true, "errors": true, "strings": true,
	"StatusError": true, "ErrClosedHandle": true, "kinmokuseiCFFIMutex": true,
	"ErrHandleHasActiveRegistrations": true,
	"ErrEmbeddedNUL":                  true, "ErrNullCString": true,
	"ErrNullOwnedCString": true,
	"ErrNullOwnedBuffer":  true, "ErrOwnedBufferTooLarge": true,
	"ErrNullOwnedArray": true, "ErrOwnedArrayTooLarge": true,
	"ErrNilCallback": true, "ErrClosedCallbackRegistration": true, "CallbackPanicError": true, "CallbackInputError": true,
	"kinmokuseiResult": true, "kinmokuseiError": true,
}

var cffiScalars = map[string]cffiScalar{
	"int8":    {goType: "int8", cgoType: "C.int8_t", zero: "0", fromC: func(value string) string { return "int8(" + value + ")" }, toC: func(value string) string { return "C.int8_t(" + value + ")" }, canResult: true},
	"int16":   {goType: "int16", cgoType: "C.int16_t", zero: "0", fromC: func(value string) string { return "int16(" + value + ")" }, toC: func(value string) string { return "C.int16_t(" + value + ")" }, canResult: true},
	"byte":    {goType: "byte", cgoType: "C.uint8_t", zero: "0", fromC: func(value string) string { return "byte(" + value + ")" }, toC: func(value string) string { return "C.uint8_t(" + value + ")" }, canResult: true},
	"uint16":  {goType: "uint16", cgoType: "C.uint16_t", zero: "0", fromC: func(value string) string { return "uint16(" + value + ")" }, toC: func(value string) string { return "C.uint16_t(" + value + ")" }, canResult: true},
	"uint32":  {goType: "uint32", cgoType: "C.uint32_t", zero: "0", fromC: func(value string) string { return "uint32(" + value + ")" }, toC: func(value string) string { return "C.uint32_t(" + value + ")" }, canResult: true},
	"uint64":  {goType: "uint64", cgoType: "C.uint64_t", zero: "0", fromC: func(value string) string { return "uint64(" + value + ")" }, toC: func(value string) string { return "C.uint64_t(" + value + ")" }, canResult: true},
	"int32":   {goType: "int32", cgoType: "C.int32_t", zero: "0", fromC: func(value string) string { return "int32(" + value + ")" }, toC: func(value string) string { return "C.int32_t(" + value + ")" }, canResult: true},
	"int64":   {goType: "int64", cgoType: "C.int64_t", zero: "0", fromC: func(value string) string { return "int64(" + value + ")" }, toC: func(value string) string { return "C.int64_t(" + value + ")" }, canResult: true},
	"float32": {goType: "float32", cgoType: "C.float", zero: "0", fromC: func(value string) string { return "float32(" + value + ")" }, toC: func(value string) string { return "C.float(" + value + ")" }, canResult: true},
	"float64": {goType: "float64", cgoType: "C.double", zero: "0", fromC: func(value string) string { return "float64(" + value + ")" }, toC: func(value string) string { return "C.double(" + value + ")" }, canResult: true},
	"cInt32":  {goType: "int32", cgoType: "C.int", zero: "0", fromC: func(value string) string { return "int32(" + value + ")" }, toC: func(value string) string { return "C.int(" + value + ")" }, canResult: true},
	"cUint32": {goType: "uint32", cgoType: "C.uint", zero: "0", fromC: func(value string) string { return "uint32(" + value + ")" }, toC: func(value string) string { return "C.uint(" + value + ")" }, canResult: true},
	"boolean": {goType: "bool", cgoType: "C.bool", zero: "false", fromC: func(value string) string { return "bool(" + value + ")" }, toC: func(value string) string { return "C.bool(" + value + ")" }, canResult: true},
	"cstring": {goType: "string", cgoType: "*C.char", zero: `""`, fromC: func(value string) string { return "C.GoString(" + value + ")" }, toC: func(value string) string { return value }, canResult: true},
	"void":    {goType: "", cgoType: "", zero: "", fromC: func(value string) string { return value }, toC: func(value string) string { return value }},
}

// GenerateCFFI emits a private cgo package from a strictly checked manifest.
// The resulting ordinary Go API is intended to be wrapped by Kinmokusei code
// through direct Go interop; C names and representations never enter its type
// system.
func GenerateCFFI(data []byte) (CFFIOutput, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var manifest cffiManifest
	if err := decoder.Decode(&manifest); err != nil {
		return CFFIOutput{}, fmt.Errorf("invalid C FFI manifest: %w", err)
	}
	if err := ensureCFFIEOF(decoder); err != nil {
		return CFFIOutput{}, err
	}
	if err := validateCFFIManifest(manifest); err != nil {
		return CFFIOutput{}, err
	}
	namedTypes := cffiNamedTypes(manifest)
	callbacks := cffiCallbacksByName(manifest)
	handles := cffiHandlesByName(manifest)

	var source strings.Builder
	source.WriteString("// Code generated by " + product.DisplayName + ". DO NOT EDIT.\npackage ")
	source.WriteString(manifest.Package)
	source.WriteString("\n\n/*\n#include <stdint.h>\n#include <stdbool.h>\n#include <stdlib.h>\n")
	if cffiManifestUsesType(manifest, "cInt32") {
		source.WriteString("typedef char kinmokusei_c_int_must_be_32_bits[(sizeof(int) == 4) ? 1 : -1];\n")
	}
	if cffiManifestUsesType(manifest, "cUint32") {
		source.WriteString("typedef char kinmokusei_c_uint_must_be_32_bits[(sizeof(unsigned int) == 4) ? 1 : -1];\n")
	}
	usesRetainedCString := cffiManifestHasRegistrationParameterType(manifest, "retainedCString")
	usesRetainedBytes := cffiManifestHasRegistrationParameterType(manifest, "retainedBytes")
	usesCopiedCallbackBytes := cffiManifestHasCallbackParameterType(manifest, "copiedBytes") || cffiManifestHasCallbackParameterType(manifest, "inoutBytes")
	if cffiManifestHasParameterType(manifest, "cstring") || usesRetainedCString {
		source.WriteString("static inline void kinmokusei_cffi_free_string(char *value) { free(value); }\n")
	}
	if cffiManifestHasParameterType(manifest, "borrowedBytes") || usesRetainedBytes {
		source.WriteString("static inline void kinmokusei_cffi_free_bytes(void *value) { free(value); }\n")
	}
	if len(manifest.CFlags) != 0 {
		source.WriteString("#cgo CFLAGS: ")
		source.WriteString(strings.Join(manifest.CFlags, " "))
		source.WriteByte('\n')
	}
	if len(manifest.LDFlags) != 0 {
		source.WriteString("#cgo LDFLAGS: ")
		source.WriteString(strings.Join(manifest.LDFlags, " "))
		source.WriteByte('\n')
	}
	for _, target := range manifest.Targets {
		constraint := target.GOOS
		if target.GOARCH != "" {
			constraint += "," + target.GOARCH
		}
		if len(target.CFlags) != 0 {
			source.WriteString("#cgo " + constraint + " CFLAGS: " + strings.Join(target.CFlags, " ") + "\n")
		}
		if len(target.LDFlags) != 0 {
			source.WriteString("#cgo " + constraint + " LDFLAGS: " + strings.Join(target.LDFlags, " ") + "\n")
		}
	}
	source.WriteString("#include ")
	source.WriteString(strconv.Quote(manifest.Header))
	source.WriteByte('\n')
	for _, union := range manifest.TaggedUnions {
		generateCFFIUnionCHelpers(&source, union, namedTypes)
	}
	for _, callback := range manifest.Callbacks {
		generateCFFICallbackCHelpers(&source, callback, namedTypes)
	}
	for _, function := range manifest.Functions {
		if cffiFunctionCallback(function, callbacks) != nil {
			generateCFFICallbackFunctionCHelper(&source, function, callbacks, handles, namedTypes)
		}
	}
	for _, registration := range manifest.CallbackRegistrations {
		generateCFFICallbackRegistrationCHelpers(&source, registration, callbacks, handles, namedTypes)
	}
	source.WriteString("*/\nimport \"C\"\n\nimport (\n\t\"fmt\"\n")
	usesCStringInput := cffiManifestHasParameterType(manifest, "cstring") || usesRetainedCString || cffiManifestHasCallbackResultType(manifest, "ownedCString")
	returnsCString := cffiManifestReturnsType(manifest, "cstring")
	returnsOwnedCString := cffiManifestReturnsType(manifest, "ownedCString")
	returnsOwnedBytes := cffiManifestReturnsType(manifest, "ownedBytes")
	returnsOwnedArray := cffiManifestReturnsType(manifest, "ownedArray")
	callbackOwnsBytes := cffiManifestHasCallbackResultType(manifest, "ownedBytes")
	callbackOwnsArray := cffiManifestHasCallbackResultType(manifest, "ownedArray")
	if len(manifest.Handles) != 0 || len(manifest.Callbacks) != 0 || usesCStringInput || returnsCString || returnsOwnedCString || returnsOwnedBytes || returnsOwnedArray {
		source.WriteString("\n\t\"errors\"")
	}
	if usesCStringInput {
		source.WriteString("\n\t\"strings\"")
	}
	if manifest.ThreadPolicy == "threadAffine" {
		source.WriteString("\n\t\"runtime\"")
	}
	if len(manifest.Callbacks) != 0 {
		source.WriteString("\n\t\"runtime/cgo\"")
	}
	if returnsOwnedBytes || returnsOwnedArray || callbackOwnsBytes || callbackOwnsArray || usesRetainedBytes || usesCopiedCallbackBytes {
		source.WriteString("\n\t\"unsafe\"")
	}
	if manifest.ThreadPolicy != "threadSafe" || len(manifest.Handles) != 0 || len(manifest.Callbacks) != 0 {
		source.WriteString("\n\t\"sync\"")
	}
	source.WriteString("\n)\n\n")
	source.WriteString("type StatusError struct { Function string; Code int32 }\n")
	source.WriteString("func (err *StatusError) Error() string { return fmt.Sprintf(\"C FFI %s failed with status %d\", err.Function, err.Code) }\n")
	if manifest.ThreadPolicy == "serialized" {
		source.WriteString("var kinmokuseiCFFIMutex sync.Mutex\n")
	}
	if manifest.ThreadPolicy == "threadAffine" {
		source.WriteString(`type kinmokuseiCFFIRequest struct {
	call func()
	done chan struct{}
	panicValue any
}
var kinmokuseiCFFIOnce sync.Once
var kinmokuseiCFFIRequests chan *kinmokuseiCFFIRequest
func kinmokuseiCFFIDo(call func()) {
	kinmokuseiCFFIOnce.Do(func() {
		kinmokuseiCFFIRequests = make(chan *kinmokuseiCFFIRequest)
		ready := make(chan struct{})
		go func() {
			runtime.LockOSThread()
			close(ready)
			for request := range kinmokuseiCFFIRequests {
				func() {
					defer func() { request.panicValue = recover() }()
					request.call()
				}()
				close(request.done)
			}
		}()
		<-ready
	})
	request := &kinmokuseiCFFIRequest{call: call, done: make(chan struct{})}
	kinmokuseiCFFIRequests <- request
	<-request.done
	if request.panicValue != nil { panic(request.panicValue) }
}
`)
	}
	sortedHandles := append([]cffiHandle(nil), manifest.Handles...)
	sort.SliceStable(sortedHandles, func(left, right int) bool { return sortedHandles[left].Name < sortedHandles[right].Name })
	if len(sortedHandles) != 0 {
		source.WriteString("var ErrClosedHandle = errors.New(\"C FFI handle is nil or closed\")\n")
		source.WriteString("var ErrHandleHasActiveRegistrations = errors.New(\"C FFI handle has active callback registrations\")\n")
	}
	if usesCStringInput {
		source.WriteString("var ErrEmbeddedNUL = errors.New(\"C FFI string contains an embedded NUL byte\")\n")
	}
	if returnsCString {
		source.WriteString("var ErrNullCString = errors.New(\"C FFI returned a null string pointer\")\n")
	}
	if returnsOwnedCString {
		source.WriteString("var ErrNullOwnedCString = errors.New(\"C FFI returned a null owned string pointer\")\n")
	}
	if returnsOwnedBytes {
		source.WriteString("var ErrNullOwnedBuffer = errors.New(\"C FFI returned a null owned buffer with non-zero length\")\n")
		source.WriteString("var ErrOwnedBufferTooLarge = errors.New(\"C FFI returned an owned buffer too large to copy\")\n")
	}
	if returnsOwnedArray {
		source.WriteString("var ErrNullOwnedArray = errors.New(\"C FFI returned a null owned array with non-zero length\")\n")
		source.WriteString("var ErrOwnedArrayTooLarge = errors.New(\"C FFI returned an owned array too large to copy\")\n")
	}
	if len(manifest.Callbacks) != 0 {
		source.WriteString("var ErrNilCallback = errors.New(\"C FFI callback is nil\")\n")
		source.WriteString("type CallbackPanicError struct { Function string; Value any }\n")
		source.WriteString("func (err *CallbackPanicError) Error() string { return fmt.Sprintf(\"C FFI callback used by %s panicked: %v\", err.Function, err.Value) }\n")
		source.WriteString("type CallbackInputError struct { Function string; Parameter string; Reason string }\n")
		source.WriteString("func (err *CallbackInputError) Error() string { return fmt.Sprintf(\"C FFI callback used by %s received invalid parameter %s: %s\", err.Function, err.Parameter, err.Reason) }\n")
	}
	if len(manifest.CallbackRegistrations) != 0 {
		source.WriteString("var ErrClosedCallbackRegistration = errors.New(\"C FFI callback registration is nil or closed\")\n")
	}
	sortedEnums := append([]cffiEnum(nil), manifest.Enums...)
	sort.SliceStable(sortedEnums, func(left, right int) bool { return sortedEnums[left].Name < sortedEnums[right].Name })
	for _, enum := range sortedEnums {
		generateCFFIEnum(&source, enum)
	}
	sortedStructs := append([]cffiStruct(nil), manifest.Structs...)
	sort.SliceStable(sortedStructs, func(left, right int) bool { return sortedStructs[left].Name < sortedStructs[right].Name })
	for _, structure := range sortedStructs {
		generateCFFIStruct(&source, structure, namedTypes)
	}
	sortedUnions := append([]cffiTaggedUnion(nil), manifest.TaggedUnions...)
	sort.SliceStable(sortedUnions, func(left, right int) bool { return sortedUnions[left].Name < sortedUnions[right].Name })
	for _, union := range sortedUnions {
		generateCFFITaggedUnion(&source, union, namedTypes)
	}
	sortedCallbacks := append([]cffiCallback(nil), manifest.Callbacks...)
	sort.SliceStable(sortedCallbacks, func(left, right int) bool { return sortedCallbacks[left].Name < sortedCallbacks[right].Name })
	for _, callback := range sortedCallbacks {
		generateCFFICallback(&source, callback, namedTypes)
	}
	sortedRegistrations := append([]cffiCallbackRegistration(nil), manifest.CallbackRegistrations...)
	sort.SliceStable(sortedRegistrations, func(left, right int) bool { return sortedRegistrations[left].Name < sortedRegistrations[right].Name })
	for _, registration := range sortedRegistrations {
		generateCFFICallbackRegistration(&source, manifest.ThreadPolicy, registration, callbacks, handles, namedTypes)
	}
	for _, handle := range sortedHandles {
		generateCFFIHandle(&source, manifest.ThreadPolicy, handle)
	}
	functions := append([]cffiFunction(nil), manifest.Functions...)
	sort.SliceStable(functions, func(left, right int) bool { return functions[left].Name < functions[right].Name })
	for _, function := range functions {
		generateCFFIFunction(&source, manifest.ThreadPolicy, function, handles, namedTypes, callbacks)
	}
	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return CFFIOutput{}, fmt.Errorf("generated C FFI Go failed formatting: %w\n%s", err, source.String())
	}
	return CFFIOutput{Package: manifest.Package, Source: formatted}, nil
}

func ensureCFFIEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid C FFI manifest: multiple JSON values")
		}
		return fmt.Errorf("invalid C FFI manifest: trailing data: %w", err)
	}
	return nil
}

func validateCFFIManifest(manifest cffiManifest) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("unsupported C FFI manifest schema version %d; expected 1", manifest.SchemaVersion)
	}
	if !validCFFIIdentifier(manifest.Package) {
		return fmt.Errorf("invalid C FFI Go package name %q", manifest.Package)
	}
	if manifest.Header == "" || strings.ContainsAny(manifest.Header, "\r\n\"") {
		return fmt.Errorf("C FFI header must be a non-empty single-line quoted include path")
	}
	if manifest.ThreadPolicy != "threadSafe" && manifest.ThreadPolicy != "serialized" && manifest.ThreadPolicy != "threadAffine" {
		return fmt.Errorf("C FFI threadPolicy must be threadSafe, serialized, or threadAffine")
	}
	for kind, flags := range map[string][]string{"C flag": manifest.CFlags, "linker flag": manifest.LDFlags} {
		if err := validateCFFIFlags(kind, flags); err != nil {
			return err
		}
	}
	knownTargets := map[string]bool{}
	for _, target := range manifest.Targets {
		if !cffiGOOS[target.GOOS] || (target.GOARCH != "" && !cffiGOARCH[target.GOARCH]) {
			return fmt.Errorf("unsupported C FFI target %q/%q", target.GOOS, target.GOARCH)
		}
		key := target.GOOS + "/" + target.GOARCH
		if knownTargets[key] {
			return fmt.Errorf("duplicate C FFI target %q", key)
		}
		knownTargets[key] = true
		if len(target.CFlags) == 0 && len(target.LDFlags) == 0 {
			return fmt.Errorf("C FFI target %q must declare cFlags or ldFlags", key)
		}
		for kind, flags := range map[string][]string{"target C flag": target.CFlags, "target linker flag": target.LDFlags} {
			if err := validateCFFIFlags(kind, flags); err != nil {
				return err
			}
		}
	}
	if len(manifest.Functions) == 0 {
		return fmt.Errorf("C FFI manifest must declare at least one function")
	}
	names := map[string]bool{}
	cTypes := map[string]bool{}
	enums := map[string]cffiEnum{}
	structures := map[string]cffiStruct{}
	unions := map[string]cffiTaggedUnion{}
	for _, enum := range manifest.Enums {
		underlying, scalar := cffiScalars[enum.Underlying]
		if !validCFFIPublicName(enum.Name) || !validCFFIIdentifier(enum.CType) || cTypes[enum.CType] {
			return fmt.Errorf("C FFI enum name %q and C type %q must be identifiers", enum.Name, enum.CType)
		}
		if !scalar || !cffiIntegerType(enum.Underlying) || !underlying.canResult {
			return fmt.Errorf("C FFI enum %q has unsupported integer underlying type %q", enum.Name, enum.Underlying)
		}
		if names[enum.Name] {
			return fmt.Errorf("duplicate C FFI type name %q", enum.Name)
		}
		names[enum.Name] = true
		cTypes[enum.CType] = true
		enums[enum.Name] = enum
	}
	for _, structure := range manifest.Structs {
		if !validCFFIPublicName(structure.Name) || !validCFFIIdentifier(structure.CType) || cTypes[structure.CType] {
			return fmt.Errorf("C FFI struct name %q and C type %q must be identifiers", structure.Name, structure.CType)
		}
		if len(structure.Fields) == 0 {
			return fmt.Errorf("C FFI struct %q must declare at least one field", structure.Name)
		}
		if names[structure.Name] {
			return fmt.Errorf("duplicate C FFI type name %q", structure.Name)
		}
		names[structure.Name] = true
		cTypes[structure.CType] = true
		structures[structure.Name] = structure
	}
	for _, union := range manifest.TaggedUnions {
		if !validCFFIPublicName(union.Name) || !validCFFIIdentifier(union.CType) || names[union.Name] || cTypes[union.CType] {
			return fmt.Errorf("C FFI tagged union name %q and C type %q must be unique exported identifiers", union.Name, union.CType)
		}
		if len(union.Variants) == 0 {
			return fmt.Errorf("C FFI tagged union %q must declare at least one variant", union.Name)
		}
		names[union.Name] = true
		cTypes[union.CType] = true
		unions[union.Name] = union
	}
	for _, enum := range manifest.Enums {
		values := map[string]bool{}
		for _, value := range enum.Values {
			if !validCFFIPublicName(value.Name) || !validCFFIIdentifier(value.Symbol) || values[value.Name] || names[value.Name] {
				return fmt.Errorf("C FFI enum %q has invalid or duplicate value %q", enum.Name, value.Name)
			}
			values[value.Name] = true
			names[value.Name] = true
		}
	}
	for _, structure := range manifest.Structs {
		fields := map[string]bool{}
		cFields := map[string]bool{}
		for _, field := range structure.Fields {
			if !validCFFIPublicName(field.Name) || !validCFFIIdentifier(field.CName) || fields[field.Name] || cFields[field.CName] {
				return fmt.Errorf("C FFI struct %q has invalid or duplicate field %q", structure.Name, field.Name)
			}
			fields[field.Name] = true
			cFields[field.CName] = true
			if !cffiPODFieldType(field.Type, enums, structures) {
				return fmt.Errorf("C FFI struct %q field %q has unsupported POD type %q", structure.Name, field.Name, field.Type)
			}
		}
	}
	for _, union := range manifest.TaggedUnions {
		// Union member names are referenced only inside the generated C helpers.
		// They therefore may be Go keywords (for example the common C field
		// name `type`) even though identifiers referenced through C.foo may not.
		if !validCFFIPublicName(union.Tag.Name) || !validCFFICIdentifier(union.Tag.CName) {
			return fmt.Errorf("C FFI tagged union %q has an invalid tag field", union.Name)
		}
		_, tagEnum := enums[union.Tag.Type]
		if !cffiIntegerType(union.Tag.Type) && !tagEnum {
			return fmt.Errorf("C FFI tagged union %q tag has unsupported integer or enum type %q", union.Name, union.Tag.Type)
		}
		fieldNames := map[string]bool{union.Tag.Name: true}
		cNames := map[string]bool{union.Tag.CName: true}
		tags := map[string]bool{}
		for _, variant := range union.Variants {
			if !validCFFIPublicName(variant.Name) || !validCFFICIdentifier(variant.CName) || fieldNames[variant.Name] || cNames[variant.CName] || !cffiPODFieldType(variant.Type, enums, structures) || len(variant.Tags) == 0 {
				return fmt.Errorf("C FFI tagged union %q has invalid variant %q", union.Name, variant.Name)
			}
			if union.OverlaidTag {
				if _, structure := structures[variant.Type]; !structure {
					return fmt.Errorf("C FFI tagged union %q with overlaidTag requires POD struct variant %q", union.Name, variant.Name)
				}
			}
			fieldNames[variant.Name] = true
			cNames[variant.CName] = true
			for _, tag := range variant.Tags {
				if !validCFFIIdentifier(tag) || tags[tag] {
					return fmt.Errorf("C FFI tagged union %q has invalid or duplicate tag symbol %q", union.Name, tag)
				}
				tags[tag] = true
			}
		}
	}
	if err := validateCFFIStructCycles(structures); err != nil {
		return err
	}
	handles := map[string]cffiHandle{}
	for _, handle := range manifest.Handles {
		if !validCFFIPublicName(handle.Name) || !validCFFIIdentifier(handle.CType) || !validCFFIIdentifier(handle.Release) || cTypes[handle.CType] {
			return fmt.Errorf("C FFI handle name %q, C type %q, and release symbol %q must be identifiers", handle.Name, handle.CType, handle.Release)
		}
		if _, builtin := cffiScalars[handle.Name]; builtin || names[handle.Name] || handles[handle.Name].Name != "" {
			return fmt.Errorf("duplicate or reserved C FFI handle name %q", handle.Name)
		}
		handles[handle.Name] = handle
		names[handle.Name] = true
		cTypes[handle.CType] = true
	}
	callbacks := map[string]cffiCallback{}
	for _, callback := range manifest.Callbacks {
		if !validCFFIPublicName(callback.Name) || names[callback.Name] {
			return fmt.Errorf("C FFI callback name %q must be a unique exported identifier", callback.Name)
		}
		if callback.Lifetime != "callScoped" && callback.Lifetime != "registered" {
			return fmt.Errorf("C FFI callback %q lifetime must be callScoped or registered", callback.Name)
		}
		result, scalarResult := cffiScalars[callback.Result]
		_, enumResult := enums[callback.Result]
		ownedBytesResult := callback.Result == "ownedBytes"
		ownedCStringResult := callback.Result == "ownedCString"
		ownedArrayResult := callback.Result == "ownedArray"
		ownedResult := ownedBytesResult || ownedCStringResult || ownedArrayResult
		if ownedResult && callback.Lifetime != "registered" {
			return fmt.Errorf("C FFI callback %q %s result requires registered lifetime", callback.Name, callback.Result)
		}
		if ownedArrayResult {
			if !cffiPODFieldType(callback.ResultElement, enums, structures) {
				return fmt.Errorf("C FFI ownedArray callback %q requires a supported resultElement", callback.Name)
			}
		} else if callback.ResultElement != "" {
			return fmt.Errorf("C FFI callback %q may declare resultElement only for ownedArray", callback.Name)
		}
		if !ownedResult && (!scalarResult || callback.Result == "cstring" || (callback.Result != "void" && !result.canResult)) && !enumResult {
			return fmt.Errorf("C FFI callback %q has unsupported scalar or enum result type %q", callback.Name, callback.Result)
		}
		parameters := map[string]bool{}
		for _, parameter := range callback.Parameters {
			if !validCFFIIdentifier(parameter.Name) || cffiReservedGoNames[parameter.Name] || parameters[parameter.Name] {
				return fmt.Errorf("C FFI callback %q has invalid or duplicate parameter %q", callback.Name, parameter.Name)
			}
			parameters[parameter.Name] = true
			_, union := unions[parameter.Type]
			copied := parameter.Type == "copiedCString" || parameter.Type == "nullableCopiedCString" || parameter.Type == "copiedBytes" || parameter.Type == "inoutBytes"
			if !cffiPODFieldType(parameter.Type, enums, structures) && !union && !copied {
				return fmt.Errorf("C FFI callback %q parameter %q has unsupported scalar, enum, POD, or tagged-union type %q", callback.Name, parameter.Name, parameter.Type)
			}
		}
		names[callback.Name] = true
		callbacks[callback.Name] = callback
	}
	registeredCallbacks := map[string]bool{}
	for _, registration := range manifest.CallbackRegistrations {
		callback, exists := callbacks[registration.Callback]
		registerName := "Register" + registration.Name
		if !validCFFIPublicName(registration.Name) || names[registration.Name] || names[registerName] {
			return fmt.Errorf("C FFI callback registration name %q must be a unique exported identifier", registration.Name)
		}
		if !exists || callback.Lifetime != "registered" {
			return fmt.Errorf("C FFI callback registration %q must reference a registered callback", registration.Name)
		}
		if !validCFFIIdentifier(registration.Register) || !validCFFIIdentifier(registration.Unregister) {
			return fmt.Errorf("C FFI callback registration %q register/unregister symbols must be identifiers", registration.Name)
		}
		parameters := map[string]bool{}
		handleParameters := 0
		for _, parameter := range registration.Parameters {
			if !validCFFIIdentifier(parameter.Name) || cffiReservedGoNames[parameter.Name] || parameter.Name == "callback" || parameters[parameter.Name] {
				return fmt.Errorf("C FFI callback registration %q has invalid or duplicate parameter %q", registration.Name, parameter.Name)
			}
			parameters[parameter.Name] = true
			_, union := unions[parameter.Type]
			_, handle := handles[parameter.Type]
			retained := parameter.Type == "retainedCString" || parameter.Type == "retainedBytes"
			if !cffiPODFieldType(parameter.Type, enums, structures) && !union && !handle && !retained {
				return fmt.Errorf("C FFI callback registration %q parameter %q has unsupported value type %q", registration.Name, parameter.Name, parameter.Type)
			}
			if handle {
				handleParameters++
			}
		}
		if handleParameters > 1 {
			return fmt.Errorf("C FFI callback registration %q may use at most one handle parameter in schema 1", registration.Name)
		}
		names[registration.Name] = true
		names[registerName] = true
		registeredCallbacks[registration.Callback] = true
	}
	for _, callback := range manifest.Callbacks {
		if callback.Lifetime == "registered" && !registeredCallbacks[callback.Name] {
			return fmt.Errorf("C FFI registered callback %q requires a callbackRegistration", callback.Name)
		}
	}
	for _, function := range manifest.Functions {
		if !validCFFIPublicName(function.Name) || !validCFFIIdentifier(function.Symbol) {
			return fmt.Errorf("C FFI function name %q and symbol %q must be identifiers", function.Name, function.Symbol)
		}
		if names[function.Name] {
			return fmt.Errorf("duplicate C FFI function name %q", function.Name)
		}
		names[function.Name] = true
		result, scalarResult := cffiScalars[function.Result]
		ownedCStringResult := function.Result == "ownedCString"
		ownedBytesResult := function.Result == "ownedBytes"
		ownedArrayResult := function.Result == "ownedArray"
		_, namedResult := enums[function.Result]
		if _, structureResult := structures[function.Result]; structureResult {
			namedResult = true
		}
		if _, unionResult := unions[function.Result]; unionResult {
			namedResult = true
		}
		_, handleResult := handles[function.Result]
		if (!scalarResult || (function.Result != "void" && !result.canResult)) && !namedResult && !handleResult && !ownedCStringResult && !ownedBytesResult && !ownedArrayResult {
			return fmt.Errorf("C FFI function %q has unsupported result type %q", function.Name, function.Result)
		}
		if ownedCStringResult {
			if function.Convention != "statusOut" || !validCFFIIdentifier(function.ResultRelease) {
				return fmt.Errorf("C FFI ownedCString function %q must use statusOut and declare a resultRelease C symbol", function.Name)
			}
			if function.ResultElement != "" {
				return fmt.Errorf("C FFI ownedCString function %q may not declare resultElement", function.Name)
			}
		} else if ownedBytesResult {
			if function.Convention != "statusOut" || !validCFFIIdentifier(function.ResultRelease) {
				return fmt.Errorf("C FFI ownedBytes function %q must use statusOut and declare a resultRelease C symbol", function.Name)
			}
			if function.ResultElement != "" {
				return fmt.Errorf("C FFI ownedBytes function %q may not declare resultElement", function.Name)
			}
		} else if ownedArrayResult {
			if function.Convention != "statusOut" || !validCFFIIdentifier(function.ResultRelease) || !cffiPODFieldType(function.ResultElement, enums, structures) {
				return fmt.Errorf("C FFI ownedArray function %q must use statusOut and declare a supported resultElement and resultRelease C symbol", function.Name)
			}
		} else if function.ResultRelease != "" || function.ResultElement != "" {
			return fmt.Errorf("C FFI function %q may declare resultElement/resultRelease only for ownedCString, ownedBytes, or ownedArray", function.Name)
		}
		if function.Convention != "direct" && function.Convention != "statusOut" && function.Convention != "status" {
			return fmt.Errorf("C FFI function %q convention must be direct, statusOut, or status", function.Name)
		}
		if function.Convention == "statusOut" && function.Result == "void" {
			return fmt.Errorf("C FFI statusOut function %q requires a non-void result", function.Name)
		}
		if function.Convention == "status" && function.Result != "void" {
			return fmt.Errorf("C FFI status function %q requires a void result", function.Name)
		}
		parameters := map[string]bool{}
		handleParameters := 0
		callbackParameters := 0
		for _, parameter := range function.Parameters {
			if !validCFFIIdentifier(parameter.Name) || cffiReservedGoNames[parameter.Name] || parameter.Name == "output" || parameter.Name == "status" || parameters[parameter.Name] {
				return fmt.Errorf("C FFI function %q has invalid or duplicate parameter %q", function.Name, parameter.Name)
			}
			parameters[parameter.Name] = true
			typeInfo, scalar := cffiScalars[parameter.Type]
			_, named := enums[parameter.Type]
			if _, structure := structures[parameter.Type]; structure {
				named = true
			}
			if _, union := unions[parameter.Type]; union {
				named = true
			}
			_, handle := handles[parameter.Type]
			callbackType, callback := callbacks[parameter.Type]
			if parameter.Type == "borrowedBytes" {
				continue
			}
			if (!scalar || parameter.Type == "void" || typeInfo.goType == "") && !named && !handle && !callback {
				return fmt.Errorf("C FFI function %q parameter %q has unsupported type %q", function.Name, parameter.Name, parameter.Type)
			}
			if handle {
				handleParameters++
			}
			if callback {
				if callbackType.Lifetime != "callScoped" {
					return fmt.Errorf("C FFI function %q may use only callScoped callback parameters", function.Name)
				}
				callbackParameters++
			}
		}
		if handleResult && function.Convention != "statusOut" {
			return fmt.Errorf("C FFI function %q returning a handle must use statusOut", function.Name)
		}
		if handleParameters != 0 && function.Convention != "statusOut" && function.Convention != "status" {
			return fmt.Errorf("C FFI function %q using handle parameters must use statusOut or status", function.Name)
		}
		if handleParameters > 1 {
			return fmt.Errorf("C FFI function %q may use at most one handle parameter in schema 1", function.Name)
		}
		if callbackParameters > 1 {
			return fmt.Errorf("C FFI function %q may use at most one callScoped callback parameter in schema 1", function.Name)
		}
		if callbackParameters != 0 && (ownedCStringResult || ownedBytesResult || ownedArrayResult || handleResult || returnsCStringResult(function)) {
			return fmt.Errorf("C FFI function %q may not combine a callScoped callback with owned, handle, or string results in schema 1", function.Name)
		}
	}
	return nil
}

func validCFFIIdentifier(name string) bool {
	return cffiIdentifier.MatchString(name) && gotoken.Lookup(name) == gotoken.IDENT
}

func validCFFICIdentifier(name string) bool {
	return cffiIdentifier.MatchString(name) && !cffiCKeywords[name]
}

var cffiCKeywords = map[string]bool{
	"auto": true, "break": true, "case": true, "char": true, "const": true,
	"continue": true, "default": true, "do": true, "double": true, "else": true,
	"enum": true, "extern": true, "float": true, "for": true, "goto": true,
	"if": true, "inline": true, "int": true, "long": true, "register": true,
	"restrict": true, "return": true, "short": true, "signed": true, "sizeof": true,
	"static": true, "struct": true, "switch": true, "typedef": true, "union": true,
	"unsigned": true, "void": true, "volatile": true, "while": true,
	"_Alignas": true, "_Alignof": true, "_Atomic": true, "_Bool": true,
	"_Complex": true, "_Generic": true, "_Imaginary": true, "_Noreturn": true,
	"_Static_assert": true, "_Thread_local": true,
}

func validCFFIPublicName(name string) bool {
	return validCFFIIdentifier(name) && name[0] >= 'A' && name[0] <= 'Z' && !cffiReservedGoNames[name]
}

var cffiGOOS = map[string]bool{"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true, "illumos": true, "ios": true, "linux": true, "netbsd": true, "openbsd": true, "solaris": true, "windows": true}
var cffiGOARCH = map[string]bool{"386": true, "amd64": true, "arm": true, "arm64": true, "loong64": true, "ppc64": true, "ppc64le": true, "riscv64": true, "s390x": true}

func validateCFFIFlags(kind string, flags []string) error {
	for _, flag := range flags {
		if flag == "" || strings.ContainsAny(flag, "\r\n") || strings.Contains(flag, "/*") || strings.Contains(flag, "*/") {
			return fmt.Errorf("%s must be a non-empty single-line value", kind)
		}
	}
	return nil
}

func cffiIntegerType(name string) bool {
	switch name {
	case "int8", "int16", "byte", "uint16", "uint32", "uint64", "int32", "int64", "cInt32", "cUint32":
		return true
	default:
		return false
	}
}

func cffiPODFieldType(name string, enums map[string]cffiEnum, structures map[string]cffiStruct) bool {
	if scalar, exists := cffiScalars[name]; exists {
		return name != "void" && name != "cstring" && scalar.goType != ""
	}
	if _, exists := enums[name]; exists {
		return true
	}
	_, exists := structures[name]
	return exists
}

func validateCFFIStructCycles(structures map[string]cffiStruct) error {
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("C FFI struct %q has a recursive by-value field cycle", name)
		}
		if state[name] == 2 {
			return nil
		}
		state[name] = 1
		for _, field := range structures[name].Fields {
			if _, nested := structures[field.Type]; nested {
				if err := visit(field.Type); err != nil {
					return err
				}
			}
		}
		state[name] = 2
		return nil
	}
	for name := range structures {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func cffiNamedTypes(manifest cffiManifest) map[string]cffiScalar {
	types := map[string]cffiScalar{}
	for _, enum := range manifest.Enums {
		name := enum.Name
		cType := enum.CType
		types[name] = cffiScalar{
			goType: name, cgoType: "C." + cType, zero: name + "(0)", canResult: true,
			fromC: func(value string) string { return name + "(" + value + ")" },
			toC:   func(value string) string { return "C." + cType + "(" + value + ")" },
		}
	}
	for _, structure := range manifest.Structs {
		name := structure.Name
		cType := structure.CType
		types[name] = cffiScalar{
			goType: name, cgoType: "C." + cType, zero: name + "{}", canResult: true,
			fromC: func(value string) string { return "kinmokuseiCFFIFrom" + name + "(" + value + ")" },
			toC:   func(value string) string { return "kinmokuseiCFFITo" + name + "(" + value + ")" },
		}
	}
	for _, union := range manifest.TaggedUnions {
		name := union.Name
		cType := union.CType
		types[name] = cffiScalar{
			goType: name, cgoType: "C." + cType, zero: name + "{}", canResult: true,
			fromC: func(value string) string { return "kinmokuseiCFFIFrom" + name + "(" + value + ")" },
			toC:   func(value string) string { return "kinmokuseiCFFITo" + name + "(" + value + ")" },
		}
	}
	return types
}

func cffiCallbacksByName(manifest cffiManifest) map[string]cffiCallback {
	callbacks := make(map[string]cffiCallback, len(manifest.Callbacks))
	for _, callback := range manifest.Callbacks {
		callbacks[callback.Name] = callback
	}
	return callbacks
}

func cffiHandlesByName(manifest cffiManifest) map[string]cffiHandle {
	handles := make(map[string]cffiHandle, len(manifest.Handles))
	for _, handle := range manifest.Handles {
		handles[handle.Name] = handle
	}
	return handles
}

func cffiFunctionCallback(function cffiFunction, callbacks map[string]cffiCallback) *cffiCallback {
	for _, parameter := range function.Parameters {
		if callback, exists := callbacks[parameter.Type]; exists {
			copy := callback
			return &copy
		}
	}
	return nil
}

func returnsCStringResult(function cffiFunction) bool {
	return function.Result == "cstring" || function.Result == "ownedCString"
}

func cffiManifestUsesType(manifest cffiManifest, target string) bool {
	for _, function := range manifest.Functions {
		if function.Result == target || cffiFunctionHasType(function, target) {
			return true
		}
	}
	for _, enum := range manifest.Enums {
		if enum.Underlying == target {
			return true
		}
	}
	for _, structure := range manifest.Structs {
		for _, field := range structure.Fields {
			if field.Type == target {
				return true
			}
		}
	}
	for _, union := range manifest.TaggedUnions {
		if union.Tag.Type == target {
			return true
		}
		for _, variant := range union.Variants {
			if variant.Type == target {
				return true
			}
		}
	}
	for _, callback := range manifest.Callbacks {
		if callback.Result == target || callback.ResultElement == target {
			return true
		}
		for _, parameter := range callback.Parameters {
			if parameter.Type == target {
				return true
			}
		}
	}
	for _, registration := range manifest.CallbackRegistrations {
		for _, parameter := range registration.Parameters {
			if parameter.Type == target {
				return true
			}
		}
	}
	return false
}

func cffiManifestReturnsType(manifest cffiManifest, target string) bool {
	for _, function := range manifest.Functions {
		if function.Result == target {
			return true
		}
	}
	return false
}

func cffiManifestHasCallbackResultType(manifest cffiManifest, target string) bool {
	for _, callback := range manifest.Callbacks {
		if callback.Result == target {
			return true
		}
	}
	return false
}

func cffiManifestHasParameterType(manifest cffiManifest, target string) bool {
	for _, function := range manifest.Functions {
		if cffiFunctionHasType(function, target) {
			return true
		}
	}
	return false
}

func cffiManifestHasRegistrationParameterType(manifest cffiManifest, target string) bool {
	for _, registration := range manifest.CallbackRegistrations {
		for _, parameter := range registration.Parameters {
			if parameter.Type == target {
				return true
			}
		}
	}
	return false
}

func cffiManifestHasCallbackParameterType(manifest cffiManifest, target string) bool {
	for _, callback := range manifest.Callbacks {
		for _, parameter := range callback.Parameters {
			if parameter.Type == target {
				return true
			}
		}
	}
	return false
}

func cffiFunctionHasType(function cffiFunction, target string) bool {
	for _, parameter := range function.Parameters {
		if parameter.Type == target {
			return true
		}
	}
	return false
}

func generateCFFIEnum(source *strings.Builder, enum cffiEnum) {
	underlying := cffiScalars[enum.Underlying]
	source.WriteString("\ntype " + enum.Name + " " + underlying.goType + "\n")
	if len(enum.Values) == 0 {
		return
	}
	values := append([]cffiEnumValue(nil), enum.Values...)
	sort.SliceStable(values, func(left, right int) bool { return values[left].Name < values[right].Name })
	source.WriteString("const (\n")
	for _, value := range values {
		source.WriteString(value.Name + " " + enum.Name + " = " + enum.Name + "(C." + value.Symbol + ")\n")
	}
	source.WriteString(")\n")
}

func generateCFFIUnionCHelpers(source *strings.Builder, union cffiTaggedUnion, namedTypes map[string]cffiScalar) {
	tag := cffiTypeInfo(union.Tag.Type, namedTypes)
	prefix := "kinmokusei_cffi_" + union.Name
	source.WriteString("static inline " + strings.TrimPrefix(tag.cgoType, "C.") + " " + prefix + "_get_tag(const " + union.CType + " *value) { return value->" + union.Tag.CName + "; }\n")
	source.WriteString("static inline void " + prefix + "_set_tag(" + union.CType + " *value, " + strings.TrimPrefix(tag.cgoType, "C.") + " tag) { value->" + union.Tag.CName + " = tag; }\n")
	for _, variant := range union.Variants {
		typeInfo := cffiTypeInfo(variant.Type, namedTypes)
		cType := strings.TrimPrefix(typeInfo.cgoType, "C.")
		source.WriteString("static inline " + cType + " " + prefix + "_get_" + variant.Name + "(const " + union.CType + " *value) { return value->" + variant.CName + "; }\n")
		source.WriteString("static inline void " + prefix + "_set_" + variant.Name + "(" + union.CType + " *value, " + cType + " item) { value->" + variant.CName + " = item; }\n")
	}
}

func cffiCType(typeInfo cffiScalar) string {
	typeName := typeInfo.cgoType
	if strings.HasPrefix(typeName, "*C.") {
		return strings.TrimPrefix(typeName, "*C.") + " *"
	}
	if typeName == "C.uint" {
		return "unsigned int"
	}
	return strings.TrimPrefix(typeName, "C.")
}

func generateCFFICallbackCHelpers(source *strings.Builder, callback cffiCallback, namedTypes map[string]cffiScalar) {
	prefix := "kinmokusei_cffi_callback_" + callback.Name
	ownedBytesResult := callback.Result == "ownedBytes"
	ownedCStringResult := callback.Result == "ownedCString"
	ownedArrayResult := callback.Result == "ownedArray"
	result := cffiTypeInfo(callback.Result, namedTypes)
	resultType := "void"
	if ownedBytesResult {
		resultType = "uint8_t *"
	} else if ownedCStringResult {
		resultType = "char *"
	} else if ownedArrayResult {
		resultType = cffiCType(cffiTypeInfo(callback.ResultElement, namedTypes)) + " *"
	} else if callback.Result != "void" {
		resultType = cffiCType(result)
	}
	parameters := make([]string, 0, len(callback.Parameters)+1)
	arguments := make([]string, 0, len(callback.Parameters)+1)
	exportedParameters := []string{"uintptr_t context"}
	for index, parameter := range callback.Parameters {
		name := "value" + strconv.Itoa(index)
		switch parameter.Type {
		case "copiedCString", "nullableCopiedCString":
			parameters = append(parameters, "const char *"+name)
			arguments = append(arguments, "(char *)"+name)
			exportedParameters = append(exportedParameters, "char *"+name)
		case "copiedBytes":
			parameters = append(parameters, "const uint8_t *"+name, "size_t "+name+"_length")
			arguments = append(arguments, "(uint8_t *)"+name, name+"_length")
			exportedParameters = append(exportedParameters, "uint8_t *"+name, "size_t "+name+"_length")
		case "inoutBytes":
			parameters = append(parameters, "uint8_t *"+name, "size_t "+name+"_length")
			arguments = append(arguments, name, name+"_length")
			exportedParameters = append(exportedParameters, "uint8_t *"+name, "size_t "+name+"_length")
		default:
			typeInfo := cffiTypeInfo(parameter.Type, namedTypes)
			parameters = append(parameters, cffiCType(typeInfo)+" "+name)
			arguments = append(arguments, name)
			exportedParameters = append(exportedParameters, cffiCType(typeInfo)+" "+name)
		}
	}
	if ownedBytesResult || ownedArrayResult {
		parameters = append(parameters, "size_t *output_length")
		exportedParameters = append(exportedParameters, "size_t *output_length")
		arguments = append(arguments, "output_length")
	}
	parameters = append(parameters, "void *context")
	source.WriteString("typedef " + resultType + " (*" + prefix + "_fn)(" + strings.Join(parameters, ", ") + ");\n")
	source.WriteString("extern " + resultType + " " + prefix + "_go(" + strings.Join(exportedParameters, ", ") + ");\n")
	callArguments := append([]string{"(uintptr_t)context"}, arguments...)
	source.WriteString("static inline " + resultType + " " + prefix + "_bridge(" + strings.Join(parameters, ", ") + ") { ")
	if callback.Result != "void" {
		source.WriteString("return ")
	}
	source.WriteString(prefix + "_go(" + strings.Join(callArguments, ", ") + "); }\n")
	if ownedBytesResult {
		source.WriteString("typedef void (*" + prefix + "_release_fn)(uint8_t *value);\n")
		source.WriteString("static inline void " + prefix + "_release(uint8_t *value) { free(value); }\n")
	} else if ownedCStringResult {
		source.WriteString("typedef void (*" + prefix + "_release_fn)(char *value);\n")
		source.WriteString("static inline void " + prefix + "_release(char *value) { free(value); }\n")
	} else if ownedArrayResult {
		elementType := cffiCType(cffiTypeInfo(callback.ResultElement, namedTypes))
		source.WriteString("typedef void (*" + prefix + "_release_fn)(" + elementType + " *value);\n")
		source.WriteString("static inline void " + prefix + "_release(" + elementType + " *value) { free(value); }\n")
	}
}

func generateCFFICallbackFunctionCHelper(source *strings.Builder, function cffiFunction, callbacks map[string]cffiCallback, handles map[string]cffiHandle, namedTypes map[string]cffiScalar) {
	resultType := "int32_t"
	if function.Convention == "direct" {
		if function.Result == "void" {
			resultType = "void"
		} else {
			resultType = cffiCType(cffiTypeInfo(function.Result, namedTypes))
		}
	}
	parameters := []string{}
	arguments := []string{}
	for index, parameter := range function.Parameters {
		name := "value" + strconv.Itoa(index)
		if callback, exists := callbacks[parameter.Type]; exists {
			parameters = append(parameters, "uintptr_t "+name)
			arguments = append(arguments, "kinmokusei_cffi_callback_"+callback.Name+"_bridge", "(void *)(uintptr_t)"+name)
		} else if handle, exists := handles[parameter.Type]; exists {
			parameters = append(parameters, handle.CType+" *"+name)
			arguments = append(arguments, name)
		} else if parameter.Type == "borrowedBytes" {
			parameters = append(parameters, "uint8_t *"+name, "size_t "+name+"_length")
			arguments = append(arguments, name, name+"_length")
		} else {
			typeInfo := cffiTypeInfo(parameter.Type, namedTypes)
			parameters = append(parameters, cffiCType(typeInfo)+" "+name)
			arguments = append(arguments, name)
		}
	}
	if function.Convention == "statusOut" {
		result := cffiTypeInfo(function.Result, namedTypes)
		parameters = append(parameters, cffiCType(result)+" *output")
		arguments = append(arguments, "output")
	}
	source.WriteString("static inline " + resultType + " kinmokusei_cffi_call_" + function.Name + "(" + strings.Join(parameters, ", ") + ") { ")
	if resultType != "void" {
		source.WriteString("return ")
	}
	source.WriteString(function.Symbol + "(" + strings.Join(arguments, ", ") + "); }\n")
}

func generateCFFICallbackRegistrationCHelpers(source *strings.Builder, registration cffiCallbackRegistration, callbacks map[string]cffiCallback, handles map[string]cffiHandle, namedTypes map[string]cffiScalar) {
	callback := callbacks[registration.Callback]
	bridge := "kinmokusei_cffi_callback_" + callback.Name + "_bridge"
	parameters := make([]string, 0, len(registration.Parameters)+1)
	arguments := make([]string, 0, len(registration.Parameters)+2)
	for index, parameter := range registration.Parameters {
		name := "value" + strconv.Itoa(index)
		if handle, exists := handles[parameter.Type]; exists {
			parameters = append(parameters, handle.CType+" *"+name)
			arguments = append(arguments, name)
		} else if parameter.Type == "retainedCString" {
			parameters = append(parameters, "char *"+name)
			arguments = append(arguments, name)
		} else if parameter.Type == "retainedBytes" {
			parameters = append(parameters, "uint8_t *"+name, "size_t "+name+"_length")
			arguments = append(arguments, name, name+"_length")
		} else {
			typeInfo := cffiTypeInfo(parameter.Type, namedTypes)
			parameters = append(parameters, cffiCType(typeInfo)+" "+name)
			arguments = append(arguments, name)
		}
	}
	parameters = append(parameters, "uintptr_t context")
	arguments = append(arguments, bridge)
	if callback.Result == "ownedBytes" || callback.Result == "ownedCString" || callback.Result == "ownedArray" {
		arguments = append(arguments, "kinmokusei_cffi_callback_"+callback.Name+"_release")
	}
	arguments = append(arguments, "(void *)(uintptr_t)context")
	for _, operation := range []struct {
		name   string
		symbol string
	}{
		{"register", registration.Register},
		{"unregister", registration.Unregister},
	} {
		source.WriteString("static inline int32_t kinmokusei_cffi_" + operation.name + "_" + registration.Name + "(" + strings.Join(parameters, ", ") + ") { return " + operation.symbol + "(" + strings.Join(arguments, ", ") + "); }\n")
	}
}

func generateCFFITaggedUnion(source *strings.Builder, union cffiTaggedUnion, namedTypes map[string]cffiScalar) {
	tag := cffiTypeInfo(union.Tag.Type, namedTypes)
	prefix := "C.kinmokusei_cffi_" + union.Name
	source.WriteString("\ntype " + union.Name + " struct {\n" + union.Tag.Name + " " + tag.goType + "\n")
	for _, variant := range union.Variants {
		typeInfo := cffiTypeInfo(variant.Type, namedTypes)
		source.WriteString(variant.Name + " " + typeInfo.goType + "\n")
	}
	source.WriteString("}\n")
	source.WriteString("func kinmokuseiCFFIFrom" + union.Name + "(value C." + union.CType + ") " + union.Name + " {\n")
	source.WriteString("var output " + union.Name + "\n")
	source.WriteString("output." + union.Tag.Name + " = " + tag.fromC(prefix+"_get_tag(&value)") + "\n")
	source.WriteString("switch output." + union.Tag.Name + " {\n")
	for _, variant := range union.Variants {
		typeInfo := cffiTypeInfo(variant.Type, namedTypes)
		source.WriteString("case " + cffiUnionGoTags(tag.goType, variant.Tags) + ":\n")
		source.WriteString("output." + variant.Name + " = " + typeInfo.fromC(prefix+"_get_"+variant.Name+"(&value)") + "\n")
	}
	source.WriteString("}\nreturn output\n}\n")
	source.WriteString("func kinmokuseiCFFITo" + union.Name + "(value " + union.Name + ") C." + union.CType + " {\n")
	source.WriteString("var output C." + union.CType + "\n")
	source.WriteString("switch value." + union.Tag.Name + " {\n")
	for _, variant := range union.Variants {
		typeInfo := cffiTypeInfo(variant.Type, namedTypes)
		source.WriteString("case " + cffiUnionGoTags(tag.goType, variant.Tags) + ":\n")
		source.WriteString(prefix + "_set_" + variant.Name + "(&output, " + typeInfo.toC("value."+variant.Name) + ")\n")
	}
	source.WriteString("}\n")
	source.WriteString(prefix + "_set_tag(&output, " + tag.toC("value."+union.Tag.Name) + ")\n")
	source.WriteString("return output\n}\n")
}

func cffiUnionGoTags(goType string, symbols []string) string {
	values := make([]string, len(symbols))
	for index, symbol := range symbols {
		values[index] = goType + "(C." + symbol + ")"
	}
	return strings.Join(values, ", ")
}

func generateCFFICallback(source *strings.Builder, callback cffiCallback, namedTypes map[string]cffiScalar) {
	ownedBytesResult := callback.Result == "ownedBytes"
	ownedCStringResult := callback.Result == "ownedCString"
	ownedArrayResult := callback.Result == "ownedArray"
	ownedResult := ownedBytesResult || ownedCStringResult || ownedArrayResult
	result := cffiTypeInfo(callback.Result, namedTypes)
	arrayElement := cffiTypeInfo(callback.ResultElement, namedTypes)
	source.WriteString("\ntype " + callback.Name + " func(")
	for index, parameter := range callback.Parameters {
		if index != 0 {
			source.WriteString(", ")
		}
		goType := ""
		switch parameter.Type {
		case "copiedCString":
			goType = "string"
		case "nullableCopiedCString":
			goType = "*string"
		case "copiedBytes", "inoutBytes":
			goType = "[]byte"
		default:
			goType = cffiTypeInfo(parameter.Type, namedTypes).goType
		}
		source.WriteString(parameter.Name + " " + goType)
	}
	source.WriteByte(')')
	if ownedBytesResult {
		source.WriteString(" []byte")
	} else if ownedCStringResult {
		source.WriteString(" string")
	} else if ownedArrayResult {
		source.WriteString(" []" + arrayElement.goType)
	} else if callback.Result != "void" {
		source.WriteString(" " + result.goType)
	}
	source.WriteByte('\n')
	stateType := "kinmokuseiCFFI" + callback.Name + "State"
	stateFields := "mutex sync.Mutex; callback " + callback.Name + "; failureKind uint8; panicValue any; inputParameter string; inputReason string"
	if callback.Lifetime == "registered" {
		stateFields += "; closing bool; inFlight sync.WaitGroup"
	}
	source.WriteString("type " + stateType + " struct { " + stateFields + " }\n")
	source.WriteString("func (state *" + stateType + ") recordPanic(value any) { state.mutex.Lock(); defer state.mutex.Unlock(); if state.failureKind == 0 { state.failureKind = 1; state.panicValue = value } }\n")
	source.WriteString("func (state *" + stateType + ") recordInputError(parameter string, reason string) { state.mutex.Lock(); defer state.mutex.Unlock(); if state.failureKind == 0 { state.failureKind = 2; state.inputParameter = parameter; state.inputReason = reason } }\n")
	source.WriteString("func (state *" + stateType + ") hasFailed() bool { state.mutex.Lock(); defer state.mutex.Unlock(); return state.failureKind != 0 }\n")
	source.WriteString("func (state *" + stateType + ") callbackError(function string) error { state.mutex.Lock(); defer state.mutex.Unlock(); if state.failureKind == 1 { return &CallbackPanicError{Function: function, Value: state.panicValue} }; if state.failureKind == 2 { return &CallbackInputError{Function: function, Parameter: state.inputParameter, Reason: state.inputReason} }; return nil }\n")
	if callback.Lifetime == "registered" {
		source.WriteString("func (state *" + stateType + ") begin() bool { state.mutex.Lock(); defer state.mutex.Unlock(); if state.closing || state.failureKind != 0 { return false }; state.inFlight.Add(1); return true }\n")
		source.WriteString("func (state *" + stateType + ") end() { state.inFlight.Done() }\n")
		source.WriteString("func (state *" + stateType + ") stop() { state.mutex.Lock(); state.closing = true; state.mutex.Unlock() }\n")
		source.WriteString("func (state *" + stateType + ") resume() { state.mutex.Lock(); state.closing = false; state.mutex.Unlock() }\n")
		source.WriteString("func (state *" + stateType + ") wait() { state.inFlight.Wait() }\n")
	}
	exportName := "kinmokusei_cffi_callback_" + callback.Name + "_go"
	source.WriteString("//export " + exportName + "\nfunc " + exportName + "(context C.uintptr_t")
	arguments := make([]string, 0, len(callback.Parameters))
	conversions := make([]string, 0, len(callback.Parameters))
	copyBacks := make([]string, 0, len(callback.Parameters))
	failureReturn := "return"
	if ownedResult {
		failureReturn = "return nil"
	} else if callback.Result != "void" {
		failureReturn = "return " + result.toC(result.zero)
	}
	for index, parameter := range callback.Parameters {
		name := "value" + strconv.Itoa(index)
		switch parameter.Type {
		case "copiedCString":
			source.WriteString(", " + name + " *C.char")
			conversions = append(conversions, "if "+name+" == nil { state.recordInputError("+strconv.Quote(parameter.Name)+", \"null string pointer\"); "+failureReturn+" }\n")
			arguments = append(arguments, "C.GoString("+name+")")
		case "nullableCopiedCString":
			local := "kinmokuseiNullableString" + strconv.Itoa(index)
			source.WriteString(", " + name + " *C.char")
			conversions = append(conversions, "var "+local+" *string\nif "+name+" != nil { kinmokuseiString := C.GoString("+name+"); "+local+" = &kinmokuseiString }\n")
			arguments = append(arguments, local)
		case "copiedBytes", "inoutBytes":
			local := "kinmokuseiCopiedBytes" + strconv.Itoa(index)
			length := name + "Length"
			source.WriteString(", " + name + " *C.uint8_t, " + length + " C.size_t")
			conversions = append(conversions, "var "+local+" []byte\nif "+name+" == nil {\nif "+length+" != 0 { state.recordInputError("+strconv.Quote(parameter.Name)+", \"null byte pointer with non-zero length\"); "+failureReturn+" }\n"+local+" = []byte{}\n} else {\nif uint64("+length+") > uint64(^uint(0)>>1) { state.recordInputError("+strconv.Quote(parameter.Name)+", \"byte length exceeds Go int\"); "+failureReturn+" }\nkinmokuseiSource := unsafe.Slice((*byte)(unsafe.Pointer("+name+")), int("+length+"))\n"+local+" = append([]byte{}, kinmokuseiSource...)\n}\n")
			arguments = append(arguments, local)
			if parameter.Type == "inoutBytes" {
				copyBacks = append(copyBacks, "if "+name+" != nil { copy(unsafe.Slice((*byte)(unsafe.Pointer("+name+")), int("+length+")), "+local+") }\n")
			}
		default:
			typeInfo := cffiTypeInfo(parameter.Type, namedTypes)
			source.WriteString(", " + name + " " + typeInfo.cgoType)
			arguments = append(arguments, typeInfo.fromC(name))
		}
	}
	if ownedBytesResult {
		source.WriteString(", outputLength *C.size_t) (output *C.uint8_t) {\n")
	} else if ownedCStringResult {
		source.WriteString(") (output *C.char) {\n")
	} else if ownedArrayResult {
		source.WriteString(", outputLength *C.size_t) (output *" + arrayElement.cgoType + ") {\n")
	} else if callback.Result != "void" {
		source.WriteString(") (output " + result.cgoType + ") {\n")
	} else {
		source.WriteString(") {\n")
	}
	source.WriteString("state := cgo.Handle(context).Value().(*" + stateType + ")\n")
	if callback.Lifetime == "registered" {
		if ownedBytesResult || ownedArrayResult {
			source.WriteString("if !state.begin() { if outputLength != nil { *outputLength = 0 }; return nil }\n")
		} else if ownedCStringResult {
			source.WriteString("if !state.begin() { return nil }\n")
		} else if callback.Result != "void" {
			source.WriteString("if !state.begin() { return " + result.toC(result.zero) + " }\n")
		} else {
			source.WriteString("if !state.begin() { return }\n")
		}
		source.WriteString("defer state.end()\n")
	} else if ownedBytesResult || ownedArrayResult {
		source.WriteString("if state.hasFailed() { if outputLength != nil { *outputLength = 0 }; return nil }\n")
	} else if ownedCStringResult {
		source.WriteString("if state.hasFailed() { return nil }\n")
	} else if callback.Result != "void" {
		source.WriteString("if state.hasFailed() { return " + result.toC(result.zero) + " }\n")
	} else {
		source.WriteString("if state.hasFailed() { return }\n")
	}
	if ownedBytesResult {
		source.WriteString("if outputLength != nil { *outputLength = 0 }\n")
		source.WriteString("defer func() { if value := recover(); value != nil { state.recordPanic(value); if output != nil { C.free(unsafe.Pointer(output)) }; output = nil; if outputLength != nil { *outputLength = 0 } } }()\n")
		source.WriteString("if outputLength == nil { state.recordInputError(\"$result\", \"null owned byte length pointer\"); return nil }\n")
	} else if ownedCStringResult {
		source.WriteString("defer func() { if value := recover(); value != nil { state.recordPanic(value); output = nil } }()\n")
	} else if ownedArrayResult {
		source.WriteString("if outputLength != nil { *outputLength = 0 }\n")
		source.WriteString("defer func() { if value := recover(); value != nil { state.recordPanic(value); if output != nil { C.free(unsafe.Pointer(output)) }; output = nil; if outputLength != nil { *outputLength = 0 } } }()\n")
		source.WriteString("if outputLength == nil { state.recordInputError(\"$result\", \"null owned array length pointer\"); return nil }\n")
	} else if callback.Result != "void" {
		source.WriteString("defer func() { if value := recover(); value != nil { state.recordPanic(value); output = " + result.toC(result.zero) + " } }()\n")
	} else {
		source.WriteString("defer func() { if value := recover(); value != nil { state.recordPanic(value) } }()\n")
	}
	for _, conversion := range conversions {
		source.WriteString(conversion)
	}
	if ownedBytesResult {
		source.WriteString("kinmokuseiCallbackResult := state.callback(" + strings.Join(arguments, ", ") + ")\n")
		source.WriteString("if len(kinmokuseiCallbackResult) != 0 { output = (*C.uint8_t)(C.CBytes(kinmokuseiCallbackResult)) }\n")
		for _, copyBack := range copyBacks {
			source.WriteString(copyBack)
		}
		source.WriteString("*outputLength = C.size_t(len(kinmokuseiCallbackResult))\nreturn output\n")
	} else if ownedCStringResult {
		source.WriteString("kinmokuseiCallbackResult := state.callback(" + strings.Join(arguments, ", ") + ")\n")
		source.WriteString("if strings.ContainsRune(kinmokuseiCallbackResult, '\\x00') { state.recordInputError(\"$result\", \"embedded NUL in owned string result\"); return nil }\n")
		for _, copyBack := range copyBacks {
			source.WriteString(copyBack)
		}
		source.WriteString("return C.CString(kinmokuseiCallbackResult)\n")
	} else if ownedArrayResult {
		source.WriteString("kinmokuseiCallbackResult := state.callback(" + strings.Join(arguments, ", ") + ")\n")
		source.WriteString("if len(kinmokuseiCallbackResult) != 0 {\n")
		source.WriteString("var kinmokuseiArrayElement " + arrayElement.cgoType + "\n")
		source.WriteString("kinmokuseiElementSize := uint64(unsafe.Sizeof(kinmokuseiArrayElement))\n")
		source.WriteString("if uint64(len(kinmokuseiCallbackResult)) > uint64(^uintptr(0))/kinmokuseiElementSize { state.recordInputError(\"$result\", \"owned array size exceeds address space\"); return nil }\n")
		source.WriteString("output = (*" + arrayElement.cgoType + ")(C.malloc(C.size_t(uint64(len(kinmokuseiCallbackResult)) * kinmokuseiElementSize)))\n")
		source.WriteString("if output == nil { panic(\"C allocation failed\") }\n")
		source.WriteString("kinmokuseiOutput := unsafe.Slice(output, len(kinmokuseiCallbackResult))\n")
		source.WriteString("for index, value := range kinmokuseiCallbackResult { kinmokuseiOutput[index] = " + arrayElement.toC("value") + " }\n")
		source.WriteString("}\n")
		for _, copyBack := range copyBacks {
			source.WriteString(copyBack)
		}
		source.WriteString("*outputLength = C.size_t(len(kinmokuseiCallbackResult))\nreturn output\n")
	} else if callback.Result != "void" {
		if len(copyBacks) == 0 {
			source.WriteString("return " + result.toC("state.callback("+strings.Join(arguments, ", ")+")") + "\n")
		} else {
			source.WriteString("kinmokuseiCallbackResult := state.callback(" + strings.Join(arguments, ", ") + ")\n")
			for _, copyBack := range copyBacks {
				source.WriteString(copyBack)
			}
			source.WriteString("return " + result.toC("kinmokuseiCallbackResult") + "\n")
		}
	} else {
		source.WriteString("state.callback(" + strings.Join(arguments, ", ") + ")\n")
		for _, copyBack := range copyBacks {
			source.WriteString(copyBack)
		}
	}
	source.WriteString("}\n")
}

func generateCFFICallbackRegistration(source *strings.Builder, policy string, registration cffiCallbackRegistration, callbacks map[string]cffiCallback, handles map[string]cffiHandle, namedTypes map[string]cffiScalar) {
	callback := callbacks[registration.Callback]
	stateType := "kinmokuseiCFFI" + callback.Name + "State"
	rawRegister := "kinmokuseiCFFIRawRegister" + registration.Name
	rawClose := "kinmokuseiCFFIRawClose"
	fields := []string{"mutex sync.Mutex", "context cgo.Handle", "state *" + stateType, "closed bool"}
	publicParameters := make([]string, 0, len(registration.Parameters)+1)
	parameterNames := make([]string, 0, len(registration.Parameters)+1)
	registerArguments := make([]string, 0, len(registration.Parameters)+1)
	unregisterArguments := make([]string, 0, len(registration.Parameters)+1)
	initializers := []string{"context: context", "state: state"}
	allocations := []string{}
	localCleanups := []string{}
	closeCleanups := []string{}
	stringChecks := []string{}
	coupledHandleName := ""
	coupledHandleField := ""
	for index, parameter := range registration.Parameters {
		field := "parameter" + strconv.Itoa(index)
		parameterNames = append(parameterNames, parameter.Name)
		if handle, exists := handles[parameter.Type]; exists {
			fields = append(fields, field+" *"+handle.Name)
			publicParameters = append(publicParameters, parameter.Name+" *"+handle.Name)
			registerArguments = append(registerArguments, parameter.Name+".pointer")
			unregisterArguments = append(unregisterArguments, "registration."+field+".pointer")
			coupledHandleName = parameter.Name
			coupledHandleField = field
			initializers = append(initializers, field+": "+parameter.Name)
		} else if parameter.Type == "retainedCString" {
			local := "kinmokuseiCString" + strconv.Itoa(index)
			fields = append(fields, field+" *C.char")
			publicParameters = append(publicParameters, parameter.Name+" string")
			registerArguments = append(registerArguments, local)
			unregisterArguments = append(unregisterArguments, "registration."+field)
			stringChecks = append(stringChecks, "if strings.IndexByte("+parameter.Name+", 0) >= 0 { return nil, ErrEmbeddedNUL }\n")
			allocations = append(allocations, local+" := C.CString("+parameter.Name+")\n")
			localCleanups = append(localCleanups, "C.kinmokusei_cffi_free_string("+local+")\n")
			closeCleanups = append(closeCleanups, "C.kinmokusei_cffi_free_string(registration."+field+")\n")
			initializers = append(initializers, field+": "+local)
		} else if parameter.Type == "retainedBytes" {
			local := "kinmokuseiBytes" + strconv.Itoa(index)
			length := local + "Length"
			fields = append(fields, field+" unsafe.Pointer", field+"Length C.size_t")
			publicParameters = append(publicParameters, parameter.Name+" []byte")
			registerArguments = append(registerArguments, "(*C.uint8_t)("+local+")", length)
			unregisterArguments = append(unregisterArguments, "(*C.uint8_t)(registration."+field+")", "registration."+field+"Length")
			allocations = append(allocations, "var "+local+" unsafe.Pointer\nif len("+parameter.Name+") != 0 { "+local+" = C.CBytes("+parameter.Name+") }\n"+length+" := C.size_t(len("+parameter.Name+"))\n")
			localCleanups = append(localCleanups, "if "+local+" != nil { C.kinmokusei_cffi_free_bytes("+local+") }\n")
			closeCleanups = append(closeCleanups, "if registration."+field+" != nil { C.kinmokusei_cffi_free_bytes(registration."+field+") }\n")
			initializers = append(initializers, field+": "+local, field+"Length: "+length)
		} else {
			typeInfo := cffiTypeInfo(parameter.Type, namedTypes)
			fields = append(fields, field+" "+typeInfo.goType)
			publicParameters = append(publicParameters, parameter.Name+" "+typeInfo.goType)
			registerArguments = append(registerArguments, typeInfo.toC(parameter.Name))
			unregisterArguments = append(unregisterArguments, typeInfo.toC("registration."+field))
			initializers = append(initializers, field+": "+parameter.Name)
		}
	}
	publicParameters = append(publicParameters, "callback "+callback.Name)
	parameterNames = append(parameterNames, "callback")
	registerArguments = append(registerArguments, "C.uintptr_t(context)")
	unregisterArguments = append(unregisterArguments, "C.uintptr_t(registration.context)")
	source.WriteString("\ntype " + registration.Name + " struct { " + strings.Join(fields, "; ") + " }\n")
	source.WriteString("func Register" + registration.Name + "(" + strings.Join(publicParameters, ", ") + ") (*" + registration.Name + ", error) {\n")
	if policy == "threadAffine" {
		source.WriteString("var result *" + registration.Name + "\nvar resultError error\nkinmokuseiCFFIDo(func() { result, resultError = " + rawRegister + "(" + strings.Join(parameterNames, ", ") + ") })\nreturn result, resultError\n}\n")
	} else {
		source.WriteString("return " + rawRegister + "(" + strings.Join(parameterNames, ", ") + ")\n}\n")
	}
	source.WriteString("func " + rawRegister + "(" + strings.Join(publicParameters, ", ") + ") (*" + registration.Name + ", error) {\n")
	source.WriteString("if callback == nil { return nil, ErrNilCallback }\n")
	for _, check := range stringChecks {
		source.WriteString(check)
	}
	if coupledHandleName != "" {
		source.WriteString("if " + coupledHandleName + " == nil { return nil, ErrClosedHandle }\n")
	}
	for _, allocation := range allocations {
		source.WriteString(allocation)
	}
	source.WriteString("state := &" + stateType + "{callback: callback}\ncontext := cgo.NewHandle(state)\n")
	if policy == "serialized" {
		source.WriteString("kinmokuseiCFFIMutex.Lock()\n")
	}
	if coupledHandleName != "" {
		source.WriteString(coupledHandleName + ".mutex.Lock()\n")
		source.WriteString("if " + coupledHandleName + ".closed || " + coupledHandleName + ".pointer == nil {\n")
		source.WriteString(coupledHandleName + ".mutex.Unlock()\n")
		if policy == "serialized" {
			source.WriteString("kinmokuseiCFFIMutex.Unlock()\n")
		}
		for _, cleanup := range localCleanups {
			source.WriteString(cleanup)
		}
		source.WriteString("context.Delete()\nreturn nil, ErrClosedHandle\n}\n")
	}
	source.WriteString("status := int32(C.kinmokusei_cffi_register_" + registration.Name + "(" + strings.Join(registerArguments, ", ") + "))\n")
	if coupledHandleName != "" {
		source.WriteString("if status == 0 { " + coupledHandleName + ".registrations++ }\n")
		source.WriteString(coupledHandleName + ".mutex.Unlock()\n")
	}
	if policy == "serialized" {
		source.WriteString("kinmokuseiCFFIMutex.Unlock()\n")
	}
	source.WriteString("if status != 0 {\n")
	for _, cleanup := range localCleanups {
		source.WriteString(cleanup)
	}
	source.WriteString("context.Delete()\nreturn nil, &StatusError{Function: " + strconv.Quote("Register"+registration.Name) + ", Code: status}\n}\n")
	source.WriteString("return &" + registration.Name + "{" + strings.Join(initializers, ", ") + "}, nil\n}\n")
	source.WriteString("func (registration *" + registration.Name + ") CallbackError() error { if registration == nil || registration.state == nil { return ErrClosedCallbackRegistration }; return registration.state.callbackError(" + strconv.Quote(registration.Name) + ") }\n")
	source.WriteString("func (registration *" + registration.Name + ") Close() error {\n")
	if policy == "threadAffine" {
		source.WriteString("var result error\nkinmokuseiCFFIDo(func() { result = registration." + rawClose + "() })\nreturn result\n}\n")
	} else {
		source.WriteString("return registration." + rawClose + "()\n}\n")
	}
	source.WriteString("func (registration *" + registration.Name + ") " + rawClose + "() error {\n")
	source.WriteString("if registration == nil { return ErrClosedCallbackRegistration }\nregistration.mutex.Lock()\ndefer registration.mutex.Unlock()\n")
	source.WriteString("if registration.closed || registration.state == nil || registration.context == 0 { return ErrClosedCallbackRegistration }\nregistration.state.stop()\n")
	if policy == "serialized" {
		source.WriteString("kinmokuseiCFFIMutex.Lock()\n")
	}
	if coupledHandleField != "" {
		source.WriteString("registration." + coupledHandleField + ".mutex.Lock()\n")
	}
	source.WriteString("status := int32(C.kinmokusei_cffi_unregister_" + registration.Name + "(" + strings.Join(unregisterArguments, ", ") + "))\n")
	if coupledHandleField != "" {
		source.WriteString("registration." + coupledHandleField + ".mutex.Unlock()\n")
	}
	if policy == "serialized" {
		source.WriteString("kinmokuseiCFFIMutex.Unlock()\n")
	}
	source.WriteString("if status != 0 { registration.state.resume(); return &StatusError{Function: " + strconv.Quote(registration.Name+".Close") + ", Code: status} }\n")
	source.WriteString("registration.state.wait()\n")
	if coupledHandleField != "" {
		source.WriteString("registration." + coupledHandleField + ".mutex.Lock()\n")
		source.WriteString("registration." + coupledHandleField + ".registrations--\n")
		source.WriteString("registration." + coupledHandleField + ".mutex.Unlock()\n")
	}
	for _, cleanup := range closeCleanups {
		source.WriteString(cleanup)
	}
	source.WriteString("registration.context.Delete()\nregistration.context = 0\nregistration.closed = true\nreturn nil\n}\n")
}

func generateCFFIStruct(source *strings.Builder, structure cffiStruct, namedTypes map[string]cffiScalar) {
	source.WriteString("\ntype " + structure.Name + " struct {\n")
	for _, field := range structure.Fields {
		typeInfo := cffiTypeInfo(field.Type, namedTypes)
		source.WriteString(field.Name + " " + typeInfo.goType + "\n")
	}
	source.WriteString("}\n")
	source.WriteString("func kinmokuseiCFFITo" + structure.Name + "(value " + structure.Name + ") C." + structure.CType + " {\n")
	source.WriteString("var output C." + structure.CType + "\n")
	for _, field := range structure.Fields {
		typeInfo := cffiTypeInfo(field.Type, namedTypes)
		source.WriteString("output." + field.CName + " = " + typeInfo.toC("value."+field.Name) + "\n")
	}
	source.WriteString("return output\n}\n")
	source.WriteString("func kinmokuseiCFFIFrom" + structure.Name + "(value C." + structure.CType + ") " + structure.Name + " {\n")
	source.WriteString("var output " + structure.Name + "\n")
	for _, field := range structure.Fields {
		typeInfo := cffiTypeInfo(field.Type, namedTypes)
		source.WriteString("output." + field.Name + " = " + typeInfo.fromC("value."+field.CName) + "\n")
	}
	source.WriteString("return output\n}\n")
}

func cffiTypeInfo(name string, namedTypes map[string]cffiScalar) cffiScalar {
	if scalar, exists := cffiScalars[name]; exists {
		return scalar
	}
	return namedTypes[name]
}

func cffiGoResultTypes(function cffiFunction, handles map[string]cffiHandle, namedTypes map[string]cffiScalar, callbacks map[string]cffiCallback) []string {
	result := cffiTypeInfo(function.Result, namedTypes)
	if function.Result == "ownedCString" {
		result = cffiScalar{goType: "string", zero: `""`, canResult: true}
	} else if function.Result == "ownedBytes" {
		result = cffiScalar{goType: "[]byte", zero: "nil", canResult: true}
	} else if function.Result == "ownedArray" {
		element := cffiTypeInfo(function.ResultElement, namedTypes)
		result = cffiScalar{goType: "[]" + element.goType, zero: "nil", canResult: true}
	}
	if handle, exists := handles[function.Result]; exists {
		result.goType = "*" + handle.Name
	}
	if function.Convention == "statusOut" {
		return []string{result.goType, "error"}
	}
	if function.Convention == "status" {
		return []string{"error"}
	}
	hasCString := cffiFunctionHasType(function, "cstring")
	hasCallback := cffiFunctionCallback(function, callbacks) != nil
	if function.Result == "void" {
		if hasCString || hasCallback {
			return []string{"error"}
		}
		return nil
	}
	if hasCString || hasCallback || function.Result == "cstring" {
		return []string{result.goType, "error"}
	}
	return []string{result.goType}
}

func writeCFFIGoFunctionSignature(source *strings.Builder, name string, function cffiFunction, handles map[string]cffiHandle, namedTypes map[string]cffiScalar, callbacks map[string]cffiCallback) {
	source.WriteString("func " + name + "(")
	for index, parameter := range function.Parameters {
		if index != 0 {
			source.WriteString(", ")
		}
		goType := "string"
		if _, exists := callbacks[parameter.Type]; exists {
			goType = parameter.Type
		} else if handle, exists := handles[parameter.Type]; exists {
			goType = "*" + handle.Name
		} else if parameter.Type == "borrowedBytes" {
			goType = "[]byte"
		} else if parameter.Type != "cstring" {
			goType = cffiTypeInfo(parameter.Type, namedTypes).goType
		}
		source.WriteString(parameter.Name + " " + goType)
	}
	source.WriteByte(')')
	results := cffiGoResultTypes(function, handles, namedTypes, callbacks)
	if len(results) == 1 {
		source.WriteByte(' ')
		source.WriteString(results[0])
	} else if len(results) > 1 {
		source.WriteString(" (" + strings.Join(results, ", ") + ")")
	}
}

func generateThreadAffineCFFIFunction(source *strings.Builder, function cffiFunction, handles map[string]cffiHandle, namedTypes map[string]cffiScalar, callbacks map[string]cffiCallback) {
	source.WriteByte('\n')
	writeCFFIGoFunctionSignature(source, function.Name, function, handles, namedTypes, callbacks)
	source.WriteString(" {\n")
	results := cffiGoResultTypes(function, handles, namedTypes, callbacks)
	for index, result := range results {
		name := "kinmokuseiResult"
		if index == 1 {
			name = "kinmokuseiError"
		}
		source.WriteString("var " + name + " " + result + "\n")
	}
	arguments := make([]string, 0, len(function.Parameters)+1)
	for _, parameter := range function.Parameters {
		arguments = append(arguments, parameter.Name)
	}
	call := "kinmokuseiCFFIRaw" + function.Name + "(" + strings.Join(arguments, ", ") + ")"
	source.WriteString("kinmokuseiCFFIDo(func() { ")
	if len(results) == 1 {
		source.WriteString("kinmokuseiResult = ")
	} else if len(results) == 2 {
		source.WriteString("kinmokuseiResult, kinmokuseiError = ")
	}
	source.WriteString(call + " })\n")
	if len(results) == 1 {
		source.WriteString("return kinmokuseiResult\n")
	} else if len(results) == 2 {
		source.WriteString("return kinmokuseiResult, kinmokuseiError\n")
	}
	source.WriteString("}\n")
	var raw strings.Builder
	generateCFFIFunction(&raw, "threadSafe", function, handles, namedTypes, callbacks)
	rawSource := strings.Replace(raw.String(), "\nfunc "+function.Name+"(", "\nfunc kinmokuseiCFFIRaw"+function.Name+"(", 1)
	source.WriteString(rawSource)
}

func generateCFFIHandle(source *strings.Builder, policy string, handle cffiHandle) {
	source.WriteString("\ntype " + handle.Name + " struct { mutex sync.Mutex; pointer *C." + handle.CType + "; closed bool; registrations int }\n")
	if policy == "threadAffine" {
		source.WriteString("\nfunc (handle *" + handle.Name + ") Close() error {\n")
		source.WriteString("var kinmokuseiResult error\nkinmokuseiCFFIDo(func() { kinmokuseiResult = handle.kinmokuseiCFFIRawClose() })\nreturn kinmokuseiResult\n}\n")
		source.WriteString("func (handle *" + handle.Name + ") kinmokuseiCFFIRawClose() error {\n")
	} else {
		source.WriteString("func (handle *" + handle.Name + ") Close() error {\n")
	}
	if policy == "serialized" {
		source.WriteString("kinmokuseiCFFIMutex.Lock()\ndefer kinmokuseiCFFIMutex.Unlock()\n")
	}
	source.WriteString("if handle == nil { return ErrClosedHandle }\nhandle.mutex.Lock()\ndefer handle.mutex.Unlock()\n")
	source.WriteString("if handle.closed || handle.pointer == nil { return ErrClosedHandle }\n")
	source.WriteString("if handle.registrations != 0 { return ErrHandleHasActiveRegistrations }\n")
	source.WriteString("C." + handle.Release + "(handle.pointer)\nhandle.pointer = nil\nhandle.closed = true\nreturn nil\n}\n")
}

func generateCFFIFunction(source *strings.Builder, policy string, function cffiFunction, handles map[string]cffiHandle, namedTypes map[string]cffiScalar, callbacks map[string]cffiCallback) {
	if policy == "threadAffine" {
		generateThreadAffineCFFIFunction(source, function, handles, namedTypes, callbacks)
		return
	}
	result, scalarResult := cffiScalars[function.Result]
	ownedCStringResult := function.Result == "ownedCString"
	ownedBytesResult := function.Result == "ownedBytes"
	ownedArrayResult := function.Result == "ownedArray"
	if ownedCStringResult {
		result = cffiScalar{goType: "string", cgoType: "*C.char", zero: `""`, canResult: true}
		scalarResult = true
	} else if ownedBytesResult {
		result = cffiScalar{goType: "[]byte", cgoType: "*C.uint8_t", zero: "nil", canResult: true}
		scalarResult = true
	} else if ownedArrayResult {
		element := cffiTypeInfo(function.ResultElement, namedTypes)
		result = cffiScalar{goType: "[]" + element.goType, cgoType: "*" + element.cgoType, zero: "nil", canResult: true}
		scalarResult = true
	}
	if namedResult, exists := namedTypes[function.Result]; exists {
		result = namedResult
		scalarResult = true
	}
	resultHandle, handleResult := handles[function.Result]
	hasCString := cffiFunctionHasType(function, "cstring")
	callback := cffiFunctionCallback(function, callbacks)
	hasCallback := callback != nil
	returnsCString := function.Result == "cstring"
	source.WriteString("\nfunc ")
	source.WriteString(function.Name)
	source.WriteByte('(')
	arguments := make([]string, 0, len(function.Parameters)+1)
	for index, parameter := range function.Parameters {
		if index != 0 {
			source.WriteString(", ")
		}
		if _, exists := callbacks[parameter.Type]; exists {
			source.WriteString(parameter.Name + " " + parameter.Type)
			arguments = append(arguments, "C.uintptr_t(kinmokuseiCallbackHandle"+strconv.Itoa(index)+")")
		} else if handle, exists := handles[parameter.Type]; exists {
			source.WriteString(parameter.Name + " *" + handle.Name)
			arguments = append(arguments, parameter.Name+".pointer")
		} else if parameter.Type == "cstring" {
			source.WriteString(parameter.Name + " string")
			arguments = append(arguments, "kinmokuseiCString"+strconv.Itoa(index))
		} else if parameter.Type == "borrowedBytes" {
			source.WriteString(parameter.Name + " []byte")
			arguments = append(arguments, "kinmokuseiBytes"+strconv.Itoa(index), "C.size_t(len("+parameter.Name+"))")
		} else {
			typeInfo := cffiTypeInfo(parameter.Type, namedTypes)
			source.WriteString(parameter.Name + " " + typeInfo.goType)
			arguments = append(arguments, typeInfo.toC(parameter.Name))
		}
	}
	source.WriteByte(')')
	if function.Convention == "statusOut" {
		if handleResult {
			source.WriteString(" (*" + resultHandle.Name + ", error)")
		} else {
			source.WriteString(" (" + result.goType + ", error)")
		}
	} else if function.Convention == "status" {
		source.WriteString(" error")
	} else if (hasCString || hasCallback) && function.Result == "void" {
		source.WriteString(" error")
	} else if hasCString || hasCallback || returnsCString {
		source.WriteString(" (" + result.goType + ", error)")
	} else if function.Result != "void" {
		source.WriteByte(' ')
		source.WriteString(result.goType)
	}
	source.WriteString(" {\n")
	for index, parameter := range function.Parameters {
		if parameter.Type != "cstring" {
			continue
		}
		failure := cffiFailureReturn(function, handleResult, result, "ErrEmbeddedNUL")
		source.WriteString("if strings.IndexByte(" + parameter.Name + ", 0) >= 0 { " + failure + " }\n")
		name := "kinmokuseiCString" + strconv.Itoa(index)
		source.WriteString(name + " := C.CString(" + parameter.Name + ")\n")
		source.WriteString("defer C.kinmokusei_cffi_free_string(" + name + ")\n")
	}
	var callbackState string
	for index, parameter := range function.Parameters {
		callbackType, exists := callbacks[parameter.Type]
		if !exists {
			continue
		}
		failure := cffiFailureReturn(function, handleResult, result, "ErrNilCallback")
		source.WriteString("if " + parameter.Name + " == nil { " + failure + " }\n")
		callbackState = "kinmokuseiCallbackState" + strconv.Itoa(index)
		source.WriteString(callbackState + " := &kinmokuseiCFFI" + callbackType.Name + "State{callback: " + parameter.Name + "}\n")
		source.WriteString("kinmokuseiCallbackHandle" + strconv.Itoa(index) + " := cgo.NewHandle(" + callbackState + ")\n")
		source.WriteString("defer kinmokuseiCallbackHandle" + strconv.Itoa(index) + ".Delete()\n")
	}
	for index, parameter := range function.Parameters {
		if parameter.Type != "borrowedBytes" {
			continue
		}
		name := "kinmokuseiBytes" + strconv.Itoa(index)
		source.WriteString("var " + name + " *C.uint8_t\n")
		source.WriteString("if len(" + parameter.Name + ") != 0 {\n")
		source.WriteString("kinmokuseiBuffer := C.CBytes(" + parameter.Name + ")\n")
		source.WriteString(name + " = (*C.uint8_t)(kinmokuseiBuffer)\n")
		source.WriteString("defer C.kinmokusei_cffi_free_bytes(kinmokuseiBuffer)\n")
		source.WriteString("}\n")
	}
	if policy == "serialized" {
		source.WriteString("kinmokuseiCFFIMutex.Lock()\ndefer kinmokuseiCFFIMutex.Unlock()\n")
	}
	for _, parameter := range function.Parameters {
		if _, exists := handles[parameter.Type]; exists {
			if function.Convention == "status" {
				source.WriteString("if " + parameter.Name + " == nil { return ErrClosedHandle }\n")
			} else {
				source.WriteString("if " + parameter.Name + " == nil { return " + cffiResultZero(function.Result, handleResult, result) + ", ErrClosedHandle }\n")
			}
			source.WriteString(parameter.Name + ".mutex.Lock()\ndefer " + parameter.Name + ".mutex.Unlock()\n")
			if function.Convention == "status" {
				source.WriteString("if " + parameter.Name + ".closed || " + parameter.Name + ".pointer == nil { return ErrClosedHandle }\n")
			} else {
				source.WriteString("if " + parameter.Name + ".closed || " + parameter.Name + ".pointer == nil { return " + cffiResultZero(function.Result, handleResult, result) + ", ErrClosedHandle }\n")
			}
		}
	}
	callArguments := strings.Join(arguments, ", ")
	callSymbol := function.Symbol
	if hasCallback {
		callSymbol = "kinmokusei_cffi_call_" + function.Name
	}
	callbackFailure := ""
	if hasCallback {
		callbackFailure = "if callbackError := " + callbackState + ".callbackError(" + strconv.Quote(function.Name) + "); callbackError != nil { " + cffiFailureReturn(function, handleResult, result, "callbackError") + " }\n"
	}
	if function.Convention == "status" {
		source.WriteString("status := int32(C." + callSymbol + "(" + callArguments + "))\n")
		source.WriteString(callbackFailure)
		source.WriteString("if status != 0 { return &StatusError{Function: " + strconv.Quote(function.Name) + ", Code: status} }\n")
		source.WriteString("return nil\n")
	} else if function.Convention == "statusOut" {
		if ownedCStringResult {
			source.WriteString("var output *C.char\n")
		} else if ownedBytesResult {
			source.WriteString("var output *C.uint8_t\n")
			source.WriteString("var outputLength C.size_t\n")
		} else if ownedArrayResult {
			element := cffiTypeInfo(function.ResultElement, namedTypes)
			source.WriteString("var output *" + element.cgoType + "\n")
			source.WriteString("var outputLength C.size_t\n")
		} else if handleResult {
			source.WriteString("var output *C." + resultHandle.CType + "\n")
		} else {
			source.WriteString("var output " + result.cgoType + "\n")
		}
		if callArguments != "" {
			callArguments += ", "
		}
		callArguments += "&output"
		if ownedBytesResult || ownedArrayResult {
			callArguments += ", &outputLength"
		}
		source.WriteString("status := int32(C." + callSymbol + "(" + callArguments + "))\n")
		if ownedCStringResult || ownedBytesResult || ownedArrayResult {
			source.WriteString("if output != nil { defer C." + function.ResultRelease + "(output) }\n")
		}
		source.WriteString(callbackFailure)
		zero := cffiResultZero(function.Result, handleResult, result)
		source.WriteString("if status != 0 { return " + zero + ", &StatusError{Function: " + strconv.Quote(function.Name) + ", Code: status} }\n")
		if ownedCStringResult {
			source.WriteString("if output == nil { return \"\", ErrNullOwnedCString }\n")
			source.WriteString("return C.GoString(output), nil\n")
		} else if ownedBytesResult {
			source.WriteString("if outputLength == 0 { return []byte{}, nil }\n")
			source.WriteString("if output == nil { return nil, ErrNullOwnedBuffer }\n")
			source.WriteString("if uint64(outputLength) > uint64(^uint(0)>>1) { return nil, ErrOwnedBufferTooLarge }\n")
			source.WriteString("sourceBytes := unsafe.Slice((*byte)(unsafe.Pointer(output)), int(outputLength))\n")
			source.WriteString("return append([]byte(nil), sourceBytes...), nil\n")
		} else if ownedArrayResult {
			element := cffiTypeInfo(function.ResultElement, namedTypes)
			source.WriteString("if outputLength == 0 { return []" + element.goType + "{}, nil }\n")
			source.WriteString("if output == nil { return nil, ErrNullOwnedArray }\n")
			source.WriteString("elementSize := uint64(unsafe.Sizeof(*output))\n")
			source.WriteString("if uint64(outputLength) > uint64(^uint(0)>>1)/elementSize { return nil, ErrOwnedArrayTooLarge }\n")
			source.WriteString("sourceValues := unsafe.Slice(output, int(outputLength))\n")
			source.WriteString("result := make([]" + element.goType + ", len(sourceValues))\n")
			source.WriteString("for index, value := range sourceValues { result[index] = " + element.fromC("value") + " }\n")
			source.WriteString("return result, nil\n")
		} else if handleResult {
			source.WriteString("if output == nil { return nil, &StatusError{Function: " + strconv.Quote(function.Name) + ", Code: -1} }\n")
			source.WriteString("return &" + resultHandle.Name + "{pointer: output}, nil\n")
		} else {
			if returnsCString {
				source.WriteString("if output == nil { return \"\", ErrNullCString }\n")
			}
			source.WriteString("return " + result.fromC("output") + ", nil\n")
		}
	} else if function.Result == "void" {
		source.WriteString("C." + callSymbol + "(" + callArguments + ")\n")
		source.WriteString(callbackFailure)
		if hasCString || hasCallback {
			source.WriteString("return nil\n")
		}
	} else if scalarResult {
		call := "C." + callSymbol + "(" + callArguments + ")"
		if returnsCString {
			source.WriteString("output := " + call + "\n")
			source.WriteString("if output == nil { return \"\", ErrNullCString }\n")
			source.WriteString("return C.GoString(output), nil\n")
		} else if hasCallback {
			value := result.fromC(call)
			source.WriteString("output := " + value + "\n")
			source.WriteString(callbackFailure)
			source.WriteString("return output, nil\n")
		} else if hasCString {
			value := result.fromC(call)
			source.WriteString("return " + value + ", nil\n")
		} else {
			value := result.fromC(call)
			source.WriteString("return " + value + "\n")
		}
	}
	source.WriteString("}\n")
}

func cffiFailureReturn(function cffiFunction, handleResult bool, scalar cffiScalar, failure string) string {
	if function.Convention == "statusOut" || function.Result != "void" {
		return "return " + cffiResultZero(function.Result, handleResult, scalar) + ", " + failure
	}
	return "return " + failure
}

func cffiResultZero(_ string, handle bool, scalar cffiScalar) string {
	if handle {
		return "nil"
	}
	return scalar.zero
}
