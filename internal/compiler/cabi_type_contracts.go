package compiler

import (
	"fmt"
	"math/big"

	"github.com/puffball1567/kinmokusei/internal/cabi"
)

func compareCABIType(result *CABICompatibility, location string, baseline, current cabi.Type) {
	if baseline.Kind != current.Kind {
		result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf("changed semantic type of %s from %s to %s", location, baseline.Kind, current.Kind))
		return
	}
	if baseline.Kind != "enum" {
		return
	}
	currentMembers := make(map[string]string, len(current.Members))
	baselineMembers := make(map[string]bool, len(baseline.Members))
	for _, member := range current.Members {
		currentMembers[member.Name] = member.Value
	}
	for _, member := range baseline.Members {
		baselineMembers[member.Name] = true
		value, exists := currentMembers[member.Name]
		if !exists {
			result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf("removed enum member %q from %s", member.Name, location))
		} else if value != member.Value {
			result.BreakingChanges = append(result.BreakingChanges, fmt.Sprintf("changed enum member %q of %s from %s to %s", member.Name, location, member.Value, value))
		}
	}
	for _, member := range current.Members {
		if !baselineMembers[member.Name] {
			result.TypeAdditions = append(result.TypeAdditions, fmt.Sprintf("%s member %q", location, member.Name))
		}
	}
}

func validateCABIFunctionTypes(function cabi.Function) error {
	if len(function.ParameterTypes) != len(function.Parameters) || function.ParameterTypes == nil {
		return fmt.Errorf("parameterTypes must describe every parameter")
	}
	for index, transport := range function.Parameters {
		if err := validateCABIType(function.ParameterTypes[index], transport, false); err != nil {
			return fmt.Errorf("parameter %d: %w", index+1, err)
		}
	}
	if err := validateCABIType(function.ResultType, function.Result, true); err != nil {
		return fmt.Errorf("result: %w", err)
	}
	want := "out_parameter"
	if function.Result == "void" {
		want = "none"
	}
	if function.ResultTransport != want {
		return fmt.Errorf("resultTransport must be %q", want)
	}
	return nil
}

func validateCABIType(contract cabi.Type, transport string, allowVoid bool) error {
	bits, signed := cabiIntegerTransport(transport)
	valid := false
	switch contract.Kind {
	case "void":
		valid = allowVoid && transport == "void"
	case "boolean":
		valid = transport == "uint8_t"
	case "integer", "enum":
		valid = bits != 0
	case "float":
		valid = transport == "float" || transport == "double"
	}
	if !valid {
		return fmt.Errorf("invalid semantic type %q for transport %q", contract.Kind, transport)
	}
	if contract.Kind != "enum" && contract.Members != nil {
		return fmt.Errorf("only enum types may declare members")
	}
	if contract.Kind == "enum" && len(contract.Members) == 0 {
		return fmt.Errorf("enum types must declare at least one member")
	}
	previous := ""
	for index, member := range contract.Members {
		if member.Name == "" || index > 0 && member.Name <= previous {
			return fmt.Errorf("enum members must have nonempty unique names in ascending order")
		}
		previous = member.Name
		value, ok := new(big.Int).SetString(member.Value, 10)
		if !ok || value.String() != member.Value {
			return fmt.Errorf("enum member %q must have a canonical decimal integer value", member.Name)
		}
		// Signed negatives fit when -(value+1) fits in bits-1; positives
		// must reserve the sign bit. Keep uint64 values exact without floats.
		magnitude := new(big.Int).Set(value)
		limit := bits
		if signed {
			limit--
			if value.Sign() < 0 {
				magnitude.Neg(magnitude).Sub(magnitude, big.NewInt(1))
			}
		}
		if !signed && value.Sign() < 0 || magnitude.BitLen() > limit {
			return fmt.Errorf("enum member %q value %s is outside %s", member.Name, member.Value, transport)
		}
	}
	return nil
}

func cabiIntegerTransport(transport string) (bits int, signed bool) {
	switch transport {
	case "int8_t":
		return 8, true
	case "int16_t":
		return 16, true
	case "int32_t":
		return 32, true
	case "int64_t":
		return 64, true
	case "uint8_t":
		return 8, false
	case "uint16_t":
		return 16, false
	case "uint32_t":
		return 32, false
	case "uint64_t":
		return 64, false
	default:
		return 0, false
	}
}
