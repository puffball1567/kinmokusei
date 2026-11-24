package codegen

import (
	goast "go/ast"
	"go/token"
	"strconv"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

const matchRestCloneName = "__kinmokuseiCloneMatchRest"

func (c *matchPatternCode) generateSequence(pattern *ast.MatchPattern, value goast.Expr) error {
	length := &goast.CallExpr{Fun: goast.NewIdent("len"), Args: []goast.Expr{value}}
	rest := pattern.SequenceRestIndex()
	size := len(pattern.Elements)
	if !pattern.FixedLength {
		if rest < 0 {
			c.addCheck(length, token.EQL, matchIndex(size))
		} else if size > 1 {
			c.addCheck(length, token.GEQ, matchIndex(size-1))
		}
	}
	fromEnd := func(distance int) goast.Expr {
		return &goast.BinaryExpr{X: length, Op: token.SUB, Y: matchIndex(distance)}
	}
	for i := range pattern.Elements {
		part := &pattern.Elements[i]
		var projection goast.Expr
		if i == rest {
			if part.Binding.Name == "_" {
				continue
			}
			view := &goast.SliceExpr{X: value, Low: matchIndex(i)}
			if suffix := size - i - 1; suffix != 0 {
				view.High = fromEnd(suffix)
			}
			projection = &goast.CallExpr{Fun: goast.NewIdent(matchRestCloneName), Args: []goast.Expr{view}}
		} else {
			index := matchIndex(i)
			if rest >= 0 && i > rest {
				index = fromEnd(size - i)
			}
			projection = &goast.IndexExpr{X: value, Index: index}
		}
		if err := c.generate(part, projection); err != nil {
			return err
		}
	}
	return nil
}

func matchIndex(index int) goast.Expr {
	return &goast.BasicLit{Kind: token.INT, Value: strconv.Itoa(index)}
}

func matchRestDeclarations() ([]goast.Decl, error) {
	return parseDeferSupport(`package p
func ` + matchRestCloneName + `[S ~[]E, E any](value S) []E {
	result := make([]E, len(value))
	copy(result, value)
	return result
}
`)
}
