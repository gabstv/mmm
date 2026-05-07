package mmmvet

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

const mmmPkgPath = "github.com/gabstv/mmm"

// Analyzer detects unsafe arena allocations where pointer-bearing values are
// stored in arena memory without being pinned.
var Analyzer = &analysis.Analyzer{
	Name:     "mmmvet",
	Doc:      "detect unsafe arena allocations with GC-invisible pointer-bearing fields",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Fast exit: check if this package imports mmm.
	importsMmm := false
	for _, imp := range pass.Pkg.Imports() {
		if imp.Path() == mmmPkgPath {
			importsMmm = true
			break
		}
	}
	if !importsMmm {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// allocVars maps types.Object -> true for variables that hold the result
	// of mmm.Alloc / mmm.TryAlloc where T had pointer-bearing fields.
	allocVars := make(map[types.Object]bool)

	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.CallExpr)(nil),
	}

	// We need two passes: first collect Alloc calls, then check assignments.
	// Since inspector visits in order, we do it in one pass but handle both
	// node types.

	insp.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.CallExpr:
			checkAllocCall(pass, node, allocVars)
		case *ast.AssignStmt:
			checkAssignment(pass, node, allocVars)
		}
	})

	return nil, nil
}

// checkAllocCall checks if the call is mmm.Alloc[T] or mmm.TryAlloc[T] where
// T contains pointer-bearing fields, and records the LHS variable if so.
func checkAllocCall(pass *analysis.Pass, call *ast.CallExpr, allocVars map[types.Object]bool) {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil {
		return
	}
	if fn.Pkg() == nil || fn.Pkg().Path() != mmmPkgPath {
		return
	}
	if fn.Name() != "Alloc" && fn.Name() != "TryAlloc" {
		return
	}

	// Get the type argument T from the instantiation.
	ident := funcIdent(call)
	if ident == nil {
		return
	}
	inst, ok := pass.TypesInfo.Instances[ident]
	if !ok || inst.TypeArgs == nil || inst.TypeArgs.Len() == 0 {
		return
	}
	T := inst.TypeArgs.At(0)

	if !containsPointers(T) {
		return
	}

	fields := pointerBearingFields(T)
	pass.Reportf(call.Pos(), "mmm.Alloc with pointer-bearing fields: %s", strings.Join(fields, ", "))

	// Track the LHS variable so we can warn on subsequent field assignments.
	// Walk up the AST to find the enclosing AssignStmt.
	// We can't easily walk up the AST from inside Preorder, so we handle
	// the assignment tracking in checkAssignment instead.
	// Mark this call position as a "danger" call so checkAssignment can use it.
	_ = allocVars
}

// checkAssignment handles two things:
//  1. Records variables assigned from mmm.Alloc calls with pointer-bearing fields.
//  2. Warns when a pointer-bearing field is assigned without Pin/PinManaged.
func checkAssignment(pass *analysis.Pass, stmt *ast.AssignStmt, allocVars map[types.Object]bool) {
	// Check if any RHS is an Alloc call.
	for i, rhs := range stmt.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok {
			continue
		}
		fn := typeutil.StaticCallee(pass.TypesInfo, call)
		if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != mmmPkgPath {
			continue
		}
		if fn.Name() != "Alloc" && fn.Name() != "TryAlloc" {
			continue
		}

		ident := funcIdent(call)
		if ident == nil {
			continue
		}
		inst, ok := pass.TypesInfo.Instances[ident]
		if !ok || inst.TypeArgs == nil || inst.TypeArgs.Len() == 0 {
			continue
		}
		T := inst.TypeArgs.At(0)
		if !containsPointers(T) {
			continue
		}

		// Find the corresponding LHS variable.
		if i < len(stmt.Lhs) {
			lhsIdent, ok := stmt.Lhs[i].(*ast.Ident)
			if ok {
				obj := pass.TypesInfo.ObjectOf(lhsIdent)
				if obj != nil {
					allocVars[obj] = true
				}
			}
		}
	}

	// Now check if LHS is a field access on a tracked variable, and RHS is not Pin/PinManaged.
	for i, lhs := range stmt.Lhs {
		sel, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		// Resolve the receiver.
		recvIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			continue
		}
		recvObj := pass.TypesInfo.ObjectOf(recvIdent)
		if recvObj == nil || !allocVars[recvObj] {
			continue
		}

		// Check if this field's type contains pointers.
		selObj := pass.TypesInfo.ObjectOf(sel.Sel)
		if selObj == nil {
			continue
		}
		fieldType := selObj.Type()
		if !containsPointers(fieldType) {
			continue
		}

		// Check if the RHS is a call to Pin or PinManaged.
		if i < len(stmt.Rhs) {
			if isPinCall(pass, stmt.Rhs[i]) {
				continue
			}
			// Also handle multi-value assignment like: s.Name, _ = PinManaged(...)
			// In that case there's one RHS call and multiple LHS.
			if len(stmt.Rhs) == 1 {
				if isPinCall(pass, stmt.Rhs[0]) {
					continue
				}
			}
		}

		pass.Reportf(lhs.Pos(), "assigning pointer-bearing field %q to arena-allocated struct without Pin or PinManaged", sel.Sel.Name)
	}
}

// isPinCall returns true if expr is a call to mmm.Pin or mmm.PinManaged.
func isPinCall(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != mmmPkgPath {
		return false
	}
	return fn.Name() == "Pin" || fn.Name() == "PinManaged"
}

// funcIdent extracts the function identifier from a CallExpr for use with
// TypesInfo.Instances (generic instantiation lookup).
func funcIdent(call *ast.CallExpr) *ast.Ident {
	switch fun := call.Fun.(type) {
	case *ast.IndexExpr: // Alloc[T](a)
		return selectorIdent(fun.X)
	case *ast.IndexListExpr: // future: multiple type params
		return selectorIdent(fun.X)
	case *ast.SelectorExpr: // non-generic call
		return fun.Sel
	case *ast.Ident:
		return fun
	}
	return nil
}

func selectorIdent(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		return e.Sel
	case *ast.Ident:
		return e
	}
	return nil
}
