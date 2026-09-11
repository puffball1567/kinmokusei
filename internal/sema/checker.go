package sema

import (
	"fmt"
	"go/importer"
	gotypes "go/types"
	"strings"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/diagnostic"
	"github.com/puffball1567/kinmokusei/internal/source"
)

type Checker struct {
	callableControlState
	diagnostics                []diagnostic.Diagnostic
	functions                  map[string]functionSymbol
	globals                    map[string]valueSymbol
	scopes                     []map[string]valueSymbol
	classes                    map[string]*classSymbol
	structs                    map[string]*structSymbol
	interfaces                 map[string]*interfaceSymbol
	nativeTypes                map[string]*nativeTypeSymbol
	enums                      map[string]*enumSymbol
	currentClass               string
	allowed                    map[string]map[string]bool
	goPackages                 map[string]map[string]*goPackageSymbol
	goNamedImports             map[string]map[string]goNamedImport
	goImporter                 gotypes.Importer
	allowUnsafeGo              bool
	inConstructor              bool
	inFieldInitializer         bool
	callableScopeBases         []int
	capturedWrites             []map[source.Span]source.Span
	loopFlowContexts           []loopFlowContext
	breakFlowContexts          []breakFlowContext
	suppressFlowEffects        int
	memberFlow                 map[memberFlowKey]memberFlowState
	memberTypes                map[memberFlowKey]Type
	usesTasks                  bool
	usesExceptions             bool
	nativeTypeIndirectionDepth int
	taskOperandDepth           int
	directCallCallee           ast.Expression
	typeParameterScopes        []map[string]Type
	deferredParameterBounds    map[*gotypes.TypeParam]bool
	pendingBoundInstances      *[]boundInstance
	nativeConstraintsReady     bool
	parameterRangeShapes       map[*gotypes.TypeParam]Type
	functionTypeParameters     map[*ast.FunctionDecl]map[string]Type
	receiverTypeParameters     map[*ast.MethodDecl]map[string]Type
	methodTypeParameters       map[*ast.MethodDecl]map[string]Type
	validFallthrough           map[*ast.BranchStmt]bool
	capturedMemberWrites       []source.Span
	capturedMemberRoots        []map[source.Span]bool
	structGoTypesFinalized     bool
	numericValues              map[ast.Expression]gotypes.TypeAndValue
	globalDependencyOwner      string
	globalDependencies         map[string]map[string]bool
}

type GoInteropPolicy struct {
	AllowUnsafe bool
}

func Check(program *ast.Program) []diagnostic.Diagnostic {
	return CheckScoped(program, nil)
}

func CheckScoped(program *ast.Program, allowed map[string]map[string]bool) []diagnostic.Diagnostic {
	return CheckScopedWithGoImporter(program, allowed, importer.Default())
}

func CheckScopedWithGoImporter(program *ast.Program, allowed map[string]map[string]bool, goImporter gotypes.Importer) []diagnostic.Diagnostic {
	return CheckScopedWithGoImporterAndPolicy(program, allowed, goImporter, GoInteropPolicy{})
}

func CheckScopedWithGoImporterAndPolicy(program *ast.Program, allowed map[string]map[string]bool, goImporter gotypes.Importer, policy GoInteropPolicy) []diagnostic.Diagnostic {
	if goImporter == nil {
		goImporter = importer.Default()
	}
	c := &Checker{
		functions: map[string]functionSymbol{}, globals: map[string]valueSymbol{},
		classes: map[string]*classSymbol{}, structs: map[string]*structSymbol{}, interfaces: map[string]*interfaceSymbol{}, nativeTypes: map[string]*nativeTypeSymbol{}, enums: map[string]*enumSymbol{}, allowed: allowed,
		goPackages: map[string]map[string]*goPackageSymbol{}, goImporter: goImporter, allowUnsafeGo: policy.AllowUnsafe,
		goNamedImports: map[string]map[string]goNamedImport{},
		memberFlow:     map[memberFlowKey]memberFlowState{}, memberTypes: map[memberFlowKey]Type{},
		functionTypeParameters: map[*ast.FunctionDecl]map[string]Type{},
		receiverTypeParameters: map[*ast.MethodDecl]map[string]Type{},
		methodTypeParameters:   map[*ast.MethodDecl]map[string]Type{},
		validFallthrough:       map[*ast.BranchStmt]bool{},
	}
	c.installExceptionBuiltin()
	c.declareGoPackages(program)
	c.predeclareInterfaceNames(program)
	c.predeclareNamedTypes(program)
	c.declareNativeConstraints(program)
	c.finalizeDeferredTypeParameterConstraints(program)
	c.declareNativeTypes(program)
	c.declareStructs(program)
	c.finalizeNativeStructGoTypes(program)
	c.structGoTypesFinalized = true
	c.declareNativeTypes(program)
	c.declareInterfaces(program)
	c.declareClasses(program)
	c.declareTopLevel(program)
	c.declareReceiverMethods(program)
	c.validateStructValueCycles(program)
	for _, decl := range c.globalCheckOrder(program) {
		c.checkGlobalBinding(decl)
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.EnumDecl); ok {
			c.checkEnum(decl)
		}
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.ClassDecl); ok {
			c.checkClass(decl)
		}
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.StructDecl); ok {
			c.checkStruct(decl)
		}
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.MethodDecl); ok {
			c.pushTypeParameterScope(c.receiverTypeParameters[decl])
			receiver := decl.ReceiverType
			if receiver.IsPointer() && receiver.Pointee != nil {
				receiver = *receiver.Pointee
			}
			if _, exists := c.nativeTypes[receiver.Name]; exists {
				c.checkNativeTypeMethod(decl, decl.ReceiverName, receiver.Name)
			} else {
				c.checkStructMethod(decl, decl.ReceiverName, receiver.Name)
			}
			c.popTypeParameterScope()
		}
	}
	for _, decl := range program.Declarations {
		if decl, ok := decl.(*ast.FunctionDecl); ok {
			c.checkFunction(decl)
		}
	}
	c.checkGlobalInitializationCycles(program)
	c.checkSourceExports(program)
	c.checkCABIExports(program)
	c.checkGeneratedNames(program)
	c.markResolvedTypeRefs(program)
	program.UsesTasks = c.usesTasks
	program.UsesExceptions = c.usesExceptions
	return c.diagnostics
}

func (c *Checker) checkBlock(block *ast.BlockStmt, nested bool) {
	if nested {
		c.pushScope()
		defer c.popScope()
	}
	terminated := false
	var reachableFlow *nullableFlowSnapshot
	for _, stmt := range block.Statements {
		if _, labeled := stmt.(*ast.LabeledStmt); labeled && terminated {
			if reachableFlow != nil {
				c.suppressFlowEffects--
				c.restoreNullableFlow(*reachableFlow)
				reachableFlow = nil
			}
			terminated = false
		}
		if !terminated {
			c.checkStatement(stmt)
			terminated = statementDefinitelyStopsBlock(stmt)
			continue
		}
		if reachableFlow == nil {
			snapshot := c.snapshotNullableFlow()
			reachableFlow = &snapshot
			c.suppressFlowEffects++
		}
		c.checkStatement(stmt)
	}
	if reachableFlow != nil {
		c.suppressFlowEffects--
		c.restoreNullableFlow(*reachableFlow)
	}
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	switch stmt := stmt.(type) {
	case *ast.LabeledStmt:
		c.invalidateControlTransferFlow(stmt.Span)
		c.checkStatement(stmt.Statement)
	case *ast.VariableDecl:
		declared := Type{Kind: Invalid, Name: "<inferred>"}
		if stmt.Type.IsSpecified() {
			declared = c.resolveType(stmt.Type)
		}
		var value Type
		if propagated, ok := stmt.Value.(*ast.PropagateExpr); ok {
			value = c.checkPropagateExpression(propagated)
		} else {
			value = c.checkExpressionExpectedSlot(&stmt.Value, declared)
		}
		if !stmt.Type.IsSpecified() {
			declared = c.inferredVariableType(value, stmt.Value.GetSpan())
			if !stmt.Constant || !numericInitializerEmitsConstant(stmt.Value) {
				c.checkNumericMaterialization(stmt.Value, declared)
			}
		}
		if declared.Kind == Void {
			c.report(stmt.Type.Span, "variables cannot have type void")
		}
		c.rejectResultValueType(declared, stmt.Type.Span, "variables")
		c.requireAssignable(declared, value, stmt.Value.GetSpan())
		stmt.ResolvedType = typeRefFromType(declared, stmt.Span)
		c.declareLocal(stmt.Name, declared, stmt.Constant, stmt, stmt.Span)
		c.updateIdentifierFlow(stmt.Name, stmt.NameSpan, value)
	case *ast.MultiVariableDecl:
		c.checkMultiVariableDeclaration(stmt)
	case *ast.ReturnStmt:
		if c.exceptionDepth != 0 {
			stmt.CrossesTry = true
		}
		if c.inConstructor {
			c.report(stmt.Span, "constructors cannot return early; use conditional initialization and let the constructor complete")
		}
		if c.arrowReturns != nil {
			c.collectArrowReturn(stmt)
			c.reportPendingTasksBeforeExit()
			return
		}
		if c.result.Kind == Result {
			c.checkResultReturn(stmt)
			c.reportPendingTasksBeforeExit()
			return
		}
		if stmt.Value == nil {
			if c.result.Kind != Void {
				c.report(stmt.Span, fmt.Sprintf("expected return value of type %s", c.result.Name))
			}
			c.reportPendingTasksBeforeExit()
			return
		}
		value := c.checkExpressionExpectedSlot(&stmt.Value, c.result)
		if c.result.Kind == Void {
			c.report(stmt.Value.GetSpan(), "void function cannot return a value")
		} else {
			c.requireAssignable(c.result, value, stmt.Value.GetSpan())
		}
		c.reportPendingTasksBeforeExit()
	case *ast.ThrowStmt:
		c.usesExceptions = true
		if stmt.Bare {
			if len(c.catchTargets) == 0 {
				c.report(stmt.Span, "bare throw may only be used inside a catch block")
			} else {
				stmt.RethrowOffset = c.catchTargets[len(c.catchTargets)-1]
			}
			c.reportPendingTasksBeforeExit()
			return
		}
		value := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
		c.requireAssignable(builtins["error"], value, stmt.Value.GetSpan())
		c.reportPendingTasksBeforeExit()
	case *ast.TryStmt:
		c.checkTryStatement(stmt)
	case *ast.IfStmt:
		condition := c.checkExpression(stmt.Condition)
		if condition.Kind != Invalid && condition.Kind != Boolean {
			c.report(stmt.Condition.GetSpan(), fmt.Sprintf("if condition must be boolean, got %s", condition.Name))
		}
		narrowing, hasNarrowing := c.nullableConditionNarrowing(stmt.Condition)
		entryFlow := c.snapshotNullableFlow()

		c.restoreNullableFlow(entryFlow)
		if hasNarrowing && narrowing.nonNullWhenTrue {
			c.applyNarrowing(narrowing)
		}
		c.checkBlock(stmt.Then, true)
		thenFlow := c.snapshotNullableFlow()

		c.restoreNullableFlow(entryFlow)
		if hasNarrowing && !narrowing.nonNullWhenTrue {
			c.applyNarrowing(narrowing)
		}
		if stmt.Else != nil {
			c.checkStatementBranch(stmt.Else)
		}
		elseFlow := c.snapshotNullableFlow()

		continuing := make([]nullableFlowSnapshot, 0, 2)
		if !statementDefinitelyStopsBlock(stmt.Then) {
			continuing = append(continuing, thenFlow)
		}
		if stmt.Else == nil || !statementDefinitelyStopsBlock(stmt.Else) {
			continuing = append(continuing, elseFlow)
		}
		c.restoreNullableFlow(c.mergeNullableFlow(entryFlow, continuing...))
	case *ast.BlockStmt:
		c.checkBlock(stmt, true)
	case *ast.ExpressionStmt:
		if propagated, ok := stmt.Value.(*ast.PropagateExpr); ok {
			value := c.checkPropagateExpression(propagated)
			if value.Kind != Invalid && value.Kind != Void {
				c.report(stmt.Span, fmt.Sprintf("propagated result of type %s must be bound to a variable", value.String()))
			}
			return
		}
		value := c.checkExpression(stmt.Value)
		if value.Kind == Result {
			c.report(stmt.Span, "Result values must be consumed with ?, explicitly split, or returned")
		}
		if !isAllowedExpressionStatement(stmt.Value) {
			c.report(stmt.Span, "only function calls and Go channel receives may be used as expression statements")
		}
	case *ast.AssignmentStmt:
		target := c.checkAssignmentTarget(stmt.Target)
		if target.Kind == Task {
			c.report(stmt.Target.GetSpan(), "Task bindings cannot be reassigned")
		}
		if stmt.Operator == "" || stmt.Operator == "=" {
			value := c.checkExpressionExpectedSlot(&stmt.Value, target)
			c.requireAssignable(target, value, stmt.Value.GetSpan())
			c.updateAssignmentFlow(stmt.Target, value)
		} else {
			c.markAssignmentTargetRead(stmt.Target)
			value := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
			operator := strings.TrimSuffix(stmt.Operator, "=")
			result := c.checkBinaryOperands(&ast.BinaryExpr{Left: stmt.Target, Operator: operator, Right: stmt.Value, Span: stmt.Span}, target, value)
			c.requireAssignable(target, result, stmt.Value.GetSpan())
			c.invalidateMemberWriteTarget(stmt.Target, stmt.Span)
		}
	case *ast.IncDecStmt:
		target := c.checkAssignmentTarget(stmt.Target)
		c.markAssignmentTargetRead(stmt.Target)
		if target.Kind != Invalid && !target.IsNumeric() {
			c.report(stmt.Span, fmt.Sprintf("operator %s requires a numeric assignable operand", stmt.Operator))
		}
		c.invalidateMemberWriteTarget(stmt.Target, stmt.Span)
	case *ast.MultiAssignmentStmt:
		c.checkMultiAssignment(stmt)
	case *ast.WhileStmt:
		entryFlow := c.snapshotNullableFlow()
		c.checkLoopFixedPoint(entryFlow, func() (nullableFlowSnapshot, bool) {
			c.checkLoopCondition(stmt.Condition)
			stmt.GuaranteedEntry = c.expressionAlwaysTrue(stmt.Condition)
			narrowing, hasNarrowing := c.nullableConditionNarrowing(stmt.Condition)
			if hasNarrowing && narrowing.nonNullWhenTrue {
				c.applyNarrowing(narrowing)
			}
			c.loopDepth++
			c.checkBlock(stmt.Body, true)
			c.loopDepth--
			return c.snapshotNullableFlow(), !statementDefinitelyStopsBlock(stmt.Body)
		})
	case *ast.ForStmt:
		c.pushScope()
		if stmt.Initializer != nil {
			if variable, ok := stmt.Initializer.(*ast.VariableDecl); ok {
				if _, propagated := variable.Value.(*ast.PropagateExpr); propagated {
					c.report(variable.Value.GetSpan(), "result propagation cannot be used in a for-loop initializer")
				}
			}
			c.checkStatement(stmt.Initializer)
		}
		entryFlow := c.snapshotNullableFlow()
		c.checkLoopFixedPoint(entryFlow, func() (nullableFlowSnapshot, bool) {
			if stmt.Condition != nil {
				c.checkLoopCondition(stmt.Condition)
				stmt.GuaranteedEntry = c.expressionAlwaysTrue(stmt.Condition)
			}
			narrowing, hasNarrowing := c.nullableConditionNarrowing(stmt.Condition)
			if hasNarrowing && narrowing.nonNullWhenTrue {
				c.applyNarrowing(narrowing)
			}
			c.loopDepth++
			c.checkBlock(stmt.Body, true)
			if stmt.Post != nil {
				c.checkStatement(stmt.Post)
			}
			c.loopDepth--
			return c.snapshotNullableFlow(), !statementDefinitelyStopsBlock(stmt.Body)
		})
		c.popScope()
	case *ast.ForRangeStmt:
		types := c.prepareForRange(stmt)
		iterator := stmt.Kind == ast.IteratorRange || stmt.Kind == ast.IteratorPairRange || stmt.Kind == ast.IteratorZeroRange
		if iterator {
			c.recordMemberWrite(stmt.Span)
			c.invalidateAllMemberFacts(stmt.Span, "an iterator call with unknown mutation effects")
		}
		entryFlow := c.snapshotNullableFlow()
		c.checkLoopFixedPoint(entryFlow, func() (nullableFlowSnapshot, bool) {
			if iterator {
				c.invalidateAllMemberFacts(stmt.Span, "an iterator advancing between yields")
			}
			c.checkForRangeBody(stmt, types)
			return c.snapshotNullableFlow(), !statementDefinitelyStopsBlock(stmt.Body)
		})
		if iterator {
			c.invalidateAllMemberFacts(stmt.Span, "an iterator completing after its last yield")
		}
	case *ast.SelectStmt:
		c.checkSelect(stmt)
	case *ast.ValueSwitchStmt:
		c.checkValueSwitch(stmt)
	case *ast.TypeSwitchStmt:
		c.checkTypeSwitch(stmt)
	case *ast.BranchStmt:
		if stmt.Kind == ast.FallthroughBranch {
			if !c.validFallthrough[stmt] {
				c.report(stmt.Span, "fallthrough may only be used as the final statement of a non-final value switch case")
			}
			return
		}
		if stmt.Kind == ast.BreakBranch && c.loopDepth == 0 && c.breakableDepth == 0 {
			c.report(stmt.Span, "break may only be used inside a loop, switch, or select")
		}
		if stmt.Kind == ast.ContinueBranch && c.loopDepth == 0 {
			c.report(stmt.Span, "continue may only be used inside a loop")
		}
		if stmt.Kind == ast.GotoBranch {
			c.invalidateControlTransferFlow(stmt.Span)
			return
		}
		if c.suppressFlowEffects == 0 && len(c.loopFlowContexts) != 0 {
			context := &c.loopFlowContexts[len(c.loopFlowContexts)-1]
			switch {
			case stmt.Kind == ast.ContinueBranch && c.loopDepth != 0:
				context.continues = append(context.continues, c.snapshotNullableFlow())
			case stmt.Kind == ast.BreakBranch && c.loopDepth != 0 && c.breakableDepth == 0:
				context.breaks = append(context.breaks, c.snapshotNullableFlow())
			}
		}
		if c.suppressFlowEffects == 0 && stmt.Kind == ast.BreakBranch && c.breakableDepth != 0 && len(c.breakFlowContexts) != 0 {
			context := &c.breakFlowContexts[len(c.breakFlowContexts)-1]
			context.breaks = append(context.breaks, c.snapshotNullableFlow())
		}
	case *ast.CallControlStmt:
		call, ok := stmt.Value.(*ast.CallExpr)
		if !ok {
			c.checkExpression(stmt.Value)
			return
		}
		value := c.checkExpression(call)
		if value.Kind == Result {
			keyword := "defer"
			if stmt.Kind == ast.GoCall {
				keyword = "go"
			}
			c.report(call.Span, fmt.Sprintf("%s cannot discard a Result; consume it with ? in a Result function", keyword))
		}
		if call.Conversion {
			keyword := "defer"
			if stmt.Kind == ast.GoCall {
				keyword = "go"
			}
			c.report(call.Span, keyword+" requires a function or method call; type conversions are not calls")
		}
	case *ast.DetachStmt:
		c.usesTasks = true
		c.taskOperandDepth++
		task := c.singleValue(c.checkExpression(stmt.Value), stmt.Value.GetSpan())
		c.taskOperandDepth--
		if task.Kind != Task || task.Element == nil {
			if task.Kind != Invalid {
				c.report(stmt.Value.GetSpan(), fmt.Sprintf("detach requires Task<T>, got %s", task.String()))
			}
			return
		}
		c.consumeTask(stmt.Value)
		result := *task.Element
		stmt.ResultTask = result.Kind == Result
		stmt.Void = result.Kind == Void || stmt.ResultTask && result.Element != nil && result.Element.Kind == Void
		value := result
		if stmt.ResultTask && result.Element != nil {
			value = *result.Element
		}
		c.prepareGoTypeForEmission(&value, stmt.Span)
		stmt.ValueType = typeRefFromType(value, stmt.Span)
	case *ast.ChannelSendStmt:
		channelType := c.singleValue(c.checkExpression(stmt.Channel), stmt.Channel.GetSpan())
		if channelType.Kind == Nullable {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("nullable channel %s must be checked against null before sending", channelType.String()))
			if channelType.Element == nil {
				return
			}
			channelType = *channelType.Element
		}
		goType, ok := goTypeOf(channelType)
		if !ok {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("channel send requires a Go channel, got %s", channelType.String()))
			c.checkExpression(stmt.Value)
			return
		}
		channel, ok := gotypes.Unalias(goType).Underlying().(*gotypes.Chan)
		if !ok {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("channel send requires a Go channel, got %s", channelType.String()))
			c.checkExpression(stmt.Value)
			return
		}
		if channel.Dir() == gotypes.RecvOnly {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("cannot send to receive-only channel %s", channelType.String()))
		}
		element, err := kinmokuseiTypeFromGo(channel.Elem())
		if err != nil {
			c.report(stmt.Channel.GetSpan(), fmt.Sprintf("channel element type is not supported: %v", err))
			c.checkExpression(stmt.Value)
			return
		}
		if channelType.Element != nil {
			element = *channelType.Element
		}
		value := c.checkExpressionExpectedSlot(&stmt.Value, element)
		c.requireAssignable(element, value, stmt.Value.GetSpan())
	}
}

func (c *Checker) checkExpression(expr ast.Expression) Type {
	if expr == nil {
		return Type{Kind: Invalid, Name: "<invalid>"}
	}
	switch expr := expr.(type) {
	case *ast.LiteralExpr:
		switch expr.Kind {
		case ast.IntegerLiteral:
			return Type{Kind: UntypedInt, Name: "integer literal"}
		case ast.FloatLiteral, ast.ImaginaryLiteral:
			return c.finishNumeric(expr, gotypes.NewPackage("kinmokusei.synthetic/literal", "literal"), numericLiteralTree(expr))
		case ast.StringLiteral:
			return builtins["string"]
		case ast.BooleanLiteral:
			return builtins["boolean"]
		case ast.NilLiteral:
			return Type{Kind: Nil, Name: "nil"}
		case ast.NullLiteral:
			return Type{Kind: Null, Name: "null"}
		}
	case *ast.IdentifierExpr:
		if c.inFieldInitializer && (expr.Name == "this" || expr.Name == "super") {
			c.report(expr.Span, "class field initializers cannot reference this or super; use the constructor")
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		if symbol, ok := c.lookupSymbol(expr.Name, expr.Span); ok {
			expr.ResolvedDeclaration = symbol.declarationSpan
			if symbol.typeInfo.Kind == Task && c.taskOperandDepth == 0 {
				c.report(expr.Span, "Task values may only be consumed by await or detach and cannot be copied or passed")
			}
			if symbol.constant && symbol.typeInfo.IsNumeric() && !symbol.typeInfo.IsInteger() {
				if info, known := c.checkedNumericConstant(expr, symbol.typeInfo); known {
					if c.numericValues == nil {
						c.numericValues = map[ast.Expression]gotypes.TypeAndValue{}
					}
					c.numericValues[expr] = info
					if basic, ok := info.Type.(*gotypes.Basic); ok && basic.Info()&gotypes.IsUntyped != 0 {
						return Type{Kind: GoBasic, Name: basic.Name(), GoType: basic}
					}
				}
			}
			return symbol.typeInfo
		}
		if imported := c.lookupGoPackage(expr.Span.Path, expr.Name); imported != nil {
			expr.ResolvedDeclaration = imported.declaration.AliasSpan
			if imported.declaration.ResolvedAlias != "" {
				expr.Name = imported.declaration.ResolvedAlias
			}
			return Type{Kind: GoPackage, Name: expr.Name, GoPackage: imported}
		}
		if imported, ok := c.lookupNamedGoImport(expr.Name, expr.Span); ok {
			return c.checkNamedGoIdentifier(expr, imported)
		}
		if function, ok := c.functions[expr.Name]; ok && c.isTopLevelAllowed(expr.Span, expr.Name) {
			c.recordGlobalDependency(expr.Name)
			expr.ResolvedDeclaration = function.declarationSpan
			callable := callableTypeForFunction(function)
			if callable.Generic {
				c.report(expr.Span, fmt.Sprintf("generic function %q must be called before it can be used as a value", expr.Name))
			}
			return callable
		}
		if named, ok := c.nativeTypes[expr.Name]; ok && c.isTopLevelAllowed(expr.Span, expr.Name) {
			expr.ResolvedDeclaration = named.declaration.NameSpan
			c.report(expr.Span, fmt.Sprintf("type %q cannot be used as a value; call it with one argument to convert", expr.Name))
			return Type{Kind: Invalid, Name: "<invalid>"}
		}
		c.report(expr.Span, fmt.Sprintf("undefined name %q", expr.Name))
		return Type{Kind: Invalid, Name: "<invalid>"}
	case *ast.UnaryExpr:
		return c.checkUnary(expr)
	case *ast.BinaryExpr:
		return c.checkBinary(expr)
	case *ast.GoTypeAssertionExpr:
		return c.checkGoTypeAssertion(expr)
	case *ast.TaskStartExpr:
		return c.checkTaskStart(expr)
	case *ast.AwaitExpr:
		return c.checkAwait(expr)
	case *ast.PropagateExpr:
		c.report(expr.Span, "result propagation may only be used as a variable initializer or as a void expression statement")
		return c.checkPropagateExpression(expr)
	case *ast.CallExpr:
		return c.checkCall(expr)
	case *ast.ArrowExpr:
		return c.checkArrow(expr)
	case *ast.ArrayLiteralExpr:
		return c.checkArrayLiteral(expr)
	case *ast.ObjectLiteralExpr:
		return c.checkObjectLiteral(expr)
	case *ast.GoCompositeLiteralExpr:
		return c.checkGoCompositeLiteral(expr)
	case *ast.MemberExpr:
		return c.checkMember(expr)
	case *ast.IndexExpr:
		return c.checkIndex(expr, false)
	case *ast.SliceExpr:
		return c.checkSlice(expr)
	case *ast.NewExpr:
		return c.checkNew(expr)
	case *ast.ClassUpcastExpr:
		return c.resolveType(expr.TargetType)
	}
	return Type{Kind: Invalid, Name: "<invalid>"}
}

func (c *Checker) report(span source.Span, message string) {
	for _, existing := range c.diagnostics {
		if existing.Message == message && existing.Span.Path == span.Path && existing.Span.Start == span.Start && existing.Span.End == span.End {
			return
		}
	}
	c.diagnostics = append(c.diagnostics, diagnostic.Diagnostic{Message: message, Span: span})
}
