package sema

import (
	gotypes "go/types"
	"sort"
	"strconv"
	"strings"
)

type TypeKind int

const (
	Invalid TypeKind = iota
	Void
	Boolean
	String
	Int
	Int32
	Int64
	Uint16
	Uint32
	Uint64
	Float32
	Float64
	Byte
	UntypedInt
	Function
	Array
	FixedArray
	Object
	Map
	GoChannel
	Class
	Struct
	Interface
	GoPackage
	GoNamed
	GoPointer
	GoBasic
	GoStruct
	GoTypeName
	Nil
	Null
	MultiValue
	Result
	Task
	Nullable
	TypeParameter
)

type Type struct {
	Kind           TypeKind
	Name           string
	Parameters     []Type
	Variadic       bool
	Generic        bool
	TypeParameters []Type
	Result         *Type
	Element        *Type
	Length         int64
	Key            *Type
	Fields         map[string]Type
	FieldNames     map[string]string
	GoPackage      *goPackageSymbol
	GoType         gotypes.Type
	GoQualifier    string
	TypeArguments  []Type
	Results        []Type
	GoFields       []GoStructField
}

type GoStructField struct {
	Name     string
	Type     Type
	Tag      string
	Embedded bool
}

var builtins = map[string]Type{
	"void":      {Kind: Void, Name: "void"},
	"boolean":   {Kind: Boolean, Name: "boolean"},
	"string":    {Kind: String, Name: "string"},
	"int":       {Kind: Int, Name: "int"},
	"int32":     {Kind: Int32, Name: "int32"},
	"int64":     {Kind: Int64, Name: "int64"},
	"uint16":    {Kind: Uint16, Name: "uint16"},
	"uint32":    {Kind: Uint32, Name: "uint32"},
	"uint64":    {Kind: Uint64, Name: "uint64"},
	"float32":   {Kind: Float32, Name: "float32"},
	"float":     {Kind: Float64, Name: "float"},
	"number":    {Kind: Float64, Name: "float"},
	"float64":   {Kind: Float64, Name: "float"},
	"byte":      {Kind: Byte, Name: "byte"},
	"error":     {Kind: GoNamed, Name: "error", GoType: gotypes.Universe.Lookup("error").Type()},
	"Exception": {Kind: Class, Name: "Exception"},
}

func LookupType(name string) (Type, bool) {
	t, ok := builtins[name]
	return t, ok
}

func (t Type) IsNumeric() bool {
	if t.GoType != nil {
		basic, ok := gotypes.Unalias(t.GoType).Underlying().(*gotypes.Basic)
		return ok && basic.Info()&gotypes.IsNumeric != 0
	}
	switch t.Kind {
	case Int, Int32, Int64, Uint16, Uint32, Uint64, Float32, Float64, Byte, UntypedInt:
		return true
	default:
		return false
	}
}

func (t Type) IsComparable() bool {
	return t.isComparable(map[string]bool{})
}

func (t Type) isComparable(visiting map[string]bool) bool {
	if t.GoType != nil {
		return gotypes.Comparable(t.GoType)
	}
	switch t.Kind {
	case Boolean, String, Int, Int32, Int64, Uint16, Uint32, Uint64, Float32, Float64, Byte, UntypedInt, Class, Interface:
		return true
	case Nullable:
		return t.Element != nil && t.Element.IsComparable()
	case FixedArray:
		return t.Element != nil && t.Element.isComparable(visiting)
	case Struct:
		if visiting[t.Name] {
			return false
		}
		visiting[t.Name] = true
		defer delete(visiting, t.Name)
		for _, field := range t.Fields {
			if !field.isComparable(visiting) {
				return false
			}
		}
		return true
	case GoPointer:
		return true
	default:
		return false
	}
}

func assignable(target, value Type) bool {
	if target.Kind == Invalid || value.Kind == Invalid {
		return true
	}
	if target.Kind == MultiValue || value.Kind == MultiValue {
		return false
	}
	if target.Kind == Nullable {
		if target.Element == nil {
			return false
		}
		if value.Kind == Nil {
			return false
		}
		if value.Kind == Nullable {
			return value.Element != nil && sameType(*target.Element, *value.Element)
		}
		if value.Kind == Null {
			return true
		}
		return assignable(*target.Element, value)
	}
	if value.Kind == Nullable || target.Kind == Null || value.Kind == Null {
		return target.Kind == Null && value.Kind == Null
	}
	if target.Kind == Result || value.Kind == Result {
		return target.Kind == Result && value.Kind == Result && target.Element != nil && value.Element != nil && sameType(*target.Element, *value.Element)
	}
	if target.Kind == Task || value.Kind == Task {
		return target.Kind == Task && value.Kind == Task && target.Element != nil && value.Element != nil && sameType(*target.Element, *value.Element)
	}
	if value.Kind == Nil {
		return target.Kind == Nil || isNilable(target)
	}
	if target.Kind == Nil {
		return value.Kind == Nil
	}
	if target.Kind == GoPointer && target.GoType == nil || value.Kind == GoPointer && value.GoType == nil {
		if target.Kind != GoPointer || value.Kind != GoPointer || target.Element == nil || value.Element == nil {
			return false
		}
		if target.GoType != nil && value.GoType != nil {
			return gotypes.AssignableTo(value.GoType, target.GoType)
		}
		return sameType(*target.Element, *value.Element)
	}
	if targetGo, targetIsGo := goTypeOf(target); targetIsGo {
		valueGo, valueIsGo := goTypeOf(value)
		return valueIsGo && gotypes.AssignableTo(valueGo, targetGo)
	}
	if _, valueIsGo := goTypeOf(value); valueIsGo {
		return false
	}
	if target.Kind == Function || value.Kind == Function {
		if target.Kind != Function || value.Kind != Function || target.Variadic != value.Variadic || target.Generic != value.Generic || len(target.Parameters) != len(value.Parameters) {
			return false
		}
		for i := range target.Parameters {
			if !sameType(target.Parameters[i], value.Parameters[i]) {
				return false
			}
		}
		return target.Result != nil && value.Result != nil && sameType(*target.Result, *value.Result)
	}
	if target.Kind == Array || value.Kind == Array || target.Kind == FixedArray || value.Kind == FixedArray {
		if target.Kind != value.Kind || target.Element == nil || value.Element == nil {
			return false
		}
		if target.Kind == FixedArray && target.Length != value.Length {
			return false
		}
		return sameType(*target.Element, *value.Element)
	}
	if target.Kind == Map || value.Kind == Map {
		return target.Kind == Map && value.Kind == Map && target.Key != nil && value.Key != nil && target.Element != nil && value.Element != nil && sameType(*target.Key, *value.Key) && sameType(*target.Element, *value.Element)
	}
	if target.Kind == Object || value.Kind == Object {
		if target.Kind != Object || value.Kind != Object || len(target.Fields) != len(value.Fields) {
			return false
		}
		for name, targetField := range target.Fields {
			valueField, ok := value.Fields[name]
			if !ok || !sameType(targetField, valueField) {
				return false
			}
		}
		return true
	}
	if target.Kind == Class || value.Kind == Class {
		return target.Kind == Class && value.Kind == Class && target.Name == value.Name
	}
	if target.Kind == Struct || value.Kind == Struct {
		if target.Kind != Struct || value.Kind != Struct || target.Name != value.Name || len(target.TypeArguments) != len(value.TypeArguments) {
			return false
		}
		for index := range target.TypeArguments {
			if !sameType(target.TypeArguments[index], value.TypeArguments[index]) {
				return false
			}
		}
		return true
	}
	if target.Kind == Interface || value.Kind == Interface {
		return target.Kind == Interface && value.Kind == Interface && target.Name == value.Name
	}
	if target.Kind == value.Kind {
		return true
	}
	return value.Kind == UntypedInt && target.IsNumeric()
}

func sameType(left, right Type) bool {
	return assignable(left, right) || assignable(right, left)
}

func (t Type) String() string {
	if t.Kind != Function {
		switch t.Kind {
		case MultiValue:
			results := make([]string, len(t.Results))
			for i := range t.Results {
				results[i] = t.Results[i].String()
			}
			return "(" + strings.Join(results, ", ") + ")"
		case Result:
			if t.Element != nil {
				return "Result<" + t.Element.String() + ">"
			}
		case Task:
			if t.Element != nil {
				return "Task<" + t.Element.String() + ">"
			}
		case Nullable:
			if t.Element != nil {
				return t.Element.String() + " | null"
			}
		case Array:
			if t.Element != nil {
				return t.Element.String() + "[]"
			}
		case FixedArray:
			if t.Element != nil {
				return "[" + strconv.FormatInt(t.Length, 10) + "]" + t.Element.String()
			}
		case Map:
			if t.Key != nil && t.Element != nil {
				return "Map<" + t.Key.String() + ", " + t.Element.String() + ">"
			}
		case GoChannel:
			if t.GoType != nil && t.Element != nil {
				channel, ok := gotypes.Unalias(t.GoType).Underlying().(*gotypes.Chan)
				if !ok {
					return t.Name
				}
				switch channel.Dir() {
				case gotypes.SendOnly:
					return "GoSendChannel<" + t.Element.String() + ">"
				case gotypes.RecvOnly:
					return "GoReceiveChannel<" + t.Element.String() + ">"
				default:
					return "GoChannel<" + t.Element.String() + ">"
				}
			}
		case Object:
			names := make([]string, 0, len(t.Fields))
			for name := range t.Fields {
				names = append(names, name)
			}
			sort.Strings(names)
			fields := make([]string, len(names))
			for index, name := range names {
				fields[index] = name + ": " + t.Fields[name].String()
			}
			return "{ " + strings.Join(fields, ", ") + " }"
		case Struct, Interface, Class:
			if len(t.TypeArguments) != 0 {
				arguments := make([]string, len(t.TypeArguments))
				for index := range t.TypeArguments {
					arguments[index] = t.TypeArguments[index].String()
				}
				return t.Name + "<" + strings.Join(arguments, ", ") + ">"
			}
		}
		return t.Name
	}
	parameters := make([]string, len(t.Parameters))
	for i, parameter := range t.Parameters {
		parameters[i] = parameter.String()
		if t.Variadic && i == len(t.Parameters)-1 {
			parameters[i] = "..." + parameters[i]
		}
	}
	result := "<invalid>"
	if t.Result != nil {
		result = t.Result.String()
	}
	return "(" + strings.Join(parameters, ", ") + ") => " + result
}

func goTypeOf(t Type) (gotypes.Type, bool) {
	if t.GoType != nil {
		return t.GoType, true
	}
	switch t.Kind {
	case Nullable:
		if t.Element != nil {
			return goTypeOf(*t.Element)
		}
	case Boolean:
		return gotypes.Typ[gotypes.Bool], true
	case String:
		return gotypes.Typ[gotypes.String], true
	case Int:
		return gotypes.Typ[gotypes.Int], true
	case Int32:
		return gotypes.Typ[gotypes.Int32], true
	case Int64:
		return gotypes.Typ[gotypes.Int64], true
	case Uint16:
		return gotypes.Typ[gotypes.Uint16], true
	case Uint32:
		return gotypes.Typ[gotypes.Uint32], true
	case Uint64:
		return gotypes.Typ[gotypes.Uint64], true
	case Float32:
		return gotypes.Typ[gotypes.Float32], true
	case Float64:
		return gotypes.Typ[gotypes.Float64], true
	case Byte:
		return gotypes.Typ[gotypes.Uint8], true
	case UntypedInt:
		return gotypes.Typ[gotypes.UntypedInt], true
	case Function:
		parameters := make([]*gotypes.Var, len(t.Parameters))
		for i, parameter := range t.Parameters {
			parameterType, ok := goTypeOf(parameter)
			if !ok {
				return nil, false
			}
			if t.Variadic && i == len(t.Parameters)-1 {
				parameterType = gotypes.NewSlice(parameterType)
			}
			parameters[i] = gotypes.NewVar(0, nil, "", parameterType)
		}
		var results []*gotypes.Var
		if t.Result == nil {
			return nil, false
		}
		switch t.Result.Kind {
		case Void:
		case MultiValue:
			results = make([]*gotypes.Var, len(t.Result.Results))
			for i, result := range t.Result.Results {
				resultType, ok := goTypeOf(result)
				if !ok {
					return nil, false
				}
				results[i] = gotypes.NewVar(0, nil, "", resultType)
			}
		case Result:
			if t.Result.Element == nil {
				return nil, false
			}
			errorType := gotypes.Universe.Lookup("error").Type()
			if t.Result.Element.Kind == Void {
				results = []*gotypes.Var{gotypes.NewVar(0, nil, "", errorType)}
				break
			}
			resultType, ok := goTypeOf(*t.Result.Element)
			if !ok {
				return nil, false
			}
			results = []*gotypes.Var{
				gotypes.NewVar(0, nil, "", resultType),
				gotypes.NewVar(0, nil, "", errorType),
			}
		default:
			resultType, ok := goTypeOf(*t.Result)
			if !ok {
				return nil, false
			}
			results = []*gotypes.Var{gotypes.NewVar(0, nil, "", resultType)}
		}
		return gotypes.NewSignatureType(nil, nil, nil, gotypes.NewTuple(parameters...), gotypes.NewTuple(results...), t.Variadic), true
	case Array:
		if t.Element != nil {
			if element, ok := goTypeOf(*t.Element); ok {
				return gotypes.NewSlice(element), true
			}
		}
	case FixedArray:
		if t.Element != nil {
			if element, ok := goTypeOf(*t.Element); ok {
				return gotypes.NewArray(element, t.Length), true
			}
		}
	case Map:
		if t.Key != nil && t.Element != nil {
			key, keyOK := goTypeOf(*t.Key)
			element, elementOK := goTypeOf(*t.Element)
			if keyOK && elementOK {
				return gotypes.NewMap(key, element), true
			}
		}
	case Object:
		names := make([]string, 0, len(t.Fields))
		for name := range t.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		fields := make([]*gotypes.Var, len(names))
		tags := make([]string, len(names))
		for index, name := range names {
			fieldType, ok := goTypeOf(t.Fields[name])
			if !ok {
				return nil, false
			}
			goName := t.FieldNames[name]
			if goName == "" {
				goName = name
			}
			fields[index] = gotypes.NewField(0, nil, goName, fieldType, false)
			tags[index] = `json:"` + name + `"`
		}
		return gotypes.NewStruct(fields, tags), true
	case GoChannel:
		if t.GoType != nil {
			return t.GoType, true
		}
	}
	return nil, false
}

func isNilable(t Type) bool {
	if t.Kind == GoPointer || t.Kind == GoChannel || t.Kind == Array || t.Kind == Map || t.Kind == Function || t.Kind == Class || t.Kind == Interface {
		return true
	}
	goType, ok := goTypeOf(t)
	if !ok {
		return false
	}
	underlying := gotypes.Unalias(goType).Underlying()
	if basic, ok := underlying.(*gotypes.Basic); ok && basic.Kind() == gotypes.UnsafePointer {
		return true
	}
	switch underlying.(type) {
	case *gotypes.Pointer, *gotypes.Slice, *gotypes.Map, *gotypes.Signature, *gotypes.Interface, *gotypes.Chan:
		return true
	default:
		return false
	}
}

func (t Type) IsString() bool {
	if t.Kind == String {
		return true
	}
	if t.GoType == nil {
		return false
	}
	basic, ok := gotypes.Unalias(t.GoType).Underlying().(*gotypes.Basic)
	return ok && basic.Info()&gotypes.IsString != 0
}

func (t Type) IsBoolean() bool {
	if t.Kind == Boolean {
		return true
	}
	if t.GoType == nil {
		return false
	}
	basic, ok := gotypes.Unalias(t.GoType).Underlying().(*gotypes.Basic)
	return ok && basic.Info()&gotypes.IsBoolean != 0
}

func (t Type) IsOrdered() bool {
	if t.GoType != nil {
		basic, ok := gotypes.Unalias(t.GoType).Underlying().(*gotypes.Basic)
		return ok && basic.Info()&gotypes.IsOrdered != 0
	}
	return t.IsNumeric() || t.IsString()
}

func (t Type) IsInteger() bool {
	if t.GoType != nil {
		basic, ok := gotypes.Unalias(t.GoType).Underlying().(*gotypes.Basic)
		return ok && basic.Info()&gotypes.IsInteger != 0
	}
	switch t.Kind {
	case Int, Int32, Int64, Uint16, Uint32, Uint64, Byte, UntypedInt:
		return true
	default:
		return false
	}
}

func defaultLiteralType(t Type) Type {
	if t.Kind == UntypedInt {
		return builtins["int"]
	}
	return t
}
