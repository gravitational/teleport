// Teleport
// Copyright (C) 2026  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package metrics

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// MetricInfo holds extracted data about a single Prometheus metric declaration.
type MetricInfo struct {
	// FilePath is the relative path (from the source root) to the file containing the metric declaration.
	FilePath string
	// Namespace is the Prometheus metric namespace.
	Namespace string
	// Subsystem is the Prometheus metric subsystem.
	Subsystem string
	// Name is the Prometheus metric name component.
	Name string
	// Help is the human-readable description of the metric.
	Help string
	// FullName is the fully assembled Prometheus metric name:
	// namespace_subsystem_name, with empty components omitted.
	FullName string
	// Type is the metric type derived from the constructor function name,
	// e.g. "gauge", "counter", "histogram".
	Type string
}

// prometheusConstructors maps known Prometheus constructor function names to their metric types,
// which are presented as a column value in the output table.
var prometheusConstructors = map[string]string{
	"NewGauge":        "gauge",
	"NewGaugeVec":     "gauge",
	"NewCounter":      "counter",
	"NewCounterVec":   "counter",
	"NewHistogram":    "histogram",
	"NewHistogramVec": "histogram",
	"NewSummary":      "summary",
	"NewSummaryVec":   "summary",
}

// CollectMetrics loads and parses Go source files under rootPath, returning all found Prometheus metric declarations.
// prefix is the Go module path (e.g. "github.com/gravitational/teleport")
// rootPath is the directory tree to scan for metrics.
func CollectMetrics(prefix, rootPath string) ([]MetricInfo, error) {
	var collectedMetrics []MetricInfo

	absRootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve root path: %w", err)
	}

	// Locate the Go module root so we can collect constants from all packages
	// in the module, enabling resolution of symbolic names used in Prometheus
	// options (e.g. teleport.MetricNamespace).
	moduleRoot, err := findModuleRoot(absRootPath)
	if err != nil {
		return nil, fmt.Errorf("finding module root: %w", err)
	}

	// Pass 1: collect all package-level constants. This enables resolving
	// symbolic constant references when collecting Prometheus metric declarations.
	// For example teleport.MetricNamespace resolved to its constant value "teleport".
	allConsts, err := collectModuleConsts(prefix, moduleRoot)

	if err != nil {
		return nil, fmt.Errorf("collecting package constants: %w", err)
	}

	// Pass 2: collect Prometheus metric declarations. Resolve symbolic constant references using allConsts.
	err = filepath.WalkDir(absRootPath, func(currentPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("loading Go source file %v: %w", currentPath, err)
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) != ".go" || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		// Derive the canonical Go package import path from the module root
		// so that it is directly comparable to imported package paths.
		relFromModule, err := filepath.Rel(moduleRoot, currentPath)
		if err != nil {
			return fmt.Errorf("unable to find a relative path between %v and %v: %w", moduleRoot, currentPath, err)
		}
		pkg := path.Join(prefix, filepath.ToSlash(filepath.Dir(relFromModule)))

		f, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		defer f.Close()
		fset := token.NewFileSet()

		file, err := parser.ParseFile(fset, currentPath, f, parser.ParseComments)
		if err != nil {
			return err
		}

		relDeclPath, err := filepath.Rel(absRootPath, currentPath)
		if err != nil {
			return err
		}

		imports := namedImports(file)
		resolver := newExpressionResolver(pkg, imports, allConsts)

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			// Handle general declarations where Prometheus metrics might be defined as package-level constants or variables.
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					comment := vs.Doc
					if comment == nil && len(d.Specs) == 1 {
						comment = d.Doc
					}
					for i, val := range vs.Values {
						call, ok := val.(*ast.CallExpr)
						if !ok {
							continue
						}
						if i < len(vs.Names) {
							metric, ok := metricInfoFromCall(call, relDeclPath, imports, resolver.resolveString)
							if !ok {
								continue
							}
							// If the help string is not explicitly set in the Opts struct,
							// fall back to using the comment associated with the metric declaration.
							if metric.Help == "" {
								metric.Help = commentDescription(comment, vs.Names[i].Name)
							}
							collectedMetrics = append(collectedMetrics, metric)
						}
					}
				}
			// Handle cases where Prometheus metrics are defined within function bodies.
			case *ast.FuncDecl:
				registryNamespace, _ := allConsts[prefix]["MetricNamespace"].(string)
				functionResolver := resolver.forFunction(d, prefix, registryNamespace)
				ast.Inspect(d.Body, func(node ast.Node) bool {
					functionResolver.recordLocalValues(node)

					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					metric, ok := metricInfoFromCall(call, relDeclPath, imports, functionResolver.resolveString)
					if ok {
						collectedMetrics = append(collectedMetrics, metric)
					}
					return true
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("loading Go source files: %w", err)
	}
	return collectedMetrics, nil
}

// metricInfoFromCall checks if the given function invocation represents a Prometheus metric declaration.
// If it does, returns the corresponding MetricInfo struct.
func metricInfoFromCall(
	call *ast.CallExpr,
	filePath string,
	imports map[string]string,
	resolve func(ast.Expr) (string, bool),
) (MetricInfo, bool) {
	// Check that the function being called is a selector expression (e.g., package.Function(...)).
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return MetricInfo{}, false
	}
	// Determine the type of Prometheus metric being constructed.
	metricType, ok := prometheusConstructors[selector.Sel.Name]
	if !ok {
		return MetricInfo{}, false
	}
	// Extract the package identifier from the selector expression (e.g., "prometheus" in prometheus.NewCounter).
	packageIdentifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return MetricInfo{}, false
	}

	// Ensure that the call is to a Prometheus constructor from the correct import path and has at least one argument.
	const prometheusImportPath = "github.com/prometheus/client_golang/prometheus"
	if imports[packageIdentifier.Name] != prometheusImportPath || len(call.Args) == 0 {
		return MetricInfo{}, false
	}

	// Extract the composite literal representing the metric options.
	opts, ok := call.Args[0].(*ast.CompositeLit)
	if !ok {
		return MetricInfo{}, false
	}

	namespace, subsystem, name, help := extractMetricData(opts, resolve)
	return MetricInfo{
		FilePath:  filePath,
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
		FullName:  assembleFullName(namespace, subsystem, name),
		Type:      metricType,
	}, true
}

// findModuleRoot traverses the directory tree upward from startPath until it
// finds a directory containing a go.mod file and returns that directory.
func findModuleRoot(startPath string) (string, error) {
	dir, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}
	// If startPath is a file rather than a directory, begin from its parent.
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found starting from %v", startPath)
		}
		dir = parent
	}
}

// collectModuleConsts collects all package-level constants across the Go module rooted at moduleRoot.
func collectModuleConsts(prefix, moduleRoot string) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any)
	err := filepath.WalkDir(moduleRoot, func(currentPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip certain directories that are not relevant for collecting module-level constants.
			switch d.Name() {
			case "node_modules", ".git", "docs", "e2e", "e", "web":
				return filepath.SkipDir
			}
			// Skip directories that are roots of separate Go modules.
			if currentPath != moduleRoot {
				if _, err := os.Stat(filepath.Join(currentPath, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		// Skip non-Go files and test files.
		if filepath.Ext(d.Name()) != ".go" || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(moduleRoot, currentPath)
		if err != nil {
			return err
		}
		pkg := path.Join(prefix, filepath.ToSlash(filepath.Dir(rel)))

		f, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		defer f.Close()
		fset := token.NewFileSet()
		// Dismiss comments while parsing the file.
		file, err := parser.ParseFile(fset, currentPath, f, 0)
		if err != nil {
			return nil // skip files that fail to parse
		}

		for _, decl := range file.Decls {
			l, ok := decl.(*ast.GenDecl)
			if !ok || l.Tok != token.CONST {
				continue
			}
			for _, spec := range l.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, val := range vs.Values {
					if i >= len(vs.Names) {
						continue
					}

					pkgConsts := result[pkg]
					if pkgConsts == nil {
						pkgConsts = make(map[string]any)
						result[pkg] = pkgConsts
					}
					if value, ok := resolveConstantValue(val, pkgConsts); ok {
						pkgConsts[vs.Names[i].Name] = value
					}
				}
			}
		}
		return nil
	})
	return result, err
}

// resolveConstantValue attempts to evaluate an AST expression to its constant value.
// It supports basic literals, identifiers, binary expressions (string concatenation),
// and single-argument type conversions. It returns the value and a boolean indicating success.
func resolveConstantValue(expr ast.Expr, constants map[string]any) (any, bool) {
	switch value := expr.(type) {
	// Handle basic literal values such as strings and integers.
	case *ast.BasicLit:
		switch value.Kind {
		case token.STRING:
			unquoted, err := strconv.Unquote(value.Value)
			return unquoted, err == nil
		case token.INT:
			integer, err := strconv.ParseInt(value.Value, 0, 64)
			return integer, err == nil
		}
		// Handle identifiers by looking them up in the constants map.
	case *ast.Ident:
		constant, ok := constants[value.Name]
		return constant, ok
		// Handle binary expressions, specifically string concatenation.
	case *ast.BinaryExpr:
		return resolveStringConcat(value, func(expr ast.Expr) (any, bool) {
			return resolveConstantValue(expr, constants)
		})
	// Handle single-argument type conversions: for example Namespace: reg.Namespace()
	case *ast.CallExpr:
		if len(value.Args) == 1 {
			return resolveConstantValue(value.Args[0], constants)
		}
	}
	return nil, false
}

// resolveStringConcat attempts to resolve a binary expression representing string concatenation.
// It returns the concatenated string and a boolean indicating success.
func resolveStringConcat(
	expr *ast.BinaryExpr,
	resolve func(ast.Expr) (any, bool),
) (any, bool) {
	// only "+" is supported, as it represents string concatenation.
	if expr.Op != token.ADD {
		return nil, false
	}

	left, ok := resolve(expr.X)
	if !ok {
		return nil, false
	}
	right, ok := resolve(expr.Y)
	if !ok {
		return nil, false
	}
	ls, ok := left.(string)
	if !ok {
		return nil, false
	}
	rs, ok := right.(string)
	if !ok {
		return nil, false
	}
	return ls + rs, true
}

// expressionResolver resolves statically knowable values from Go expressions.
// A function-scoped resolver also tracks local strings and Teleport metrics Registry parameters.
type expressionResolver struct {
	// The package path of the Go package being analyzed.
	pkg string
	// A map of import aliases to their full package paths.
	imports map[string]string
	// A map of package-level constants, keyed by package path and constant name.
	constants map[string]map[string]any
	// A map of local variable names to their resolved string values.
	locals map[string]string
	// A set of registry variables encountered within the function scope.
	registryVariables map[string]struct{}
	// The namespace associated with the registry within the function scope.
	registryNamespace string
}

// newExpressionResolver creates a new expression resolver for the given package, imports, and constants.
func newExpressionResolver(pkg string, imports map[string]string, constants map[string]map[string]any) *expressionResolver {
	return &expressionResolver{
		pkg:       pkg,
		imports:   imports,
		constants: constants,
		locals:    make(map[string]string),
	}
}

// forFunction creates an isolated resolver for each function. It shares immutable package information while
// maintaining its own local variables and registry parameters.
func (r *expressionResolver) forFunction(function *ast.FuncDecl, prefix, registryNamespace string) *expressionResolver {
	return &expressionResolver{
		pkg:               r.pkg,
		imports:           r.imports,
		constants:         r.constants,
		locals:            make(map[string]string),
		registryVariables: metricRegistryParameters(function, r.imports, prefix),
		registryNamespace: registryNamespace,
	}
}

// resolveString attempts to resolve the given expression to a string value.
// It returns the resolved string and a boolean indicating success.
func (r *expressionResolver) resolveString(expr ast.Expr) (string, bool) {
	value, ok := r.resolveValue(expr)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

// resolveValue attempts to resolve the given expression to a statically knowable value.
// It returns the resolved value and a boolean indicating success.
func (r *expressionResolver) resolveValue(expr ast.Expr) (any, bool) {
	switch expr := expr.(type) {
	case *ast.BasicLit:
		return resolveConstantValue(expr, nil)
	case *ast.Ident:
		if value, ok := r.locals[expr.Name]; ok {
			return value, true
		}
		value, ok := r.constants[r.pkg][expr.Name]
		return value, ok
	case *ast.SelectorExpr:
		packageIdentifier, ok := expr.X.(*ast.Ident)
		if !ok {
			return nil, false
		}
		importPath, ok := r.imports[packageIdentifier.Name]
		if !ok {
			return nil, false
		}
		value, ok := r.constants[importPath][expr.Sel.Name]
		return value, ok
	case *ast.BinaryExpr:
		return resolveStringConcat(expr, r.resolveValue)
	case *ast.CallExpr:
		return r.resolveCall(expr)
	}
	return nil, false
}

// resolveCall attempts to resolve a function call expression.
func (r *expressionResolver) resolveCall(call *ast.CallExpr) (any, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	if len(call.Args) == 0 {
		if value, ok := r.resolveRegistryAccessor(selector); ok {
			return value, true
		}
	}
	return r.resolveSprintf(selector, call.Args)
}

// resolveRegistryAccessor attempts to resolve a call to a registry accessor method, such as Namespace or Subsystem.
func (r *expressionResolver) resolveRegistryAccessor(selector *ast.SelectorExpr) (string, bool) {
	receiver, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	if _, ok := r.registryVariables[receiver.Name]; !ok {
		return "", false
	}

	switch selector.Sel.Name {
	case "Namespace":
		return r.registryNamespace, r.registryNamespace != ""
	case "Subsystem":
		subsystem, ok := r.constants[r.pkg]["metricsSubsystem"].(string)
		return subsystem, ok
	default:
		return "", false
	}
}

// resolveSprintf attempts to resolve a call to fmt.Sprintf with a statically knowable format string and arguments.
func (r *expressionResolver) resolveSprintf(selector *ast.SelectorExpr, args []ast.Expr) (string, bool) {
	packageIdentifier, ok := selector.X.(*ast.Ident)
	if !ok || r.imports[packageIdentifier.Name] != "fmt" || selector.Sel.Name != "Sprintf" || len(args) == 0 {
		return "", false
	}

	format, ok := r.resolveString(args[0])
	if !ok {
		return "", false
	}
	values := make([]any, 0, len(args)-1)
	for _, arg := range args[1:] {
		value, ok := r.resolveValue(arg)
		// If the value cannot be resolved, use the text up to the format verb as a fallback.
		// TODO: Handle cases where the arguments are not statically resolvable.
		if !ok {
			format = truncateBeforeVerb(format, len(values))
			break
		}
		values = append(values, value)
	}
	return strings.TrimSpace(fmt.Sprintf(format, values...)), true
}

// truncateBeforeVerb removes the format string starting at the given
// format verb position, discarding unresolved trailing text.
func truncateBeforeVerb(format string, verbIndex int) string {
	count := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			continue
		}
		i++
		if format[i] == '%' {
			continue
		}
		if count == verbIndex {
			return format[:i-1]
		}
		count++
	}
	return format
}

// metricRegistryParameters returns the names of function parameters whose type
// is the Teleport metrics Registry, excluding unrelated Registry types.
func metricRegistryParameters(function *ast.FuncDecl, imports map[string]string, prefix string) map[string]struct{} {
	registryVariables := make(map[string]struct{})
	if function.Type.Params == nil {
		return registryVariables
	}

	registryImportPath := path.Join(prefix, "lib/observability/metrics")
	for _, field := range function.Type.Params.List {
		pointer, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		selector, ok := pointer.X.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Registry" {
			continue
		}
		packageIdentifier, ok := selector.X.(*ast.Ident)
		if !ok || imports[packageIdentifier.Name] != registryImportPath {
			continue
		}
		for _, name := range field.Names {
			registryVariables[name.Name] = struct{}{}
		}
	}
	return registryVariables
}

// recordLocalValues records statically resolvable local declarations and
// assignments and removes values when a later assignment cannot be resolved.
func (r *expressionResolver) recordLocalValues(node ast.Node) {
	switch node := node.(type) {
	case *ast.ValueSpec:
		r.setLocalValues(node.Names, node.Values)
	case *ast.AssignStmt:
		names := make([]*ast.Ident, 0, len(node.Lhs))
		for _, expression := range node.Lhs {
			name, ok := expression.(*ast.Ident)
			if !ok {
				return
			}
			names = append(names, name)
		}
		r.setLocalValues(names, node.Rhs)
	}
}

// setLocalValues records the resolved values of local variables.
func (r *expressionResolver) setLocalValues(names []*ast.Ident, values []ast.Expr) {
	for i, name := range names {
		if i >= len(values) {
			continue
		}
		// Attempt to resolve the value of the local variable before recording it.
		if value, ok := r.resolveString(values[i]); ok {
			r.locals[name.Name] = value
		} else {
			// Remove any previously recorded value for this local variable,
			// as it can no longer be statically resolved.
			delete(r.locals, name.Name)
		}
	}
}

// namedImports returns a map from package name (or alias) to import path for
// all imports in the given file.
func namedImports(file *ast.File) map[string]string {
	result := make(map[string]string)
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		var name string
		if imp.Name != nil {
			name = imp.Name.Name
		} else {
			parts := strings.Split(importPath, "/")
			name = parts[len(parts)-1]
		}
		// Dismiss blank (_) imports
		if name != "_" {
			result[name] = importPath
		}
	}
	return result
}

// extractMetricData reads the Namespace, Subsystem, Name, and Help entries from a Prometheus metric options struct.
// The supplied resolve function resolves literals and supported constant expressions.
func extractMetricData(lit *ast.CompositeLit, resolve func(ast.Expr) (string, bool)) (namespace, subsystem, name, help string) {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		val, ok := resolve(kv.Value)
		if !ok {
			continue
		}
		switch key.Name {
		case "Namespace":
			namespace = val
		case "Subsystem":
			subsystem = val
		case "Name":
			name = val
		case "Help":
			help = val
		}
	}
	return
}

// commentDescription extracts a description from a comment group, removing the identifier and common suffixes.
// Used to retrieve a fallback description when the "Help" value is not provided for the Prometheus metric.
func commentDescription(comment *ast.CommentGroup, ident string) string {
	if comment == nil {
		return ""
	}
	description := strings.Join(strings.Fields(comment.Text()), " ")
	if description == ident {
		return ""
	}
	// Remove common suffixes from the description.
	for _, suffix := range []string{" are ", " is ", " "} {
		// For example trim out "MetricName is " from the beginning of the description.
		if strings.HasPrefix(description, ident+suffix) {
			description = strings.TrimPrefix(description, ident+suffix)
			break
		}
	}
	if description == "" {
		return ""
	}
	return strings.ToUpper(description[:1]) + description[1:]
}

// assembleFullName builds the full Prometheus metric name by appending the namespace,
// subsystem, and name components with underscores, skipping any empty parts.
func assembleFullName(namespace, subsystem, name string) string {
	parts := make([]string, 0, 3)
	for _, p := range []string{namespace, subsystem, name} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "_")
}
