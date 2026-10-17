package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/source"
)

type pendingMapKey struct {
	typeInfo Type
	span     source.Span
}

// Imported Go inspection traverses nested storage (including pointers and type
// arguments), unlike comparability. Do not inspect a half-built named graph.
func nativeStorageGraphReady(value gotypes.Type, seen map[gotypes.Type]bool) bool {
	if value == nil {
		return false
	}
	if seen[value] {
		return true
	}
	seen[value] = true
	switch value := gotypes.Unalias(value).(type) {
	case *gotypes.Named:
		if value.Origin().Underlying() == nil {
			return false
		}
		for i := 0; i < value.TypeArgs().Len(); i++ {
			if !nativeStorageGraphReady(value.TypeArgs().At(i), seen) {
				return false
			}
		}
		if origin := value.Origin(); origin != value {
			if seen[origin] {
				return true // Bound expanding generic graphs by declaration identity.
			}
			seen[origin] = true
		}
		return nativeStorageGraphReady(value.Underlying(), seen)
	case *gotypes.Pointer:
		return nativeStorageGraphReady(value.Elem(), seen)
	case *gotypes.Array:
		return nativeStorageGraphReady(value.Elem(), seen)
	case *gotypes.Slice:
		return nativeStorageGraphReady(value.Elem(), seen)
	case *gotypes.Map:
		return nativeStorageGraphReady(value.Key(), seen) && nativeStorageGraphReady(value.Elem(), seen)
	case *gotypes.Chan:
		return nativeStorageGraphReady(value.Elem(), seen)
	case *gotypes.Struct:
		for i := 0; i < value.NumFields(); i++ {
			if !nativeStorageGraphReady(value.Field(i).Type(), seen) {
				return false
			}
		}
	case *gotypes.Signature:
		return nativeStorageGraphReady(value.Params(), seen) && nativeStorageGraphReady(value.Results(), seen)
	case *gotypes.Tuple:
		for i := 0; i < value.Len(); i++ {
			if !nativeStorageGraphReady(value.At(i).Type(), seen) {
				return false
			}
		}
	case *gotypes.Interface:
		value.Complete()
		for i := 0; i < value.NumMethods(); i++ {
			if !nativeStorageGraphReady(value.Method(i).Type(), seen) {
				return false
			}
		}
	}
	return true
}

// Comparability only follows by-value struct/array storage. Pointers, channels,
// interfaces and non-comparable containers do not need their element storage.
// An unfinished named type, or a by-value cycle, is unsafe for go/types.Comparable.
func comparableStorageReady(value gotypes.Type, visiting map[gotypes.Type]bool) bool {
	if value == nil || visiting[value] {
		return false
	}
	visiting[value] = true
	defer delete(visiting, value)
	switch value := gotypes.Unalias(value).(type) {
	case *gotypes.Named:
		origin := value.Origin()
		if origin.Underlying() == nil {
			return false
		}
		// Check the declaration layout before expanding actual arguments. This
		// rejects by-value S<T> -> S<T[]> cycles without unfolding them forever,
		// while finite Box<Box<int>> arguments remain valid.
		if origin != value && !comparableStorageReady(origin, visiting) {
			return false
		}
		return comparableStorageReady(value.Underlying(), visiting)
	case *gotypes.Array:
		return comparableStorageReady(value.Elem(), visiting)
	case *gotypes.Struct:
		for i := 0; i < value.NumFields(); i++ {
			if !comparableStorageReady(value.Field(i).Type(), visiting) {
				return false
			}
		}
	}
	return true
}

func (c *Checker) checkResolvedMapKey(key Type, span source.Span) {
	if key.Kind == Invalid {
		return
	}
	if !c.nativeInterfaceArgumentsReady {
		// Source struct descriptors and named underlyings are provisional during
		// declarations. Rebuild canonical storage after all declarations finish.
		c.pendingMapKeys = append(c.pendingMapKeys, pendingMapKey{key, span})
		return
	}
	if storage, ok := c.goTypeForNativeStorage(key); ok {
		if !comparableStorageReady(storage, map[gotypes.Type]bool{}) {
			c.report(span, fmt.Sprintf("type %s cannot be used as a Map key: storage is unresolved or cyclic", key.String()))
			return
		}
		key.GoType = storage
	}
	if !key.IsComparable() {
		c.report(span, fmt.Sprintf("type %s cannot be used as a Map key", key.String()))
	}
}

func (c *Checker) validatePendingMapKeys() {
	keys := c.pendingMapKeys
	c.pendingMapKeys = nil
	for _, key := range keys {
		c.checkResolvedMapKey(key.typeInfo, key.span)
	}
}

func (c *Checker) validatePendingNativeGoInstances() {
	instances := c.pendingNativeGoInstances
	c.pendingNativeGoInstances = nil
	for _, instance := range instances {
		ready := true
		if origin, ok := gotypes.Unalias(instance.origin).(*gotypes.Named); ok {
			ready = origin.Origin().Underlying() != nil
		}
		for _, argument := range instance.arguments {
			ready = ready && comparableStorageReady(argument, map[gotypes.Type]bool{})
		}
		if !ready {
			c.report(instance.ref.Span, fmt.Sprintf("cannot instantiate generic defined type %s: argument storage is unresolved or cyclic", instance.ref.Name))
			continue
		}
		if _, err := gotypes.Instantiate(nil, instance.origin, instance.arguments, true); err != nil {
			c.report(instance.ref.Span, fmt.Sprintf("cannot instantiate generic defined type %s: %v", instance.ref.Name, err))
		}
	}
}
