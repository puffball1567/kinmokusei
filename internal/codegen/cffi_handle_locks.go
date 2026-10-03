package codegen

import "strings"

// A stable creation ID orders locks across every generated handle type. This
// avoids declaration-order cycles, and repeated arguments lock one object only
// once, for both ordinary functions and callback-registration leases.
func generateCFFIFunctionHandleLocks(source *strings.Builder, function cffiFunction, handles map[string]cffiHandle, handleResult bool, result cffiScalar, multipleHandles bool) {
	parameters := make([]string, 0, len(function.Parameters))
	for _, parameter := range function.Parameters {
		if _, exists := handles[parameter.Type]; exists {
			parameters = append(parameters, parameter.Name)
		}
	}
	if len(parameters) == 0 {
		return
	}
	failure := cffiFailureReturn(function, handleResult, result, "ErrClosedHandle")
	copiedFailure := cffiFailureReturn(function, handleResult, result, "ErrCopiedHandle")
	for _, name := range parameters {
		generateCFFIIdentityGuard(source, name, failure, copiedFailure)
	}
	generateCFFIHandleLockGroup(source, parameters, multipleHandles, failure)
}

type cffiHandleLockGroup struct {
	leases   string
	multiple bool
}

// The closures capture mutex/lease pointers, not mutable public wrappers.
// Close may release these locks before draining callbacks and reacquire them
// only to remove leases. Deferred unlock also covers every early return.
func generateCFFIHandleLockGroup(source *strings.Builder, parameters []string, multipleHandles bool, failure string) cffiHandleLockGroup {
	if len(parameters) == 0 {
		return cffiHandleLockGroup{}
	}
	if len(parameters) == 1 {
		name := parameters[0]
		source.WriteString("kinmokuseiHandleMutex := &" + name + ".mutex\n")
		source.WriteString("kinmokuseiHandleRegistrations := &" + name + ".registrations\n")
		// Only registrations use the lease pointer; retain legal function output.
		source.WriteString("_ = kinmokuseiHandleRegistrations\nkinmokuseiHandlesLocked := false\n")
		source.WriteString("kinmokuseiLockHandles := func() { if !kinmokuseiHandlesLocked { kinmokuseiHandleMutex.Lock(); kinmokuseiHandlesLocked = true } }\n")
		source.WriteString("kinmokuseiUnlockHandles := func() { if kinmokuseiHandlesLocked { kinmokuseiHandlesLocked = false; kinmokuseiHandleMutex.Unlock() } }\n")
		source.WriteString("kinmokuseiLockHandles()\ndefer kinmokuseiUnlockHandles()\n")
		source.WriteString("if " + name + ".closed || " + name + ".pointer == nil { " + failure + " }\n")
		return cffiHandleLockGroup{leases: "(*kinmokuseiHandleRegistrations)"}
	}
	if !multipleHandles {
		panic("multiple C FFI handles require ordered lock support")
	}
	items := make([]string, 0, len(parameters))
	for _, name := range parameters {
		source.WriteString("if " + name + ".id == 0 { " + failure + " }\n")
		items = append(items, "{id: "+name+".id, mutex: &"+name+".mutex, registrations: &"+name+".registrations}")
	}
	source.WriteString("kinmokuseiHandleLocks := []kinmokuseiCFFIHandleLock{" + strings.Join(items, ", ") + "}\n")
	source.WriteString("sort.Slice(kinmokuseiHandleLocks, func(left, right int) bool { return kinmokuseiHandleLocks[left].id < kinmokuseiHandleLocks[right].id })\n")
	source.WriteString("kinmokuseiLockedHandles := make([]kinmokuseiCFFIHandleLock, 0, len(kinmokuseiHandleLocks))\n")
	source.WriteString("for _, item := range kinmokuseiHandleLocks {\n")
	source.WriteString("if len(kinmokuseiLockedHandles) != 0 && kinmokuseiLockedHandles[len(kinmokuseiLockedHandles)-1].mutex == item.mutex { continue }\n")
	source.WriteString("kinmokuseiLockedHandles = append(kinmokuseiLockedHandles, item)\n}\n")
	source.WriteString("kinmokuseiHandlesLocked := false\n")
	source.WriteString("kinmokuseiLockHandles := func() { if !kinmokuseiHandlesLocked { for _, item := range kinmokuseiLockedHandles { item.mutex.Lock() }; kinmokuseiHandlesLocked = true } }\n")
	source.WriteString("kinmokuseiUnlockHandles := func() { if kinmokuseiHandlesLocked { kinmokuseiHandlesLocked = false; for index := len(kinmokuseiLockedHandles)-1; index >= 0; index-- { kinmokuseiLockedHandles[index].mutex.Unlock() } } }\n")
	source.WriteString("kinmokuseiLockHandles()\ndefer kinmokuseiUnlockHandles()\n")
	for _, name := range parameters {
		source.WriteString("if " + name + ".closed || " + name + ".pointer == nil { " + failure + " }\n")
	}
	return cffiHandleLockGroup{leases: "(*item.registrations)", multiple: true}
}

func (group cffiHandleLockGroup) changeLeases(source *strings.Builder, increase bool) {
	if group.leases == "" {
		return
	}
	operation := "--"
	if increase {
		operation = "++"
	}
	if group.multiple {
		source.WriteString("for _, item := range kinmokuseiLockedHandles { " + group.leases + operation + " }\n")
	} else {
		source.WriteString(group.leases + operation + "\n")
	}
}

func (group cffiHandleLockGroup) unlock(source *strings.Builder) {
	if group.leases != "" {
		source.WriteString("kinmokuseiUnlockHandles()\n")
	}
}

func (group cffiHandleLockGroup) lock(source *strings.Builder) {
	if group.leases != "" {
		source.WriteString("kinmokuseiLockHandles()\n")
	}
}
