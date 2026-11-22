package codegen

import (
	goast "go/ast"
	"go/token"
	"strconv"
	"strings"
)

const regexGoTypeName = "__kinmokuseiRegex"

func regexLiteralName(index int) string {
	return regexGoTypeName + "Literal" + strconv.Itoa(index)
}

func regexRuntimeDeclarations(alias string, patterns []string) ([]goast.Decl, error) {
	declarations, err := parseDeferSupport(strings.ReplaceAll(`package runtime
type __kinmokuseiRegex struct {
	_ [0]func()
	compiled *regexp.Regexp
}

var __kinmokuseiEmptyRegex = regexp.MustCompile("")

func (r __kinmokuseiRegex) compiledPattern() *regexp.Regexp {
	if r.compiled != nil {
		return r.compiled
	}
	return __kinmokuseiEmptyRegex
}

func (r __kinmokuseiRegex) test(text string) bool {
	return r.compiledPattern().MatchString(text)
}

func (r __kinmokuseiRegex) captures(text string) []string {
	return r.compiledPattern().FindStringSubmatch(text)
}

func (r __kinmokuseiRegex) replaceAll(text, replacement string) string {
	return r.compiledPattern().ReplaceAllString(text, replacement)
}
`, "regexp.", alias+"."))
	if err != nil {
		return nil, err
	}
	for i, pattern := range patterns {
		compiled := &goast.CallExpr{Fun: &goast.SelectorExpr{X: goast.NewIdent(alias), Sel: goast.NewIdent("MustCompile")}, Args: []goast.Expr{stringLiteral(pattern)}}
		value := &goast.CompositeLit{Type: goast.NewIdent(regexGoTypeName), Elts: []goast.Expr{
			&goast.KeyValueExpr{Key: goast.NewIdent("compiled"), Value: compiled},
		}}
		declarations = append(declarations, &goast.GenDecl{Tok: token.VAR, Specs: []goast.Spec{
			&goast.ValueSpec{Names: []*goast.Ident{goast.NewIdent(regexLiteralName(i))}, Values: []goast.Expr{value}},
		}})
	}
	return declarations, nil
}
