// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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

package databaseobjectimportrule

import (
	"context"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/gravitational/trace"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
	libutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// ApplyDatabaseObjectImportRules applies the specified set of rules to a collection of objects from the same database.
// It returns a new copy of a subset of the provided objects, filtered and labeled according to the rules.
// An object is included in the return set only if it matches at least one rule.
// Objects with errors due to invalid label templates are excluded.
// The function returns the final count of errors encountered.
func ApplyDatabaseObjectImportRules(ctx context.Context, logger *slog.Logger, rules []*dbobjectimportrulev1.DatabaseObjectImportRule, database types.Database, objs []*dbobjectv1.DatabaseObject) ([]*dbobjectv1.DatabaseObject, int) {
	// sort: rules with higher priorities are applied last.
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Spec.Priority < rules[j].Spec.Priority
	})

	// get mappings from rules matching database labels
	var mappings []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping
	for _, rule := range filterRulesForDatabase(rules, database) {
		mappings = append(mappings, rule.Spec.Mappings...)
	}

	var objects []*dbobjectv1.DatabaseObject
	var errCount int
	// anything to do?
	if len(mappings) == 0 {
		return objects, errCount
	}

	// find all objects that match any of the rules
	for _, obj := range objs {
		// prepare object clone
		objClone := utils.CloneProtoMsg(obj)
		if objClone.Metadata.Labels == nil {
			objClone.Metadata.Labels = map[string]string{}
		}

		// apply each mapping in order.
		matched := false
		hadError := false
		for _, mapping := range mappings {
			match, err := applyMappingToObject(mapping, objClone.GetSpec(), objClone.Metadata.Labels)
			if err != nil {
				logger.DebugContext(ctx, "failed to apply label due to template error", "name", obj.GetMetadata().GetName(), "error", err)
				errCount++
				hadError = true
				break
			}
			if match {
				matched = true
			}
		}

		if !hadError && matched {
			objects = append(objects, objClone)
		}
	}

	return objects, errCount
}

type DbNameFilter func(string) bool

// CalculateDatabaseNameFilter returns a function that checks if the given database name will be accepted by any of the rules.
// This can be used to skip the import from the given database if we can tell already that none of the objects will be accepted.
func CalculateDatabaseNameFilter(rules []*dbobjectimportrulev1.DatabaseObjectImportRule, database types.Database) DbNameFilter {
	var patterns []string
	for _, rule := range filterRulesForDatabase(rules, database) {
		spec := rule.GetSpec()
		for _, mapping := range spec.Mappings {
			names := mapping.GetScope().GetDatabaseNames()
			// empty list of database names means "any database name".
			if len(names) == 0 {
				return func(_ string) bool {
					return true
				}
			}
			patterns = append(patterns, names...)
		}
	}
	return func(dbName string) bool {
		return matchAny(patterns, dbName)
	}
}

// filterRulesForDatabase returns the set of rules whose labels match the database used as an argument.
func filterRulesForDatabase(rules []*dbobjectimportrulev1.DatabaseObjectImportRule, database types.Database) []*dbobjectimportrulev1.DatabaseObjectImportRule {
	var out []*dbobjectimportrulev1.DatabaseObjectImportRule
	for _, rule := range rules {
		dbLabels := make(types.Labels)
		mapLabel := label.ToMap(rule.Spec.GetDatabaseLabels())
		for k, v := range mapLabel {
			dbLabels[k] = v
		}

		if ok, _, _ := services.MatchLabels(dbLabels, database.GetAllLabels()); ok {
			out = append(out, rule)
		}
	}
	return out
}

// validateTemplate evaluates the template, checking for potential errors.
func validateTemplate(template string) error {
	_, err := evalTemplate(template, &dbobjectv1.DatabaseObjectSpec{})
	return trace.Wrap(err)
}

type eval interface {
	eval(spec *dbobjectv1.DatabaseObjectSpec) (string, error)
}

type literal struct {
	text string
}

func (l literal) eval(_ *dbobjectv1.DatabaseObjectSpec) (string, error) {
	return l.text, nil
}

type expression struct {
	text string
}

func (e expression) eval(spec *dbobjectv1.DatabaseObjectSpec) (string, error) {
	type evaluationEnv struct{}

	envVar := map[string]typical.Variable{
		"true":  true,
		"false": false,
		"obj": typical.DynamicMapFunction(func(e evaluationEnv, key string) (string, error) {
			switch key {
			case "protocol":
				return spec.GetProtocol(), nil
			case "database_service_name":
				return spec.GetDatabaseServiceName(), nil
			case "object_kind":
				return spec.GetObjectKind(), nil
			case "database":
				return spec.GetDatabase(), nil
			case "schema":
				return spec.GetSchema(), nil
			case "name":
				return spec.GetName(), nil
			}

			return "", trace.NotFound("key %v not found", key)
		}),
	}

	parser, err := typical.NewParser[evaluationEnv, string](typical.ParserSpec[evaluationEnv]{Variables: envVar})
	if err != nil {
		return "", trace.Wrap(err)
	}

	expr, err := parser.Parse(e.text)
	if err != nil {
		return "", trace.Wrap(err)
	}

	text, err := expr.Evaluate(evaluationEnv{})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return text, nil
}

var reVariable = regexp.MustCompile(
	// prefix is anything that is not { or }
	`^(?P<prefix>[^}{]*)` +
		// variable is anything in brackets {{}} that is not { or }
		`{{(?P<expression>\s*[^}{]*\s*)}}` +
		// suffix is anything that is not { or }
		`(?P<suffix>[^}{]*)$`,
)

// splitExpression splits the template into several parts, to be evaluated separately.
func splitExpression(value string) ([]eval, error) {
	match := reVariable.FindStringSubmatch(value)
	if len(match) == 0 {
		if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return nil, trace.BadParameter(
				"%q is using template brackets '{{' or '}}', however expression does not parse, make sure the format is {{expression}}",
				value,
			)
		}
		return []eval{literal{text: strings.TrimSpace(value)}}, nil
	}

	return []eval{
		literal{text: strings.TrimLeftFunc(match[1], unicode.IsSpace)},
		expression{text: match[2]},
		literal{text: strings.TrimRightFunc(match[3], unicode.IsSpace)},
	}, nil
}

func evalTemplate(template string, spec *dbobjectv1.DatabaseObjectSpec) (string, error) {
	chunks, err := splitExpression(template)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var sb strings.Builder

	for _, chunk := range chunks {
		text, err := chunk.eval(spec)
		if err != nil {
			return "", trace.Wrap(err)
		}
		sb.WriteString(text)
	}

	return sb.String(), nil
}

func applyMappingToObject(mapping *dbobjectimportrulev1.DatabaseObjectImportRuleMapping, spec *dbobjectv1.DatabaseObjectSpec, labels map[string]string) (bool, error) {
	// the matching is applied to the object spec; existing object labels does not matter
	if !databaseObjectScopeMatch(mapping.GetScope(), spec) {
		return false, nil
	}
	if !databaseObjectImportMatch(mapping.GetMatch(), spec) {
		return false, nil
	}

	for key, value := range mapping.AddLabels {
		out, err := evalTemplate(value, spec)
		if err != nil {
			return false, trace.Wrap(err)
		}
		labels[key] = out
	}

	return true, nil
}

func matchPattern(pattern, value string) bool {
	re, err := libutils.CompileExpression(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

func matchAny(patterns []string, value string) bool {
	return slices.ContainsFunc(patterns, func(pattern string) bool {
		return matchPattern(pattern, value)
	})
}

func databaseObjectScopeMatch(scope *dbobjectimportrulev1.DatabaseObjectImportScope, spec *dbobjectv1.DatabaseObjectSpec) bool {
	// require at least one match if there are any names to match against.
	if len(scope.GetDatabaseNames()) > 0 && !matchAny(scope.GetDatabaseNames(), spec.GetDatabase()) {
		return false
	}
	if len(scope.GetSchemaNames()) > 0 && !matchAny(scope.GetSchemaNames(), spec.GetSchema()) {
		return false
	}
	return true
}

func databaseObjectImportMatch(match *dbobjectimportrulev1.DatabaseObjectImportMatch, spec *dbobjectv1.DatabaseObjectSpec) bool {
	switch spec.GetObjectKind() {
	case ObjectKindTable:
		return matchAny(match.GetTableNames(), spec.GetName())
	case ObjectKindView:
		return matchAny(match.GetViewNames(), spec.GetName())
	case ObjectKindProcedure:
		return matchAny(match.GetProcedureNames(), spec.GetName())
	default:
		// unknown object kind
		return false
	}

}

const (
	ObjectKindTable     = "table"
	ObjectKindView      = "view"
	ObjectKindProcedure = "procedure"
)
