package sema

import (
	"fmt"
	gotypes "go/types"
)

func kinmokuseiFunctionFromGo(signature *gotypes.Signature) (Type, error) {
	return kinmokuseiFunctionFromGoSeen(signature, map[gotypes.Type]bool{})
}

func kinmokuseiFunctionFromGoSeen(signature *gotypes.Signature, visiting map[gotypes.Type]bool) (Type, error) {
	if signature.TypeParams() != nil && signature.TypeParams().Len() != 0 {
		return Type{Kind: Function, Name: "generic function", Generic: true, GoType: signature, Result: &Type{Kind: Invalid, Name: "<generic result>"}}, nil
	}
	parameters := make([]Type, signature.Params().Len())
	for i := range parameters {
		parameterType := signature.Params().At(i).Type()
		if signature.Variadic() && i == len(parameters)-1 {
			slice, ok := gotypes.Unalias(parameterType).(*gotypes.Slice)
			if !ok {
				return Type{}, fmt.Errorf("variadic parameter %d has unexpected Go type %s", i+1, parameterType.String())
			}
			parameterType = slice.Elem()
		}
		converted, err := kinmokuseiTypeFromGoSeen(parameterType, visiting)
		if err != nil {
			return Type{}, fmt.Errorf("parameter %d: %w", i+1, err)
		}
		parameters[i] = converted
	}
	var result Type
	switch signature.Results().Len() {
	case 0:
		result = builtins["void"]
	case 1:
		converted, err := kinmokuseiTypeFromGoSeen(signature.Results().At(0).Type(), visiting)
		if err != nil {
			return Type{}, fmt.Errorf("result: %w", err)
		}
		result = converted
	default:
		results := make([]Type, signature.Results().Len())
		for i := range results {
			converted, err := kinmokuseiTypeFromGoSeen(signature.Results().At(i).Type(), visiting)
			if err != nil {
				return Type{}, fmt.Errorf("result %d: %w", i+1, err)
			}
			results[i] = converted
		}
		result = Type{Kind: MultiValue, Name: "multiple values", Results: results}
	}
	return Type{Kind: Function, Name: "function", Parameters: parameters, Variadic: signature.Variadic(), Result: &result, GoType: signature}, nil
}

func kinmokuseiTypeFromGo(goType gotypes.Type) (Type, error) {
	return kinmokuseiTypeFromGoSeen(goType, map[gotypes.Type]bool{})
}

func kinmokuseiTypeFromGoSeen(goType gotypes.Type, visiting map[gotypes.Type]bool) (Type, error) {
	if parameter, ok := goType.(*gotypes.TypeParam); ok {
		return Type{Kind: TypeParameter, Name: parameter.Obj().Name(), GoType: parameter}, nil
	}
	if visiting[goType] {
		return Type{Kind: GoNamed, Name: goTypeDisplayName(goType), GoType: goType}, nil
	}
	visiting[goType] = true
	defer delete(visiting, goType)
	switch goType := goType.(type) {
	case *gotypes.Basic:
		switch goType.Kind() {
		case gotypes.Bool, gotypes.UntypedBool:
			return builtins["boolean"], nil
		case gotypes.String, gotypes.UntypedString:
			return builtins["string"], nil
		case gotypes.Int:
			return builtins["int"], nil
		case gotypes.Int8:
			return builtins["int8"], nil
		case gotypes.Int16:
			return builtins["int16"], nil
		case gotypes.Int32, gotypes.UntypedRune:
			return builtins["int32"], nil
		case gotypes.Int64:
			return builtins["int64"], nil
		case gotypes.Uint:
			return builtins["uint"], nil
		case gotypes.Uint8:
			return builtins["byte"], nil
		case gotypes.Uint16:
			return builtins["uint16"], nil
		case gotypes.Uint32:
			return builtins["uint32"], nil
		case gotypes.Uint64:
			return builtins["uint64"], nil
		case gotypes.Float32:
			return builtins["float32"], nil
		case gotypes.Float64, gotypes.UntypedFloat:
			return builtins["float"], nil
		case gotypes.UntypedInt:
			return Type{Kind: UntypedInt, Name: "integer literal"}, nil
		case gotypes.Uintptr, gotypes.UnsafePointer,
			gotypes.Complex64, gotypes.Complex128, gotypes.UntypedComplex:
			return Type{Kind: GoBasic, Name: goType.Name(), GoType: goType}, nil
		default:
			return Type{}, fmt.Errorf("Go type %s is not supported", goType.String())
		}
	case *gotypes.Named:
		arguments, err := kinmokuseiTypeArgumentsFromGoSeen(goType.TypeParams(), goType.TypeArgs(), visiting)
		if err != nil {
			return Type{}, err
		}
		if signature, ok := goType.Underlying().(*gotypes.Signature); ok {
			converted, err := kinmokuseiFunctionFromGoSeen(signature, visiting)
			if err != nil {
				return Type{}, err
			}
			converted.Name = goTypeDisplayName(goType)
			converted.GoType = goType
			converted.TypeArguments = arguments
			return converted, nil
		}
		return Type{Kind: GoNamed, Name: goTypeDisplayName(goType), GoType: goType, TypeArguments: arguments}, nil
	case *gotypes.Alias:
		arguments, err := kinmokuseiTypeArgumentsFromGoSeen(goType.TypeParams(), goType.TypeArgs(), visiting)
		if err != nil {
			return Type{}, err
		}
		unalias := gotypes.Unalias(goType)
		if signature, ok := unalias.Underlying().(*gotypes.Signature); ok {
			converted, err := kinmokuseiFunctionFromGoSeen(signature, visiting)
			if err != nil {
				return Type{}, err
			}
			converted.Name = goTypeDisplayName(goType)
			converted.GoType = goType
			converted.TypeArguments = arguments
			return converted, nil
		}
		return Type{Kind: GoNamed, Name: goTypeDisplayName(goType), GoType: goType, TypeArguments: arguments}, nil
	case *gotypes.Signature:
		converted, err := kinmokuseiFunctionFromGoSeen(goType, visiting)
		if err != nil {
			return Type{}, err
		}
		converted.GoType = goType
		return converted, nil
	case *gotypes.Pointer:
		element, err := kinmokuseiTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: GoPointer, Name: "*" + element.String(), Element: &element, GoType: goType}, nil
	case *gotypes.Slice:
		element, err := kinmokuseiTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: Array, Name: "array", Element: &element}, nil
	case *gotypes.Array:
		element, err := kinmokuseiTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: FixedArray, Name: "fixed array", Element: &element, Length: goType.Len(), GoType: goType}, nil
	case *gotypes.Map:
		key, err := kinmokuseiTypeFromGoSeen(goType.Key(), visiting)
		if err != nil {
			return Type{}, err
		}
		value, err := kinmokuseiTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: Map, Name: "Map", Key: &key, Element: &value}, nil
	case *gotypes.Chan:
		element, err := kinmokuseiTypeFromGoSeen(goType.Elem(), visiting)
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: GoChannel, Name: "Go channel", Element: &element, GoType: goType}, nil
	case *gotypes.Struct:
		fields := make([]GoStructField, goType.NumFields())
		for index := 0; index < goType.NumFields(); index++ {
			field := goType.Field(index)
			converted, err := kinmokuseiTypeFromGoSeen(field.Type(), visiting)
			if err != nil {
				return Type{}, fmt.Errorf("field %s: %w", field.Name(), err)
			}
			fields[index] = GoStructField{Name: field.Name(), Type: converted, Tag: goType.Tag(index), Embedded: field.Embedded()}
		}
		return Type{Kind: GoStruct, Name: goTypeDisplayName(goType), GoType: goType, GoFields: fields}, nil
	case *gotypes.Interface:
		if reason := unsupportedGoInteropTypeReason(goType, "type", map[gotypes.Type]bool{}); reason != "" {
			return Type{}, fmt.Errorf("%s", reason)
		}
		methods := make([]GoInterfaceMethod, goType.NumMethods())
		for i := range methods {
			method := goType.Method(i)
			converted, err := kinmokuseiTypeFromGoSeen(method.Type(), visiting)
			if err != nil {
				return Type{}, fmt.Errorf("method %s: %w", method.Name(), err)
			}
			methods[i] = GoInterfaceMethod{Name: method.Name(), Type: converted}
		}
		return Type{Kind: GoInterface, Name: goTypeDisplayName(goType), GoType: goType, GoMethods: methods}, nil
	default:
		return Type{}, fmt.Errorf("Go type %s is not supported", goType.String())
	}
}

func goTypeContainsUnsafePointer(goType gotypes.Type, seen map[gotypes.Type]bool) bool {
	if goType == nil {
		return false
	}
	if seen == nil {
		seen = map[gotypes.Type]bool{}
	}
	if seen[goType] {
		return false
	}
	seen[goType] = true
	switch typed := goType.(type) {
	case *gotypes.Basic:
		return typed.Kind() == gotypes.UnsafePointer
	case *gotypes.Alias:
		return goTypeContainsUnsafePointer(gotypes.Unalias(typed), seen) || goTypeListContainsUnsafe(typed.TypeArgs(), seen)
	case *gotypes.Named:
		if goTypeListContainsUnsafe(typed.TypeArgs(), seen) {
			return true
		}
		_, basicUnderlying := typed.Underlying().(*gotypes.Basic)
		return basicUnderlying && goTypeContainsUnsafePointer(typed.Underlying(), seen)
	case *gotypes.Pointer:
		return goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Slice:
		return goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Array:
		return goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Map:
		return goTypeContainsUnsafePointer(typed.Key(), seen) || goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Chan:
		return goTypeContainsUnsafePointer(typed.Elem(), seen)
	case *gotypes.Signature:
		return goTupleContainsUnsafe(typed.Params(), seen) || goTupleContainsUnsafe(typed.Results(), seen)
	case *gotypes.Tuple:
		return goTupleContainsUnsafe(typed, seen)
	case *gotypes.Interface:
		typed.Complete()
		for index := 0; index < typed.NumMethods(); index++ {
			if goTypeContainsUnsafePointer(typed.Method(index).Type(), seen) {
				return true
			}
		}
	case *gotypes.TypeParam:
		return goTypeContainsUnsafePointer(typed.Constraint(), seen)
	}
	return false
}

func goTupleContainsUnsafe(tuple *gotypes.Tuple, seen map[gotypes.Type]bool) bool {
	if tuple == nil {
		return false
	}
	for index := 0; index < tuple.Len(); index++ {
		if goTypeContainsUnsafePointer(tuple.At(index).Type(), seen) {
			return true
		}
	}
	return false
}

func goTypeListContainsUnsafe(types *gotypes.TypeList, seen map[gotypes.Type]bool) bool {
	if types == nil {
		return false
	}
	for index := 0; index < types.Len(); index++ {
		if goTypeContainsUnsafePointer(types.At(index), seen) {
			return true
		}
	}
	return false
}

func kinmokuseiTypeArgumentsFromGo(parameters *gotypes.TypeParamList, arguments *gotypes.TypeList) ([]Type, error) {
	return kinmokuseiTypeArgumentsFromGoSeen(parameters, arguments, map[gotypes.Type]bool{})
}

func kinmokuseiTypeArgumentsFromGoSeen(parameters *gotypes.TypeParamList, arguments *gotypes.TypeList, visiting map[gotypes.Type]bool) ([]Type, error) {
	parameterCount := 0
	if parameters != nil {
		parameterCount = parameters.Len()
	}
	argumentCount := 0
	if arguments != nil {
		argumentCount = arguments.Len()
	}
	if parameterCount != 0 && argumentCount == 0 {
		return nil, fmt.Errorf("generic Go type requires %d type arguments", parameterCount)
	}
	converted := make([]Type, argumentCount)
	for i := range converted {
		argument, err := kinmokuseiTypeFromGoSeen(arguments.At(i), visiting)
		if err != nil {
			return nil, fmt.Errorf("type argument %d: %w", i+1, err)
		}
		converted[i] = argument
	}
	return converted, nil
}

func goTypeDisplayName(goType gotypes.Type) string {
	return gotypes.TypeString(goType, func(pkg *gotypes.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Name()
	})
}
