// Package nostructfieldassign provides a linter that forbids direct assignment
// to specific struct fields, configurable via a "fields" flag.
//
// Each entry in the flag follows the format:
//
//	<import-path>.<TypeName>.<FieldName>
//
// Example usage:
//
//	-fields=github.com/aws/aws-sdk-go-v2/aws.Config.Region,github.com/foo/bar.MyStruct.SomeField
//
// Or via golangci-lint custom linter settings:
//
//	linters-settings:
//	  custom:
//	    nostructfieldassign:
//	      settings:
//	        fields:
//	          - "github.com/aws/aws-sdk-go-v2/aws.Config.Region"
package nostructfieldassign

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// fieldKey uniquely identifies a struct field by its package import path,
// type name, and field name.
type fieldKey struct {
	pkgPath   string
	typeName  string
	fieldName string
}

// fieldsFlag implements flag.Value for a comma-separated list of
// "<pkgPath>.<TypeName>.<FieldName>" entries.
type fieldsFlag []fieldKey

func (f *fieldsFlag) String() string {
	parts := make([]string, len(*f))
	for i, k := range *f {
		parts[i] = fmt.Sprintf("%s.%s.%s", k.pkgPath, k.typeName, k.fieldName)
	}
	return strings.Join(parts, ",")
}

func (f *fieldsFlag) Set(s string) error {
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		k, err := parseFieldKey(entry)
		if err != nil {
			return err
		}
		*f = append(*f, k)
	}
	return nil
}

// parseFieldKey parses a string of the form "<pkgPath>.<TypeName>.<FieldName>".
// The package import path may itself contain dots (e.g. "github.com/aws/aws-sdk-go-v2/aws"),
// so we split from the right: the last two dot-separated tokens are TypeName and FieldName,
// and everything before that is the import path.
func parseFieldKey(s string) (fieldKey, error) {
	// We need at least pkgPath + TypeName + FieldName, so at least one slash
	// to separate the pkg path from "Type.Field", but we handle it generically.
	lastDot := strings.LastIndex(s, ".")
	if lastDot < 0 {
		return fieldKey{}, fmt.Errorf("nostructfieldassign: invalid field spec %q: expected <pkgPath>.<Type>.<Field>", s)
	}
	fieldName := s[lastDot+1:]
	rest := s[:lastDot]

	secondLastDot := strings.LastIndex(rest, ".")
	if secondLastDot < 0 {
		return fieldKey{}, fmt.Errorf("nostructfieldassign: invalid field spec %q: expected <pkgPath>.<Type>.<Field>", s)
	}
	typeName := rest[secondLastDot+1:]
	pkgPath := rest[:secondLastDot]

	if pkgPath == "" || typeName == "" || fieldName == "" {
		return fieldKey{}, fmt.Errorf("nostructfieldassign: invalid field spec %q: pkgPath, type, and field must all be non-empty", s)
	}
	return fieldKey{pkgPath: pkgPath, typeName: typeName, fieldName: fieldName}, nil
}

// NewAnalyzer creates a new Analyzer. Callers may populate defaultFields to
// pre-seed the forbidden list without requiring a flag (useful when embedding
// the analyzer in a custom binary).
func NewAnalyzer(defaultFields ...string) *analysis.Analyzer {
	ff := fieldsFlag{}
	for _, f := range defaultFields {
		if err := ff.Set(f); err != nil {
			panic(err)
		}
	}

	a := &analysis.Analyzer{
		Name:     "nostructfieldassign",
		Doc:      "forbids direct assignment to configured struct fields (assignments and composite literals)",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}

	// Bind flags before returning; the analysis framework calls Flag.Parse
	// after the analyzer is registered.
	a.Flags = *flag.NewFlagSet("nostructfieldassign", flag.ContinueOnError)
	a.Flags.Var(&ff, "fields",
		"comma-separated list of forbidden struct fields in the form <pkgPath>.<Type>.<Field>")

	a.Run = func(pass *analysis.Pass) (interface{}, error) {
		return run(pass, ff)
	}

	return a
}

// Analyzer is a ready-to-use instance with no pre-configured fields.
// Configure it at runtime via the -fields flag.
var Analyzer = NewAnalyzer()

func run(pass *analysis.Pass, fields fieldsFlag) (interface{}, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	// Build lookup maps keyed by field name for fast pre-filtering.
	// Map: fieldName -> []fieldKey (there may be multiple types with the same field name).
	byFieldName := make(map[string][]fieldKey, len(fields))
	for _, k := range fields {
		byFieldName[k.fieldName] = append(byFieldName[k.fieldName], k)
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.CompositeLit)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
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
func checkAssignment(pass *analysis.Pass, assign *ast.AssignStmt, byFieldName map[string][]fieldKey) {
	for _, lhs := range assign.Lhs {
		sel, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		keys, ok := byFieldName[sel.Sel.Name]
		if !ok {
			continue
		}
		t := pass.TypesInfo.TypeOf(sel.X)
		if matchesAny(t, sel.Sel.Name, keys) {
			pass.Reportf(sel.Pos(), "direct assignment to %s.%s is forbidden",
				typeName(t), sel.Sel.Name)
		}
	}
}

// checkCompositeLit reports forbidden field assignments inside composite literals:
//
//	pkg.Type{Field: value}
func checkCompositeLit(pass *analysis.Pass, lit *ast.CompositeLit, byFieldName map[string][]fieldKey) {
	if lit.Type == nil {
		return
	}
	t := pass.TypesInfo.TypeOf(lit)
	if t == nil {
		return
	}
	// Unwrap pointer so *aws.Config{} is also caught.
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
		keys, ok := byFieldName[ident.Name]
		if !ok {
			continue
		}
		if matchesAny(base, ident.Name, keys) {
			pass.Reportf(ident.Pos(), "setting %s.%s in a composite literal is forbidden",
				typeName(base), ident.Name)
		}
	}
}

// matchesAny returns true if t (after pointer dereferencing) corresponds to
// any of the provided fieldKeys for the given field name.
func matchesAny(t types.Type, _ string, keys []fieldKey) bool {
	named, ok := deref(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	for _, k := range keys {
		if obj.Name() == k.typeName && obj.Pkg().Path() == k.pkgPath {
			return true
		}
	}
	return false
}

// deref unwraps a single pointer level, if present.
func deref(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// typeName returns a short human-readable name for diagnostic messages.
func typeName(t types.Type) string {
	t = deref(t)
	named, ok := t.(*types.Named)
	if !ok {
		return t.String()
	}
	obj := named.Obj()
	if obj.Pkg() != nil {
		return obj.Pkg().Name() + "." + obj.Name()
	}
	return obj.Name()
}
