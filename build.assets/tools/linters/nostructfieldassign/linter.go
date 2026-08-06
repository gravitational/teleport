// Package nostructfieldassign provides a linter that forbids direct assignment
// to specific struct fields.
package nostructfieldassign

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Rule identifies a struct field that must not be assigned directly.
type Rule struct {
	// Package is the full import path that declares the struct type.
	Package string `mapstructure:"pkg"`
	// Type is the struct type name that owns the forbidden field.
	Type string `mapstructure:"type"`
	// Field is the struct field name that must not be set directly.
	Field string `mapstructure:"field"`
	// ErrorMessage is an optional explanation appended to the diagnostic.
	ErrorMessage string `mapstructure:"msg"`
}

// NewAnalyzer creates a new Analyzer with the provided forbidden-field rules.
func NewAnalyzer(rules ...Rule) *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name:     "nostructfieldassign",
		Doc:      "forbids direct assignment to configured struct fields (assignments and composite literals)",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}

	a.Run = func(pass *analysis.Pass) (any, error) {
		return run(pass, rules)
	}

	return a
}

func run(pass *analysis.Pass, rules []Rule) (any, error) {
	if len(rules) == 0 {
		return nil, nil
	}

	// Map field name to matching rules for fast pre-filtering.
	byFieldName := make(map[string][]Rule, len(rules))
	for _, rule := range rules {
		byFieldName[rule.Field] = append(byFieldName[rule.Field], rule)
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	insp.Preorder([]ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.CompositeLit)(nil),
	}, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.AssignStmt:
			checkAssignment(pass, node, byFieldName)
		case *ast.CompositeLit:
			checkCompositeLit(pass, node, byFieldName)
		}
	})

	return nil, nil
}

// checkAssignment reports forbidden assignments of the form:
//
//	x.Field = ...
//	*x.Field = ...
func checkAssignment(pass *analysis.Pass, assign *ast.AssignStmt, byFieldName map[string][]Rule) {
	for _, lhs := range assign.Lhs {
		sel, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		rules, ok := byFieldName[sel.Sel.Name]
		if !ok {
			continue
		}
		if rule, owner, ok := matchesSelector(pass, sel, rules); ok {
			pass.Reportf(sel.Pos(), "direct assignment to %s.%s is forbidden%s",
				namedTypeName(owner), sel.Sel.Name, forbiddenMsg(rule.ErrorMessage))
		}
	}
}

// checkCompositeLit reports forbidden field assignments inside composite literals:
//
//	pkg.Type{Field: value}
func checkCompositeLit(pass *analysis.Pass, lit *ast.CompositeLit, byFieldName map[string][]Rule) {
	t := pass.TypesInfo.TypeOf(lit)
	if t == nil {
		return
	}
	base := deref(t)

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		rules, ok := byFieldName[ident.Name]
		if !ok {
			continue
		}
		if rule, owner, ok := matchesAny(base, rules); ok {
			pass.Reportf(ident.Pos(), "setting %s.%s in a composite literal is forbidden%s",
				namedTypeName(owner), ident.Name, forbiddenMsg(rule.ErrorMessage))
		}
	}
}

// matchesAny returns the matching rule and declaring type if t corresponds to
// one of the provided rules.
func matchesAny(t types.Type, rules []Rule) (Rule, *types.Named, bool) {
	named, ok := deref(t).(*types.Named)
	if !ok {
		return Rule{}, nil, false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return Rule{}, nil, false
	}
	for _, rule := range rules {
		if obj.Name() == rule.Type && obj.Pkg().Path() == rule.Package {
			return rule, named, true
		}
	}
	return Rule{}, nil, false
}

// matchesSelector resolves selector expressions, including promoted fields from
// embedded structs, and returns the declaring named type for the matched field.
func matchesSelector(pass *analysis.Pass, sel *ast.SelectorExpr, rules []Rule) (Rule, *types.Named, bool) {
	selection := pass.TypesInfo.Selections[sel]
	if selection == nil {
		return matchesAny(pass.TypesInfo.TypeOf(sel.X), rules)
	}

	owner, field, ok := resolveSelectedField(pass.TypesInfo.TypeOf(sel.X), selection.Index())
	if !ok {
		return Rule{}, nil, false
	}
	for _, rule := range rules {
		if owner.Obj() == nil || owner.Obj().Pkg() == nil {
			continue
		}
		if owner.Obj().Name() == rule.Type && owner.Obj().Pkg().Path() == rule.Package && field.Name() == rule.Field {
			return rule, owner, true
		}
	}
	return Rule{}, nil, false
}

// resolveSelectedField walks a field-selection index path and returns the named
// type that declares the selected field.
func resolveSelectedField(t types.Type, index []int) (*types.Named, *types.Var, bool) {
	current := deref(t)
	for i, fieldIndex := range index {
		named, ok := current.(*types.Named)
		if !ok {
			return nil, nil, false
		}
		strct, ok := named.Underlying().(*types.Struct)
		if !ok || fieldIndex < 0 || fieldIndex >= strct.NumFields() {
			return nil, nil, false
		}
		field := strct.Field(fieldIndex)
		if i == len(index)-1 {
			return named, field, true
		}
		current = deref(field.Type())
	}
	return nil, nil, false
}

func forbiddenMsg(msg string) string {
	if msg == "" {
		return ""
	}
	return fmt.Sprintf(" because %q", msg)
}

func deref(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

func namedTypeName(named *types.Named) string {
	if named == nil {
		return ""
	}
	obj := named.Obj()
	if obj == nil {
		return named.String()
	}
	if obj.Pkg() != nil {
		return obj.Pkg().Name() + "." + obj.Name()
	}
	return obj.Name()
}
