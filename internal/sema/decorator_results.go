package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Result<T> is an effect: adapters propagate its error. A result list is data:
// every slot (including an ordinary error value) is boxed without interpreting
// its position or runtime value as a Result failure.
func decoratorInvocationResult(result Type, span source.Span) (Type, []ast.DecoratorResultSlot, string) {
	payload := result
	if result.Kind == Result && result.Element != nil {
		payload = *result.Element
	}
	if result.Kind == MultiValue && len(result.Results) != 0 {
		slots := make([]ast.DecoratorResultSlot, len(payload.Results))
		for index, value := range payload.Results {
			if !decoratorValuePayloadType(value) {
				return Type{Kind: Invalid}, nil, fmt.Sprintf("method result slot %d (%s) cannot be stored in DecoratorValue", index, value.String())
			}
			slots[index] = ast.DecoratorResultSlot{
				Type: typeRefFromType(value, span), Identity: decoratorValueRuntimeIdentity(value), Contract: decoratorValueContract(value),
			}
		}
		value := decoratorValueType()
		return Type{Kind: Array, Element: &value}, slots, ""
	}
	if payload.Kind != Void && !decoratorValuePayloadType(payload) {
		return Type{Kind: Invalid}, nil, fmt.Sprintf("method result %s cannot be stored in DecoratorValue", payload.String())
	}
	return payload, nil, ""
}
