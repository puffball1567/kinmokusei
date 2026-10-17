package codegen

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	goast "go/ast"
	"go/scanner"
	"go/token"
	"reflect"
	"strings"

	kmast "github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// SourceMapping connects a half-open physical Go token range to its innermost
// executable source origin. Compiler-created runtime helpers have no origin.
type SourceMapping struct {
	Generated source.Span
	Source    source.Span
}

const mappingMarker = "__kinmokusei_source_mapping_"

// Clone only AST-owned data. Editor maps and external metadata are read-only;
// preserving their identities avoids copying Go importer internals. Memoization
// preserves shared nodes and recursive type graphs. Emission flags live solely
// on the clone, so mapped and ordinary generation can share a checked program.
func sourceMappedProgram(program *kmast.Program) *kmast.Program {
	astPackage := reflect.TypeOf(kmast.Program{}).PkgPath()
	memo := map[any]reflect.Value{}
	var clone func(reflect.Value) reflect.Value
	clone = func(value reflect.Value) reflect.Value {
		switch value.Kind() {
		case reflect.Interface:
			if value.IsNil() {
				return value
			}
			result := reflect.New(value.Type()).Elem()
			result.Set(clone(value.Elem()))
			return result
		case reflect.Pointer:
			if value.IsNil() || value.Type().Elem().PkgPath() != astPackage {
				return value
			}
			key := value.Interface()
			if previous, ok := memo[key]; ok {
				return previous
			}
			result := reflect.New(value.Type().Elem())
			memo[key] = result
			result.Elem().Set(clone(value.Elem()))
			return result
		case reflect.Struct:
			if value.Type().PkgPath() != astPackage {
				return value
			}
			result := reflect.New(value.Type()).Elem()
			for i := 0; i < value.NumField(); i++ {
				result.Field(i).Set(clone(value.Field(i)))
			}
			if value.Type() == reflect.TypeOf(kmast.BlockStmt{}) || value.Type() == reflect.TypeOf(kmast.ArrowExpr{}) {
				result.FieldByName("SourceMapping").SetBool(true)
			}
			return result
		case reflect.Slice:
			if value.IsNil() {
				return value
			}
			result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
			for i := 0; i < value.Len(); i++ {
				result.Index(i).Set(clone(value.Index(i)))
			}
			return result
		default:
			return value
		}
	}
	return clone(reflect.ValueOf(program)).Interface().(*kmast.Program)
}

func markSourceStatements(statements []goast.Stmt, start int, span source.Span) []goast.Stmt {
	if start == len(statements) || span.Path == "" || span.Start.Line < 1 {
		return statements
	}
	encoded, _ := json.Marshal(span) // source.Span contains only strings and integers.
	payload := hex.EncodeToString(encoded)
	result := make([]goast.Stmt, 0, len(statements)+2)
	result = append(result, statements[:start]...)
	result = append(result, &goast.ExprStmt{X: goast.NewIdent(mappingMarker + "start_" + payload)})
	result = append(result, statements[start:]...)
	return append(result, &goast.ExprStmt{X: goast.NewIdent(mappingMarker + "end_" + payload)})
}

type mappingToken struct {
	kind     token.Token
	literal  string
	position source.Position
	length   int
	origin   source.Span
}

func scanMappingTokens(input []byte) ([]mappingToken, error) {
	set := token.NewFileSet()
	file := set.AddFile("generated.go", -1, len(input))
	var scan scanner.Scanner
	var scanErr error
	scan.Init(file, input, func(pos token.Position, message string) {
		if scanErr == nil {
			scanErr = fmt.Errorf("%s: %s", pos, message)
		}
	}, 0)
	var result []mappingToken
	for {
		pos, kind, literal := scan.Scan()
		if kind == token.EOF {
			break
		}
		if kind == token.SEMICOLON {
			continue
		}
		position := file.Position(pos)
		length := len(literal)
		if length == 0 {
			length = len(kind.String())
		}
		result = append(result, mappingToken{kind: kind, literal: literal, position: source.Position{Offset: position.Offset, Line: position.Line, Column: position.Column}, length: length})
	}
	return result, scanErr
}

func extractSourceMappings(input []byte) ([]byte, []SourceMapping, error) {
	tokens, err := scanMappingTokens(input)
	if err != nil {
		return nil, nil, err
	}
	var clean bytes.Buffer
	var real []mappingToken
	var stack []source.Span
	copied := 0
	for _, tok := range tokens {
		if tok.kind == token.IDENT && strings.HasPrefix(tok.literal, mappingMarker) {
			marker := strings.TrimPrefix(tok.literal, mappingMarker)
			kind, payload, ok := strings.Cut(marker, "_")
			data, decodeErr := hex.DecodeString(payload)
			var span source.Span
			if !ok || decodeErr != nil || json.Unmarshal(data, &span) != nil {
				return nil, nil, fmt.Errorf("invalid source mapping marker")
			}
			start := bytes.LastIndexByte(input[:tok.position.Offset], '\n') + 1
			end := tok.position.Offset + tok.length
			if next := bytes.IndexByte(input[end:], '\n'); next >= 0 {
				end += next + 1
			} else {
				end = len(input)
			}
			if string(bytes.TrimSpace(input[start:end])) != tok.literal || start < copied {
				return nil, nil, fmt.Errorf("source mapping marker is not a standalone statement")
			}
			clean.Write(input[copied:start])
			copied = end
			switch kind {
			case "start":
				stack = append(stack, span)
			case "end":
				if len(stack) == 0 || stack[len(stack)-1] != span {
					return nil, nil, fmt.Errorf("unbalanced source mapping markers")
				}
				stack = stack[:len(stack)-1]
			default:
				return nil, nil, fmt.Errorf("unknown source mapping marker")
			}
			continue
		}
		if len(stack) > 0 {
			tok.origin = stack[len(stack)-1]
		}
		real = append(real, tok)
	}
	if len(stack) != 0 {
		return nil, nil, fmt.Errorf("unterminated source mapping marker")
	}
	clean.Write(input[copied:])
	// Marker statements occupy complete lines in already formatted output.
	// Keep that formatting intact: format.Source would sort import groups that
	// the position-free original AST leaves in source order.
	generated := clean.Bytes()
	finalTokens, err := scanMappingTokens(generated)
	if err != nil {
		return nil, nil, err
	}
	if len(real) != len(finalTokens) {
		return nil, nil, fmt.Errorf("source mapping changed generated token count")
	}
	mappings := make([]SourceMapping, 0)
	previousMapped := false
	for i, tok := range finalTokens {
		if tok.kind != real[i].kind || tok.literal != real[i].literal {
			return nil, nil, fmt.Errorf("source mapping changed generated tokens")
		}
		origin := real[i].origin
		if origin.Path == "" {
			previousMapped = false
			continue
		}
		end := tok.position
		end.Offset += tok.length
		text := generated[tok.position.Offset:end.Offset]
		if lines := bytes.Count(text, []byte{'\n'}); lines != 0 {
			end.Line += lines
			end.Column = len(text) - bytes.LastIndexByte(text, '\n')
		} else {
			end.Column += tok.length
		}
		if previousMapped && mappings[len(mappings)-1].Source == origin {
			mappings[len(mappings)-1].Generated.End = end
		} else {
			mappings = append(mappings, SourceMapping{Generated: source.Span{Path: "generated.go", Start: tok.position, End: end}, Source: origin})
		}
		previousMapped = true
	}
	return generated, mappings, nil
}
