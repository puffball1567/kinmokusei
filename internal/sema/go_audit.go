package sema

import (
	"fmt"
	gotypes "go/types"
)

type GoInteropSupport string

const (
	GoInteropSupported      GoInteropSupport = "supported"
	GoInteropRequiresUnsafe GoInteropSupport = "requires_unsafe"
	GoInteropUnsupported    GoInteropSupport = "unsupported"
)

type GoInteropAssessment struct {
	Support GoInteropSupport
	Reason  string
}

// AssessGoInteropObject classifies an exported Go declaration using the same
// type shapes accepted by direct interop. It deliberately distinguishes APIs
// that only need the explicit unsafe policy from genuinely unsupported APIs.
func AssessGoInteropObject(object gotypes.Object) GoInteropAssessment {
	if object == nil {
		return GoInteropAssessment{Support: GoInteropUnsupported, Reason: "missing Go object"}
	}
	var goType gotypes.Type
	switch object := object.(type) {
	case *gotypes.Const, *gotypes.Var, *gotypes.TypeName:
		goType = object.Type()
	case *gotypes.Func:
		goType = object.Type()
	default:
		return GoInteropAssessment{Support: GoInteropUnsupported, Reason: fmt.Sprintf("Go declaration kind %T is not supported", object)}
	}
	if reason := unsupportedGoInteropTypeReason(goType, "type", map[gotypes.Type]bool{}); reason != "" {
		return GoInteropAssessment{Support: GoInteropUnsupported, Reason: reason}
	}
	if goTypeContainsUnsafePointer(goType, nil) {
		return GoInteropAssessment{Support: GoInteropRequiresUnsafe, Reason: `uses unsafe.Pointer and requires [go.interop] unsafe = "allow"`}
	}
	return GoInteropAssessment{Support: GoInteropSupported}
}

func unsupportedGoInteropTypeReason(goType gotypes.Type, path string, seen map[gotypes.Type]bool) string {
	if goType == nil {
		return path + " is missing"
	}
	if seen[goType] {
		return ""
	}
	seen[goType] = true
	switch typed := goType.(type) {
	case *gotypes.Basic:
		if _, err := ontamaTypeFromGo(typed); err != nil {
			return path + " uses " + err.Error()
		}
		return ""
	case *gotypes.Named:
		if signature, ok := typed.Underlying().(*gotypes.Signature); ok {
			return unsupportedGoInteropTypeReason(signature, path, seen)
		}
		return unsupportedGoTypeArgumentsReason(typed.TypeArgs(), path, seen)
	case *gotypes.Alias:
		if signature, ok := gotypes.Unalias(typed).Underlying().(*gotypes.Signature); ok {
			return unsupportedGoInteropTypeReason(signature, path, seen)
		}
		return unsupportedGoTypeArgumentsReason(typed.TypeArgs(), path, seen)
	case *gotypes.Signature:
		for index := 0; index < typed.Params().Len(); index++ {
			parameterType := typed.Params().At(index).Type()
			if typed.Variadic() && index == typed.Params().Len()-1 {
				slice, ok := gotypes.Unalias(parameterType).(*gotypes.Slice)
				if !ok {
					return fmt.Sprintf("parameter %d has an invalid variadic type %s", index+1, parameterType.String())
				}
				parameterType = slice.Elem()
			}
			if reason := unsupportedGoInteropTypeReason(parameterType, fmt.Sprintf("parameter %d", index+1), seen); reason != "" {
				return reason
			}
		}
		for index := 0; index < typed.Results().Len(); index++ {
			if reason := unsupportedGoInteropTypeReason(typed.Results().At(index).Type(), fmt.Sprintf("result %d", index+1), seen); reason != "" {
				return reason
			}
		}
		return ""
	case *gotypes.Pointer:
		return unsupportedGoInteropTypeReason(typed.Elem(), path+" pointer element", seen)
	case *gotypes.Slice:
		return unsupportedGoInteropTypeReason(typed.Elem(), path+" slice element", seen)
	case *gotypes.Array:
		return unsupportedGoInteropTypeReason(typed.Elem(), path+" array element", seen)
	case *gotypes.Map:
		if reason := unsupportedGoInteropTypeReason(typed.Key(), path+" map key", seen); reason != "" {
			return reason
		}
		return unsupportedGoInteropTypeReason(typed.Elem(), path+" map value", seen)
	case *gotypes.Chan:
		return unsupportedGoInteropTypeReason(typed.Elem(), path+" channel element", seen)
	case *gotypes.TypeParam:
		return ""
	case *gotypes.Struct:
		for index := 0; index < typed.NumFields(); index++ {
			field := typed.Field(index)
			if reason := unsupportedGoInteropTypeReason(field.Type(), fmt.Sprintf("%s field %s", path, field.Name()), seen); reason != "" {
				return reason
			}
		}
		return ""
	case *gotypes.Interface:
		return path + " uses an anonymous Go interface type"
	case *gotypes.Tuple:
		return path + " uses a Go tuple outside a function result"
	case *gotypes.Union:
		return path + " uses a Go union type"
	default:
		return fmt.Sprintf("%s uses unsupported Go type %T", path, goType)
	}
}

func unsupportedGoTypeArgumentsReason(arguments *gotypes.TypeList, path string, seen map[gotypes.Type]bool) string {
	if arguments == nil {
		return ""
	}
	for index := 0; index < arguments.Len(); index++ {
		if reason := unsupportedGoInteropTypeReason(arguments.At(index), fmt.Sprintf("%s type argument %d", path, index+1), seen); reason != "" {
			return reason
		}
	}
	return ""
}
