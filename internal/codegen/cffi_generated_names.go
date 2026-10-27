package codegen

import "fmt"

type cffiGeneratedNames struct {
	goNames, cNames map[string]string
	err             error
}

func (names *cffiGeneratedNames) claim(language, owner string, identifiers ...string) {
	if names.err != nil {
		return
	}
	namespace := names.goNames
	if language == "C" {
		namespace = names.cNames
	}
	for _, identifier := range identifiers {
		if previous, exists := namespace[identifier]; exists {
			names.err = fmt.Errorf("generated %s identifier %q conflicts between %s and %s", language, identifier, previous, owner)
			return
		}
		namespace[identifier] = owner
	}
}

// External symbols may be referenced by several aliases/registrations. They
// must not be redefined by a generated helper in C's ordinary identifier scope.
func (names *cffiGeneratedNames) reference(owner string, identifiers ...string) {
	for _, identifier := range identifiers {
		if identifier != "" && names.cNames[identifier] == "" {
			names.cNames[identifier] = owner
		}
	}
}

// Derive names before emitting either language. Keep this inventory aligned
// with the helper families in cffi.go and cffi_registration.go, including only
// conditional helpers that are actually generated. Public API names are not
// renamed, and diagnostics retain the manifest entries responsible.
func validateCFFIGeneratedNames(manifest cffiManifest) error {
	names := &cffiGeneratedNames{goNames: map[string]string{}, cNames: map[string]string{}}
	for identifier := range cffiGoBindingNames(manifest) {
		names.goNames[identifier] = fmt.Sprintf("Go declaration %q", identifier)
	}
	for _, enum := range manifest.Enums {
		names.reference(fmt.Sprintf("enum %q C type", enum.Name), enum.CType)
		for _, value := range enum.Values {
			names.reference(fmt.Sprintf("enum %q value %q", enum.Name, value.Name), value.Symbol)
		}
	}
	for _, structure := range manifest.Structs {
		names.reference(fmt.Sprintf("struct %q C type", structure.Name), structure.CType)
	}
	for _, union := range manifest.TaggedUnions {
		names.reference(fmt.Sprintf("tagged union %q C type", union.Name), union.CType)
		for _, variant := range union.Variants {
			names.reference(fmt.Sprintf("tagged union %q variant %q", union.Name, variant.Name), variant.Tags...)
		}
	}
	for _, handle := range manifest.Handles {
		names.reference(fmt.Sprintf("handle %q native API", handle.Name), handle.CType, handle.Release)
	}
	for _, function := range manifest.Functions {
		names.reference(fmt.Sprintf("function %q C symbol", function.Name), function.Symbol, function.ResultRelease)
	}
	for _, registration := range manifest.CallbackRegistrations {
		names.reference(fmt.Sprintf("registration %q native API", registration.Name), registration.Register, registration.Unregister)
	}
	for _, structure := range manifest.Structs {
		names.claim("Go", fmt.Sprintf("struct %q conversion helpers", structure.Name), "kinmokuseiCFFIFrom"+structure.Name, "kinmokuseiCFFITo"+structure.Name)
	}
	for _, union := range manifest.TaggedUnions {
		owner := fmt.Sprintf("tagged union %q conversion helpers", union.Name)
		names.claim("Go", owner, "kinmokuseiCFFIFrom"+union.Name, "kinmokuseiCFFITo"+union.Name)
		prefix := "kinmokusei_cffi_" + union.Name
		names.claim("C", owner, prefix+"_get_tag", prefix+"_set_tag")
		for _, variant := range union.Variants {
			names.claim("C", fmt.Sprintf("tagged union %q variant %q", union.Name, variant.Name), prefix+"_get_"+variant.Name, prefix+"_set_"+variant.Name)
		}
	}
	for _, callback := range manifest.Callbacks {
		owner := fmt.Sprintf("callback %q helpers", callback.Name)
		prefix := "kinmokusei_cffi_callback_" + callback.Name
		names.claim("Go", owner, "kinmokuseiCFFI"+callback.Name+"State", prefix+"_go")
		names.claim("C", owner, prefix+"_fn", prefix+"_go", prefix+"_bridge")
		if callback.Result == "ownedBytes" || callback.Result == "ownedCString" || callback.Result == "ownedArray" {
			names.claim("C", owner, prefix+"_release_fn", prefix+"_release")
		}
	}
	for _, registration := range manifest.CallbackRegistrations {
		owner := fmt.Sprintf("registration %q helpers", registration.Name)
		names.claim("Go", owner, "kinmokuseiCFFIRawRegister"+registration.Name, "kinmokusei_cffi_registration_state_"+registration.Name)
		names.claim("C", owner, "kinmokusei_cffi_register_"+registration.Name, "kinmokusei_cffi_unregister_"+registration.Name)
	}
	for _, handle := range manifest.Handles {
		names.claim("Go", fmt.Sprintf("handle %q ownership state", handle.Name), "kinmokusei_cffi_handle_state_"+handle.Name)
	}
	callbacks := cffiCallbacksByName(manifest)
	for _, function := range manifest.Functions {
		owner := fmt.Sprintf("function %q helpers", function.Name)
		if manifest.ThreadPolicy == "threadAffine" {
			names.claim("Go", owner, "kinmokuseiCFFIRaw"+function.Name)
		}
		if cffiFunctionHasCallback(function, callbacks) {
			names.claim("C", owner, "kinmokusei_cffi_call_"+function.Name)
		}
	}
	if manifest.ThreadPolicy == "mainThread" {
		names.claim("C", "main-thread runtime", "kinmokusei_cffi_main_thread", "kinmokusei_cffi_mark_main_thread", "kinmokusei_cffi_is_main_thread")
	}
	if cffiManifestUsesType(manifest, "cInt32") {
		names.claim("C", "C int width check", "kinmokusei_c_int_must_be_32_bits")
	}
	if cffiManifestUsesType(manifest, "cUint32") {
		names.claim("C", "C unsigned int width check", "kinmokusei_c_uint_must_be_32_bits")
	}
	if cffiManifestHasParameterType(manifest, "cstring") || cffiManifestHasRegistrationParameterType(manifest, "retainedCString") {
		names.claim("C", "string input cleanup", "kinmokusei_cffi_free_string")
	}
	arrays := cffiManifestHasParameterType(manifest, "borrowedArray") || cffiManifestHasRegistrationParameterType(manifest, "retainedArray")
	if arrays || cffiManifestHasParameterType(manifest, "borrowedBytes") || cffiManifestHasRegistrationParameterType(manifest, "retainedBytes") {
		names.claim("C", "buffer input cleanup", "kinmokusei_cffi_free_bytes")
	}
	if arrays {
		names.claim("C", "array input allocation", "kinmokusei_cffi_alloc_array")
	}
	return names.err
}
