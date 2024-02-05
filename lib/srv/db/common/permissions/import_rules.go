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

package permissions

import (
	"regexp"
	"sort"
	"time"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
	utils2 "github.com/gravitational/teleport/lib/utils"
)

// ApplyDatabaseObjectImportRules applies the given set of rules onto a set of objects coming from a same database.
// Returns a fresh copy of a subset of supplied objects, filtered and modified.
// For the object to be returned, it must match at least one rule.
// The modification consists of application of extra labels, per matching mappings.
func ApplyDatabaseObjectImportRules(rules []*dbobjectimportrulev1.DatabaseObjectImportRule, database types.Database, objs []*dbobjectv1.DatabaseObject) []*dbobjectv1.DatabaseObject {
	// sort: rules with higher priorities are applied last.
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Spec.Priority < rules[j].Spec.Priority
	})

	// filter rules: keep those with matching labels
	// we only need mappings from the rules, so extract those.
	var mappings []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping
	for _, rule := range rules {
		// TODO(Tener) consider expanding rule.Spec.DbLabels into types.Labels
		dbLabels := make(types.Labels)
		for k, v := range rule.Spec.DbLabels {
			dbLabels[k] = []string{v}
		}

		if ok, _, _ := services.MatchLabels(dbLabels, database.GetAllLabels()); ok {
			mappings = append(mappings, rule.Spec.Mappings...)
		}
	}

	// anything to do?
	if len(mappings) == 0 {
		return nil
	}

	var out []*dbobjectv1.DatabaseObject

	// find all objects that match any of the rules
	for _, obj := range objs {
		var objClone *dbobjectv1.DatabaseObject

		// apply each mapping in order.
		for _, mapping := range mappings {
			// the matching is applied to the object spec; existing object labels does not matter
			if !databaseObjectScopeMatch(mapping.Scope, obj.GetSpec()) {
				continue
			}
			if databaseObjectImportMatch(mapping.Match, obj.GetSpec()) {
				if objClone == nil {
					objClone = utils.CloneProtoMsg(obj)
				}

				// mapping applies additional labels
				labels := objClone.Metadata.Labels
				if labels == nil {
					labels = map[string]string{}
				}
				for k, v := range mapping.AddLabels {
					labels[k] = v
				}
				objClone.Metadata.Labels = labels
			}
		}

		if objClone != nil {
			out = append(out, objClone)
		}
	}

	return out
}

func matchPattern(pattern, value string) bool {
	matched, _ := regexp.MatchString("^"+utils2.GlobToRegexp(pattern)+"$", value)
	return matched
}

func matchAny(patterns []string, value string) bool {
	return utils.Any(patterns, func(pattern string) bool {
		return matchPattern(pattern, value)
	})
}

func databaseObjectScopeMatch(scope *dbobjectimportrulev1.DatabaseObjectImportScope, spec *dbobjectv1.DatabaseObjectSpec) bool {
	// empty scope matches anything
	if scope == nil {
		return true
	}
	return matchAny(scope.DatabaseNames, spec.Database) && matchAny(scope.SchemaNames, spec.Schema)
}

func databaseObjectImportMatch(match *dbobjectimportrulev1.DatabaseObjectImportMatch, spec *dbobjectv1.DatabaseObjectSpec) bool {
	switch spec.ObjectKind {
	case ObjectKindTable:
		return len(match.TableNames) > 0 && matchAny(match.TableNames, spec.Name)
	case ObjectKindView:
		return len(match.ViewNames) > 0 && matchAny(match.ViewNames, spec.Name)
	case ObjectKindProcedure:
		return len(match.ProcedureNames) > 0 && matchAny(match.ProcedureNames, spec.Name)
	default:
		// unknown object kind
		return false
	}

}

const (
	ObjectKindTable     = "table"
	ObjectKindView      = "view"
	ObjectKindProcedure = "procedure"

	ObjectTTL = 15 * time.Minute
)
