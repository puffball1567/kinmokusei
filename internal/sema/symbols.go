package sema

import (
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type functionSymbol struct {
	parameters         []Type
	typeParameters     []Type
	typeParameterScope map[string]Type
	variadic           bool
	result             Type
	span               source.Span
	declarationSpan    source.Span
}

type valueSymbol struct {
	typeInfo              Type
	initializingArrow     bool
	declaredType          Type
	flowInvalidated       source.Span
	flowInvalidationCause string
	flowEscaped           bool
	constant              bool
	declarationSpan       source.Span
	declaration           *ast.VariableDecl
	multiDeclaration      *ast.MultiVariableDecl
	multiIndex            int
	rangeBinding          *ast.RangeBinding
	selectCase            *ast.SelectCase
	selectIndex           int
	typeSwitchCase        *ast.TypeSwitchCase
	catchClause           *ast.CatchClause
	taskState             uint8
}

const (
	taskNotTracked uint8 = iota
	taskPending
	taskConsumed
	taskMaybeConsumed
)

type fieldSymbol struct {
	typeInfo        Type
	visibility      ast.Visibility
	goName          string
	declarationSpan source.Span
	declaringClass  string
}

type methodSymbol struct {
	goInterfaceMethod bool
	typeInfo          Type
	visibility        ast.Visibility
	static            bool
	pointerReceiver   bool
	goName            string
	declarationSpan   source.Span
	declaringClass    string
	virtual           bool
	final             bool
	virtualOwner      string
}

type classSymbol struct {
	fields              map[string]fieldSymbol
	methods             map[string]methodSymbol
	constructor         []Type
	constructorVariadic bool
	implements          map[string]bool
	implementedTypes    []Type
	goImplements        []gotypes.Type
	typeParameters      []Type
	typeParamScope      map[string]Type
	declarationSpan     source.Span
	base                string
	baseType            Type
	ancestors           []string
	ancestorTypes       []Type
	final               bool
	goNamed             *gotypes.Named
}

type structSymbol struct {
	fields          map[string]fieldSymbol
	methods         map[string]methodSymbol
	typeInfo        Type
	goNamed         *gotypes.Named
	typeParameters  []Type
	typeParamScope  map[string]Type
	declarationSpan source.Span
}

type interfaceSymbol struct {
	constraintDeclaration *ast.InterfaceDecl
	constraintResolving   bool
	constraintTermTypes   []Type
	methods               map[string]methodSymbol
	bases                 []Type
	typeParameters        []Type
	typeParamScope        map[string]Type
	declarationSpan       source.Span
	goNamed               *gotypes.Named
	constraint            bool
}

type nativeTypeSymbol struct {
	declaration    *ast.TypeDecl
	typeInfo       Type
	underlying     Type
	goNamed        *gotypes.Named
	methods        map[string]methodSymbol
	typeParameters []Type
	typeParamScope map[string]Type
	state          uint8
}

type enumSymbol struct {
	declaration *ast.EnumDecl
	members     map[string]*ast.EnumMember
}

type goPackageSymbol struct {
	path        string
	declaration *ast.ImportDecl
	packageInfo *gotypes.Package
}
