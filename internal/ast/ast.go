package ast

import "ontama.local/ontama/internal/source"

type Node interface{ GetSpan() source.Span }

type Program struct {
	Imports        []ImportDecl
	Declarations   []Declaration
	UsesTasks      bool
	UsesExceptions bool
}

type ImportDecl struct {
	Names         []string
	NameSpans     []source.Span
	Go            bool
	Alias         string
	AliasSpan     source.Span
	ResolvedAlias string
	Used          bool
	Path          string
	ResolvedPath  string
	PathSpan      source.Span
	Span          source.Span
}

func (d ImportDecl) GetSpan() source.Span { return d.Span }

type Declaration interface {
	Node
	declaration()
}
type Statement interface {
	Node
	statement()
}
type Expression interface {
	Node
	expression()
}

type TypeRef struct {
	Name                 string
	NameSpan             source.Span
	Qualifier            string
	QualifierSpan        source.Span
	ResolvedDeclaration  source.Span
	QualifierDeclaration source.Span
	GenericArguments     []TypeRef
	Element              *TypeRef
	FixedLength          *int64
	Pointee              *TypeRef
	Parameters           []TypeRef
	Return               *TypeRef
	ObjectFields         []ObjectTypeField
	Object               bool
	GoStruct             bool
	Struct               bool
	Interface            bool
	TypeParameter        bool
	NativeNamed          bool
	Go                   bool
	Nullable             bool
	Span                 source.Span
}

type ObjectTypeField struct {
	Name       string
	JSONName   string
	GoTag      string
	GoEmbedded bool
	Type       TypeRef
	Span       source.Span
}

func (t TypeRef) GetSpan() source.Span { return t.Span }
func (t TypeRef) IsFunction() bool     { return t.Return != nil }
func (t TypeRef) IsArray() bool        { return t.Element != nil }
func (t TypeRef) IsSlice() bool        { return t.Element != nil && t.FixedLength == nil }
func (t TypeRef) IsFixedArray() bool   { return t.Element != nil && t.FixedLength != nil }
func (t TypeRef) IsPointer() bool      { return t.Pointee != nil }
func (t TypeRef) IsObject() bool       { return t.Object }
func (t TypeRef) IsGoStruct() bool     { return t.GoStruct }
func (t TypeRef) IsSpecified() bool {
	return t.Name != "" || t.Qualifier != "" || t.IsFunction() || t.IsArray() || t.IsPointer() || t.IsObject() || t.IsGoStruct()
}

type Parameter struct {
	Name       string
	Type       TypeRef
	Visibility Visibility
	IsField    bool
	Span       source.Span
}

type TypeParameter struct {
	Name     string
	NameSpan source.Span
	Span     source.Span
}

type Visibility int

const (
	Private Visibility = iota
	Protected
	Public
)

type FunctionDecl struct {
	Name           string
	NameSpan       source.Span
	TypeParameters []TypeParameter
	Parameters     []Parameter
	ReturnType     TypeRef
	Body           *BlockStmt
	CABIExport     bool
	CABISymbol     string
	CABIExportSpan source.Span
	CABISymbolSpan source.Span
	Span           source.Span
}

func (*FunctionDecl) declaration()           {}
func (d *FunctionDecl) GetSpan() source.Span { return d.Span }

type FieldDecl struct {
	Name       string
	NameSpan   source.Span
	Type       TypeRef
	Visibility Visibility
	GoName     string
	Span       source.Span
}

type ConstructorDecl struct {
	Parameters []Parameter
	Body       *BlockStmt
	Span       source.Span
}

type MethodDecl struct {
	Name           string
	NameSpan       source.Span
	TypeParameters []TypeParameter
	Parameters     []Parameter
	ReturnType     TypeRef
	Body           *BlockStmt
	Visibility     Visibility
	Static         bool
	Virtual        bool
	Override       bool
	Final          bool
	VirtualOwner   string
	// PointerReceiver is used by native value structs. Class instance methods
	// always retain their existing implicit pointer receiver.
	PointerReceiver bool
	// External receiver methods are top-level declarations such as
	// `function read(this: Point): int`. Nested methods leave these empty.
	External         bool
	ReceiverName     string
	ReceiverNameSpan source.Span
	ReceiverType     TypeRef
	GoName           string
	Span             source.Span
}

func (*MethodDecl) declaration()           {}
func (d *MethodDecl) GetSpan() source.Span { return d.Span }

type ClassDecl struct {
	Name          string
	NameSpan      source.Span
	Final         bool
	Base          *TypeRef
	Implements    []TypeRef
	Fields        []FieldDecl
	Constructor   *ConstructorDecl
	Methods       []*MethodDecl
	VirtualOwners []string
	Ancestors     []string
	Descendants   []string
	HierarchyRoot string
	Span          source.Span
}

func (*ClassDecl) declaration()           {}
func (d *ClassDecl) GetSpan() source.Span { return d.Span }

// StructDecl is a nominal Go-style value type. Unlike ClassDecl it has no
// implicit reference identity or constructor allocation.
type StructDecl struct {
	Name           string
	NameSpan       source.Span
	TypeParameters []TypeParameter
	Fields         []FieldDecl
	Methods        []*MethodDecl
	Span           source.Span
}

func (*StructDecl) declaration()           {}
func (d *StructDecl) GetSpan() source.Span { return d.Span }

// TypeDecl represents either a transparent alias (`alias Name = T`) or a
// nominal Go defined type (`type Name = distinct T`).
type TypeDecl struct {
	Name       string
	NameSpan   source.Span
	Underlying TypeRef
	Alias      bool
	Span       source.Span
}

func (*TypeDecl) declaration()           {}
func (d *TypeDecl) GetSpan() source.Span { return d.Span }

type InterfaceMethod struct {
	Name       string
	NameSpan   source.Span
	Parameters []Parameter
	ReturnType TypeRef
	GoName     string
	Span       source.Span
}

type InterfaceDecl struct {
	Name     string
	NameSpan source.Span
	Methods  []InterfaceMethod
	Span     source.Span
}

func (*InterfaceDecl) declaration()           {}
func (d *InterfaceDecl) GetSpan() source.Span { return d.Span }

type VariableDecl struct {
	Constant     bool
	Name         string
	NameSpan     source.Span
	Type         TypeRef
	ResolvedType TypeRef
	Value        Expression
	Used         bool
	Span         source.Span
}

func (*VariableDecl) declaration()           {}
func (*VariableDecl) statement()             {}
func (d *VariableDecl) GetSpan() source.Span { return d.Span }

type Binding struct {
	Name                string
	Used                bool
	ResolvedDeclaration source.Span
	ResolvedType        TypeRef
	Span                source.Span
}

type MultiVariableDecl struct {
	Constant bool
	Bindings []Binding
	Value    Expression
	Span     source.Span
}

func (*MultiVariableDecl) statement()             {}
func (d *MultiVariableDecl) GetSpan() source.Span { return d.Span }

type BlockStmt struct {
	Statements []Statement
	Span       source.Span
}

func (*BlockStmt) statement()             {}
func (s *BlockStmt) GetSpan() source.Span { return s.Span }

type ReturnStmt struct {
	Value      Expression
	ResultKind ResultReturnKind
	ResultType TypeRef
	CrossesTry bool
	Span       source.Span
}

type ResultReturnKind int

const (
	NormalReturn ResultReturnKind = iota
	ResultSuccessReturn
	ResultFailureReturn
	ResultForwardReturn
)

func (*ReturnStmt) statement()             {}
func (s *ReturnStmt) GetSpan() source.Span { return s.Span }

type ThrowStmt struct {
	Value         Expression
	Bare          bool
	RethrowOffset int
	Span          source.Span
}

func (*ThrowStmt) statement()             {}
func (s *ThrowStmt) GetSpan() source.Span { return s.Span }

type TryStmt struct {
	Body          *BlockStmt
	Catches       []*CatchClause
	FinallyBody   *BlockStmt
	HandlesReturn bool
	ReturnType    TypeRef
	Terminal      bool
	Span          source.Span
}

type CatchClause struct {
	Name            string
	NameSpan        source.Span
	Type            TypeRef
	Body            *BlockStmt
	Used            bool
	MatchingClasses []string
}

func (*TryStmt) statement()             {}
func (s *TryStmt) GetSpan() source.Span { return s.Span }

type IfStmt struct {
	Condition Expression
	Then      *BlockStmt
	Else      Statement
	Span      source.Span
}

func (*IfStmt) statement()             {}
func (s *IfStmt) GetSpan() source.Span { return s.Span }

type ExpressionStmt struct {
	Value Expression
	Span  source.Span
}

func (*ExpressionStmt) statement()             {}
func (s *ExpressionStmt) GetSpan() source.Span { return s.Span }

type AssignmentStmt struct {
	Target   Expression
	Operator string
	Value    Expression
	Span     source.Span
}

func (*AssignmentStmt) statement()             {}
func (s *AssignmentStmt) GetSpan() source.Span { return s.Span }

type IncDecStmt struct {
	Target   Expression
	Operator string
	Span     source.Span
}

func (*IncDecStmt) statement()             {}
func (s *IncDecStmt) GetSpan() source.Span { return s.Span }

type MultiAssignmentStmt struct {
	Bindings []Binding
	Value    Expression
	Span     source.Span
}

func (*MultiAssignmentStmt) statement()             {}
func (s *MultiAssignmentStmt) GetSpan() source.Span { return s.Span }

type WhileStmt struct {
	Condition       Expression
	GuaranteedEntry bool
	Body            *BlockStmt
	Span            source.Span
}

func (*WhileStmt) statement()             {}
func (s *WhileStmt) GetSpan() source.Span { return s.Span }

type ForStmt struct {
	Initializer     Statement
	Condition       Expression
	GuaranteedEntry bool
	Post            Statement
	Body            *BlockStmt
	Span            source.Span
}

func (*ForStmt) statement()             {}
func (s *ForStmt) GetSpan() source.Span { return s.Span }

type RangeBinding struct {
	Name         string
	NameSpan     source.Span
	Type         TypeRef
	ResolvedType TypeRef
	Used         bool
	Assigned     bool
}

type ForRangeKind int

const (
	UnknownRange ForRangeKind = iota
	ChannelRange
	CollectionRange
)

// ForRangeStmt binds a value, or a key/index and value pair, in a fresh loop
// scope. Kind is resolved semantically so lowering preserves Go range rules.
type ForRangeStmt struct {
	Constant           bool
	Bindings           []RangeBinding
	Source             Expression
	Kind               ForRangeKind
	GuaranteedNonEmpty bool
	Body               *BlockStmt
	Span               source.Span
}

func (*ForRangeStmt) statement()             {}
func (s *ForRangeStmt) GetSpan() source.Span { return s.Span }

type SelectCaseKind int

const (
	SelectReceive SelectCaseKind = iota
	SelectSend
	SelectDefault
)

// SelectCase represents exactly one Go select communication. Receive cases
// either discard, declare Bindings, or assign Targets. Each Body is a distinct
// case-local scope and never falls through to another case.
type SelectCase struct {
	Kind     SelectCaseKind
	Constant bool
	Declare  bool
	Bindings []Binding
	Targets  []Expression
	Channel  Expression
	Value    Expression
	Body     *BlockStmt
	Span     source.Span
}

func (c *SelectCase) GetSpan() source.Span { return c.Span }

type SelectStmt struct {
	Cases []SelectCase
	Span  source.Span
}

func (*SelectStmt) statement()             {}
func (s *SelectStmt) GetSpan() source.Span { return s.Span }

// ValueSwitchCase contains one or more expressions compared against the
// switch value. Bodies are case-local scopes and never fall through.
type ValueSwitchCase struct {
	Values  []Expression
	Default bool
	Body    *BlockStmt
	Span    source.Span
}

func (c *ValueSwitchCase) GetSpan() source.Span { return c.Span }

// ValueSwitchStmt maps directly to a Go expression switch. The subject is
// evaluated once and case expressions are considered from left to right.
type ValueSwitchStmt struct {
	Value Expression
	Cases []ValueSwitchCase
	Span  source.Span
}

func (*ValueSwitchStmt) statement()             {}
func (s *ValueSwitchStmt) GetSpan() source.Span { return s.Span }

type TypeSwitchCase struct {
	Constant bool
	Name     string
	NameSpan source.Span
	Type     TypeRef
	Nil      bool
	Default  bool
	Body     *BlockStmt
	Used     bool
	Span     source.Span
}

func (c *TypeSwitchCase) GetSpan() source.Span { return c.Span }

// TypeSwitchStmt dispatches a Go interface value without changing Go's
// dynamic-type, typed-nil, ordering, or single-evaluation semantics.
type TypeSwitchStmt struct {
	Value Expression
	Cases []TypeSwitchCase
	Span  source.Span
}

func (*TypeSwitchStmt) statement()             {}
func (s *TypeSwitchStmt) GetSpan() source.Span { return s.Span }

type BranchKind int

const (
	BreakBranch BranchKind = iota
	ContinueBranch
)

type BranchStmt struct {
	Kind BranchKind
	Span source.Span
}

func (*BranchStmt) statement()             {}
func (s *BranchStmt) GetSpan() source.Span { return s.Span }

type CallControlKind int

const (
	DeferCall CallControlKind = iota
	GoCall
)

// CallControlStmt maps directly to Go's defer and go statements. Value must
// resolve to a function or method call; conversions are deliberately rejected.
type CallControlStmt struct {
	Kind  CallControlKind
	Value Expression
	Span  source.Span
}

func (*CallControlStmt) statement()             {}
func (s *CallControlStmt) GetSpan() source.Span { return s.Span }

// DetachStmt explicitly relinquishes a structured task result. Panics remain
// observable and are re-raised by the generated detached waiter.
type DetachStmt struct {
	Value      Expression
	ValueType  TypeRef
	ResultTask bool
	Void       bool
	Span       source.Span
}

func (*DetachStmt) statement()             {}
func (s *DetachStmt) GetSpan() source.Span { return s.Span }

// ChannelSendStmt is the low-level Go channel send operation. Direction and
// element assignability are checked before it is lowered to `channel <- value`.
type ChannelSendStmt struct {
	Channel Expression
	Value   Expression
	Span    source.Span
}

func (*ChannelSendStmt) statement()             {}
func (s *ChannelSendStmt) GetSpan() source.Span { return s.Span }

type IdentifierExpr struct {
	Name                string
	ResolvedDeclaration source.Span
	Span                source.Span
}

func (*IdentifierExpr) expression()            {}
func (e *IdentifierExpr) GetSpan() source.Span { return e.Span }

type LiteralKind int

const (
	IntegerLiteral LiteralKind = iota
	FloatLiteral
	StringLiteral
	BooleanLiteral
	NilLiteral
	NullLiteral
)

type LiteralExpr struct {
	Kind LiteralKind
	Text string
	Span source.Span
}

func (*LiteralExpr) expression()            {}
func (e *LiteralExpr) GetSpan() source.Span { return e.Span }

type UnaryExpr struct {
	Operator string
	Operand  Expression
	Span     source.Span
}

func (*UnaryExpr) expression()            {}
func (e *UnaryExpr) GetSpan() source.Span { return e.Span }

type BinaryExpr struct {
	Left     Expression
	Operator string
	Right    Expression
	Span     source.Span
}

func (*BinaryExpr) expression()            {}
func (e *BinaryExpr) GetSpan() source.Span { return e.Span }

// GoTypeAssertionExpr keeps the two failure modes syntactically distinct.
// Checked assertions produce (value, boolean); unchecked assertions panic on
// failure with the same runtime semantics as a Go type assertion.
type GoTypeAssertionExpr struct {
	Value         Expression
	Type          TypeRef
	Checked       bool
	ClassDowncast bool
	SourceClass   string
	Span          source.Span
}

func (*GoTypeAssertionExpr) expression()            {}
func (e *GoTypeAssertionExpr) GetSpan() source.Span { return e.Span }

// PropagateExpr explicitly unwraps an OnsenTamago Result or a Go (T, error)
// operation and returns the error from the enclosing Result function.
type PropagateExpr struct {
	Value      Expression
	ValueType  TypeRef
	ResultType TypeRef
	ErrorName  string
	Span       source.Span
}

func (*PropagateExpr) expression()            {}
func (e *PropagateExpr) GetSpan() source.Span { return e.Span }

type TaskStartExpr struct {
	Call       *CallExpr
	ValueType  TypeRef
	ResultTask bool
	Void       bool
	Span       source.Span
}

func (*TaskStartExpr) expression()            {}
func (e *TaskStartExpr) GetSpan() source.Span { return e.Span }

type AwaitExpr struct {
	Value      Expression
	ValueType  TypeRef
	ResultTask bool
	Void       bool
	Span       source.Span
}

func (*AwaitExpr) expression()            {}
func (e *AwaitExpr) GetSpan() source.Span { return e.Span }

type CallExpr struct {
	Callee           Expression
	TypeArguments    []TypeRef
	Arguments        []Expression
	Expanded         bool
	Conversion       bool
	Builtin          BuiltinCallKind
	Signature        *CallableSignature
	SuperConstructor bool
	SuperBase        string
	Span             source.Span
}

type CallableSignature struct {
	ParameterNames []string
	ParameterTypes []string
	Result         string
	Variadic       bool
}

func (*CallExpr) expression()            {}
func (e *CallExpr) GetSpan() source.Span { return e.Span }

type BuiltinCallKind int

const (
	NotBuiltinCall BuiltinCallKind = iota
	MakeGoChannelCall
	CloseGoChannelCall
	LenCall
	CapCall
	AppendCall
	CopyCall
	DeleteCall
	ClearCall
	MinCall
	MaxCall
	MakeSliceCall
	MakeMapCall
	CopyArrayCall
	ViewArrayCall
	UnsafeSizeofCall
	UnsafeAlignofCall
	UnsafeOffsetofCall
	UnsafeAddCall
	UnsafeSliceCall
	UnsafeSliceDataCall
	UnsafeStringCall
	UnsafeStringDataCall
	ResultOKCall
	ResultFailCall
)

type ArrowExpr struct {
	Parameters         []Parameter
	ReturnType         *TypeRef
	ExpressionBody     Expression
	BlockBody          *BlockStmt
	ResolvedReturnType TypeRef
	Span               source.Span
}

func (*ArrowExpr) expression()            {}
func (e *ArrowExpr) GetSpan() source.Span { return e.Span }

type ArrayLiteralExpr struct {
	Elements            []Expression
	ResolvedElementType TypeRef
	Fixed               bool
	ResolvedLength      int64
	Span                source.Span
}

func (*ArrayLiteralExpr) expression()            {}
func (e *ArrayLiteralExpr) GetSpan() source.Span { return e.Span }

type ObjectField struct {
	Name                string
	NameSpan            source.Span
	ResolvedDeclaration source.Span
	Value               Expression
	Span                source.Span
}

type ObjectLiteralExpr struct {
	Fields             []ObjectField
	ResolvedFieldTypes []TypeRef
	ResolvedFieldNames []string
	Span               source.Span
}

func (*ObjectLiteralExpr) expression()            {}
func (e *ObjectLiteralExpr) GetSpan() source.Span { return e.Span }

type GoCompositeLiteralExpr struct {
	Type               TypeRef
	Fields             []ObjectField
	ResolvedFieldNames []string
	Span               source.Span
}

func (*GoCompositeLiteralExpr) expression()            {}
func (e *GoCompositeLiteralExpr) GetSpan() source.Span { return e.Span }

type MemberExpr struct {
	Object              Expression
	Name                string
	NameSpan            source.Span
	ResolvedDeclaration source.Span
	ResolvedName        string
	Static              bool
	Constant            bool
	Addressable         bool
	GoField             bool
	GoFieldViaPointer   bool
	Go                  bool
	Super               bool
	SuperBase           string
	VirtualDispatch     bool
	VirtualOwner        string
	Span                source.Span
}

func (*MemberExpr) expression()            {}
func (e *MemberExpr) GetSpan() source.Span { return e.Span }

type IndexExpr struct {
	Object      Expression
	Index       Expression
	Addressable bool
	Assignable  bool
	Span        source.Span
}

func (*IndexExpr) expression()            {}
func (e *IndexExpr) GetSpan() source.Span { return e.Span }

type SliceExpr struct {
	Object Expression
	Low    Expression
	High   Expression
	Max    Expression
	Full   bool
	Span   source.Span
}

func (*SliceExpr) expression()            {}
func (e *SliceExpr) GetSpan() source.Span { return e.Span }

type NewExpr struct {
	ClassName           string
	ClassNameSpan       source.Span
	ResolvedDeclaration source.Span
	Arguments           []Expression
	Span                source.Span
}

func (*NewExpr) expression()            {}
func (e *NewExpr) GetSpan() source.Span { return e.Span }

// ClassUpcastExpr is inserted by semantic analysis when a derived reference is
// consumed as one of its base types. Code generation uses a nil-preserving
// helper rather than selecting an embedded base field directly.
type ClassUpcastExpr struct {
	Value       Expression
	SourceClass string
	TargetClass string
	Span        source.Span
}

func (*ClassUpcastExpr) expression()            {}
func (e *ClassUpcastExpr) GetSpan() source.Span { return e.Span }
