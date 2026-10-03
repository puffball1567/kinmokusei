package codegen

import (
	"strconv"
	"strings"
)

func generateCFFICallbackRegistration(source *strings.Builder, policy string, registration cffiCallbackRegistration, callbacks map[string]cffiCallback, handles map[string]cffiHandle, namedTypes map[string]cffiScalar, multipleHandles bool) {
	callback := callbacks[registration.Callback]
	stateType := "kinmokuseiCFFI" + callback.Name + "State"
	rawRegister := "kinmokuseiCFFIRawRegister" + registration.Name
	rawClose := "kinmokuseiCFFIRawClose"
	ownershipType := "kinmokusei_cffi_registration_state_" + registration.Name
	fields := []string{"mutex sync.Mutex", "context cgo.Handle", "state *" + stateType, "closed bool"}
	publicParameters := make([]string, 0, len(registration.Parameters)+1)
	parameterNames := make([]string, 0, len(registration.Parameters)+1)
	registerArguments := make([]string, 0, len(registration.Parameters)+1)
	unregisterArguments := make([]string, 0, len(registration.Parameters)+1)
	initializers := []string{"context: context", "state: state"}
	allocationDeclarations := []string{}
	allocations := []string{}
	localCleanups := []string{}
	closeCleanups := []string{}
	stringChecks := []string{}
	coupledHandleNames := []string{}
	coupledHandleSnapshots := []string{}
	coupledHandleFields := []string{}
	handleSnapshots := []string{}
	for index, parameter := range registration.Parameters {
		field := "parameter" + strconv.Itoa(index)
		parameterNames = append(parameterNames, parameter.Name)
		if handle, exists := handles[parameter.Type]; exists {
			handleState := "kinmokusei_cffi_handle_state_" + handle.Name
			snapshot := "kinmokuseiRetainedHandle" + strconv.Itoa(index)
			fields = append(fields, field+" *"+handleState)
			publicParameters = append(publicParameters, parameter.Name+" *"+handle.Name)
			registerArguments = append(registerArguments, snapshot+".pointer")
			unregisterArguments = append(unregisterArguments, "kinmokuseiRegistrationState."+field+".pointer")
			coupledHandleNames = append(coupledHandleNames, parameter.Name)
			coupledHandleSnapshots = append(coupledHandleSnapshots, snapshot)
			coupledHandleFields = append(coupledHandleFields, "kinmokuseiRegistrationState."+field)
			handleSnapshots = append(handleSnapshots, snapshot+" := "+parameter.Name+"."+handleState+"\n")
			initializers = append(initializers, field+": "+snapshot)
		} else if parameter.Type == "retainedCString" {
			local := "kinmokuseiCString" + strconv.Itoa(index)
			fields = append(fields, field+" *C.char")
			publicParameters = append(publicParameters, parameter.Name+" string")
			registerArguments = append(registerArguments, local)
			unregisterArguments = append(unregisterArguments, "kinmokuseiRegistrationState."+field)
			stringChecks = append(stringChecks, "if strings.IndexByte("+parameter.Name+", 0) >= 0 { return nil, ErrEmbeddedNUL }\n")
			allocationDeclarations = append(allocationDeclarations, "var "+local+" *C.char\n")
			allocations = append(allocations, local+" = C.CString("+parameter.Name+")\n")
			localCleanups = append(localCleanups, "C.kinmokusei_cffi_free_string("+local+")\n")
			closeCleanups = append(closeCleanups, "C.kinmokusei_cffi_free_string(kinmokuseiRegistrationState."+field+")\n")
			initializers = append(initializers, field+": "+local)
		} else if parameter.Type == "retainedBytes" {
			local := "kinmokuseiBytes" + strconv.Itoa(index)
			length := local + "Length"
			fields = append(fields, field+" unsafe.Pointer", field+"Length C.size_t")
			publicParameters = append(publicParameters, parameter.Name+" []byte")
			registerArguments = append(registerArguments, "(*C.uint8_t)("+local+")", length)
			unregisterArguments = append(unregisterArguments, "(*C.uint8_t)(kinmokuseiRegistrationState."+field+")", "kinmokuseiRegistrationState."+field+"Length")
			allocationDeclarations = append(allocationDeclarations, "var "+local+" unsafe.Pointer\n")
			allocations = append(allocations, "if len("+parameter.Name+") != 0 { "+local+" = C.CBytes("+parameter.Name+") }\n"+length+" := C.size_t(len("+parameter.Name+"))\n")
			localCleanups = append(localCleanups, "if "+local+" != nil { C.kinmokusei_cffi_free_bytes("+local+") }\n")
			closeCleanups = append(closeCleanups, "if kinmokuseiRegistrationState."+field+" != nil { C.kinmokusei_cffi_free_bytes(kinmokuseiRegistrationState."+field+") }\n")
			initializers = append(initializers, field+": "+local, field+"Length: "+length)
		} else if parameter.Type == "retainedArray" {
			element := cffiTypeInfo(parameter.Element, namedTypes)
			local := "kinmokuseiArray" + strconv.Itoa(index)
			length := local + "Length"
			fields = append(fields, field+" *"+element.cgoType, field+"Length C.size_t")
			publicParameters = append(publicParameters, parameter.Name+" []"+element.goType)
			registerArguments = append(registerArguments, local, length)
			unregisterArguments = append(unregisterArguments, "kinmokuseiRegistrationState."+field, "kinmokuseiRegistrationState."+field+"Length")
			allocationDeclarations = append(allocationDeclarations, "var "+local+" *"+element.cgoType+"\n")
			allocations = append(allocations, cffiRetainedArrayPreparation(parameter, local, element), length+" := C.size_t(len("+parameter.Name+"))\n")
			localCleanups = append(localCleanups, "if "+local+" != nil { C.kinmokusei_cffi_free_bytes(unsafe.Pointer("+local+")) }\n")
			closeCleanups = append(closeCleanups, "if kinmokuseiRegistrationState."+field+" != nil { C.kinmokusei_cffi_free_bytes(unsafe.Pointer(kinmokuseiRegistrationState."+field+")) }\n")
			initializers = append(initializers, field+": "+local, field+"Length: "+length)
		} else {
			typeInfo := cffiTypeInfo(parameter.Type, namedTypes)
			fields = append(fields, field+" "+typeInfo.goType)
			publicParameters = append(publicParameters, parameter.Name+" "+typeInfo.goType)
			registerArguments = append(registerArguments, typeInfo.toC(parameter.Name))
			unregisterArguments = append(unregisterArguments, typeInfo.toC("kinmokuseiRegistrationState."+field))
			initializers = append(initializers, field+": "+parameter.Name)
		}
	}
	publicParameters = append(publicParameters, "callback "+callback.Name)
	parameterNames = append(parameterNames, "callback")
	registerArguments = append(registerArguments, "C.uintptr_t(context)")
	unregisterArguments = append(unregisterArguments, "C.uintptr_t(kinmokuseiRegistrationState.context)")
	source.WriteString("\ntype " + ownershipType + " struct { " + strings.Join(fields, "; ") + " }\n")
	source.WriteString("type " + registration.Name + " struct { self *" + registration.Name + "; *" + ownershipType + " }\n")
	source.WriteString("func Register" + registration.Name + "(" + strings.Join(publicParameters, ", ") + ") (*" + registration.Name + ", error) {\n")
	if policy == "threadAffine" {
		source.WriteString("var result *" + registration.Name + "\nvar resultError error\nkinmokuseiCFFIDo(func() { result, resultError = " + rawRegister + "(" + strings.Join(parameterNames, ", ") + ") })\nreturn result, resultError\n}\n")
	} else {
		source.WriteString("return " + rawRegister + "(" + strings.Join(parameterNames, ", ") + ")\n}\n")
	}
	source.WriteString("func " + rawRegister + "(" + strings.Join(publicParameters, ", ") + ") (*" + registration.Name + ", error) {\n")
	if policy == "mainThread" {
		generateCFFIMainThreadGuard(source, "return nil, ErrWrongThread")
	}
	source.WriteString("if callback == nil { return nil, ErrNilCallback }\n")
	for _, check := range stringChecks {
		source.WriteString(check)
	}
	for _, name := range coupledHandleNames {
		generateCFFIIdentityGuard(source, name, "return nil, ErrClosedHandle", "return nil, ErrCopiedHandle")
	}
	for _, snapshot := range handleSnapshots {
		source.WriteString(snapshot)
	}
	// Install rollback before the first allocation: a later typed array can
	// fail its size/allocation check after earlier retained inputs were copied.
	// On native failure the later context defer drains callbacks first, then
	// this defer frees every successfully allocated input exactly once.
	source.WriteString("kinmokuseiRegistered := false\n")
	for _, declaration := range allocationDeclarations {
		source.WriteString(declaration)
	}
	if len(localCleanups) != 0 {
		source.WriteString("defer func() { if !kinmokuseiRegistered {\n")
		for _, cleanup := range localCleanups {
			source.WriteString(cleanup)
		}
		source.WriteString("} }()\n")
	}
	for _, allocation := range allocations {
		source.WriteString(allocation)
	}
	source.WriteString("state := &" + stateType + "{callback: callback}\ncontext := cgo.NewHandle(state)\n")
	// Acquire all leases transactionally. Every failed path drains admitted
	// callbacks and frees retained inputs only after the resource locks release.
	source.WriteString("defer func() { if !kinmokuseiRegistered { state.stop(); state.wait()\n")
	source.WriteString("context.Delete()\n} }()\n")
	generateCFFIRegistrationGlobalLock(source, policy)
	registrationLocks := generateCFFIHandleLockGroup(source, coupledHandleSnapshots, multipleHandles, "return nil, ErrClosedHandle")
	source.WriteString("status := int32(C.kinmokusei_cffi_register_" + registration.Name + "(" + strings.Join(registerArguments, ", ") + "))\n")
	source.WriteString("if status != 0 { return nil, &StatusError{Function: " + strconv.Quote("Register"+registration.Name) + ", Code: status} }\n")
	registrationLocks.changeLeases(source, true)
	source.WriteString("kinmokuseiRegistered = true\n")
	source.WriteString("kinmokuseiRegistration := &" + registration.Name + "{" + ownershipType + ": &" + ownershipType + "{" + strings.Join(initializers, ", ") + "}}\n")
	source.WriteString("kinmokuseiRegistration.self = kinmokuseiRegistration\nreturn kinmokuseiRegistration, nil\n}\n")
	source.WriteString("func (registration *" + registration.Name + ") CallbackError() error {\n")
	generateCFFIIdentityGuard(source, "registration", "return ErrClosedCallbackRegistration", "return ErrCopiedCallbackRegistration")
	source.WriteString("if registration.state == nil { return ErrClosedCallbackRegistration }; return registration.state.callbackError(" + strconv.Quote(registration.Name) + ") }\n")
	source.WriteString("func (registration *" + registration.Name + ") Close() error {\n")
	if policy == "threadAffine" {
		source.WriteString("var result error\nkinmokuseiCFFIDo(func() { result = registration." + rawClose + "() })\nreturn result\n}\n")
	} else {
		source.WriteString("return registration." + rawClose + "()\n}\n")
	}
	source.WriteString("func (registration *" + registration.Name + ") " + rawClose + "() error {\n")
	if policy == "mainThread" {
		generateCFFIMainThreadGuard(source, "return ErrWrongThread")
	}
	generateCFFIIdentityGuard(source, "registration", "return ErrClosedCallbackRegistration", "return ErrCopiedCallbackRegistration")
	source.WriteString("kinmokuseiRegistrationState := registration." + ownershipType + "\n")
	source.WriteString("kinmokuseiRegistrationState.mutex.Lock()\ndefer kinmokuseiRegistrationState.mutex.Unlock()\n")
	source.WriteString("if kinmokuseiRegistrationState.closed || kinmokuseiRegistrationState.state == nil || kinmokuseiRegistrationState.context == 0 { return ErrClosedCallbackRegistration }\nkinmokuseiRegistrationState.state.stop()\n")
	generateCFFIRegistrationGlobalLock(source, policy)
	closeLocks := generateCFFIHandleLockGroup(source, coupledHandleFields, multipleHandles, "kinmokuseiRegistrationState.state.resume(); return ErrClosedHandle")
	source.WriteString("status := int32(C.kinmokusei_cffi_unregister_" + registration.Name + "(" + strings.Join(unregisterArguments, ", ") + "))\n")
	closeLocks.unlock(source)
	generateCFFIRegistrationGlobalUnlock(source, policy)
	source.WriteString("if status != 0 { kinmokuseiRegistrationState.state.resume(); return &StatusError{Function: " + strconv.Quote(registration.Name+".Close") + ", Code: status} }\n")
	source.WriteString("kinmokuseiRegistrationState.state.wait()\n")
	closeLocks.lock(source)
	closeLocks.changeLeases(source, false)
	closeLocks.unlock(source)
	for _, cleanup := range closeCleanups {
		source.WriteString(cleanup)
	}
	source.WriteString("kinmokuseiRegistrationState.context.Delete()\nkinmokuseiRegistrationState.context = 0\nkinmokuseiRegistrationState.closed = true\nreturn nil\n}\n")
}

// Release the global lock before draining registered callbacks. A deferred
// fallback also releases it if handle validation returns before the C call.
func generateCFFIRegistrationGlobalLock(source *strings.Builder, policy string) {
	if policy != "serialized" {
		return
	}
	source.WriteString("kinmokuseiCFFIMutex.Lock()\nkinmokuseiGlobalLocked := true\n")
	source.WriteString("defer func() { if kinmokuseiGlobalLocked { kinmokuseiCFFIMutex.Unlock() } }()\n")
}

func generateCFFIRegistrationGlobalUnlock(source *strings.Builder, policy string) {
	if policy == "serialized" {
		source.WriteString("kinmokuseiGlobalLocked = false\nkinmokuseiCFFIMutex.Unlock()\n")
	}
}
