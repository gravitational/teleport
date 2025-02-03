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
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
)

// PermissionSet represents calculated database permissions.
// Each permission is a key in the map, whereas the objects it applies to are elements in the value slice.
type PermissionSet map[string][]*dbobjectv1.DatabaseObject

type GetDatabasePermissions interface {
	GetDatabasePermissions(database types.Database) (allow types.DatabasePermissions, deny types.DatabasePermissions, err error)
}

func databasePermissionMatch(perm types.DatabasePermission, obj *dbobjectv1.DatabaseObject) bool {
	ok, _, _ := services.MatchLabels(perm.Match, obj.Metadata.Labels)
	return ok
}

// CountObjectKinds counts the number of different database object kinds. It returns a map and a string representation of it.
func CountObjectKinds(objs []*dbobjectv1.DatabaseObject) (string, map[string]int) {
	var fragments []string

	counts := utils.CountBy(objs, func(obj *dbobjectv1.DatabaseObject) string {
		return obj.GetSpec().ObjectKind
	})
	kinds := slices.Sorted(maps.Keys(counts))
	for _, kind := range kinds {
		fragments = append(fragments, fmt.Sprintf("%v:%v", kind, counts[kind]))
	}

	if len(fragments) == 0 {
		return "none", counts
	}
	return strings.Join(fragments, ", "), counts
}

// SummarizePermissions summarizes permissions for logging.
func SummarizePermissions(perms PermissionSet) (string, []events.DatabasePermissionEntry) {
	eventData := map[string]map[string]int{}
	var fragments []string

	permNames := slices.Sorted(maps.Keys(perms))
	for _, perm := range permNames {
		objects := perms[perm]
		countText, countMap := CountObjectKinds(objects)
		fragments = append(fragments, fmt.Sprintf("%q: %v objects (%v)", perm, len(objects), countText))
		eventData[perm] = countMap
	}

	var entries []events.DatabasePermissionEntry
	for perm, counts := range eventData {
		countsConv := map[string]int32{}
		for k, cnt := range counts {
			countsConv[k] = int32(cnt)
		}
		entries = append(entries, events.DatabasePermissionEntry{
			Permission: perm,
			Counts:     countsConv,
		})
	}

	if len(fragments) == 0 {
		return "(none)", entries
	}

	return strings.Join(fragments, ", "), entries
}

// CalculatePermissions calculates the effective permissions for a set of database objects based on the provided allow and deny permissions.
func CalculatePermissions(getter GetDatabasePermissions, database types.Database, objs []*dbobjectv1.DatabaseObject) (PermissionSet, error) {
	allow, deny, err := getter.GetDatabasePermissions(database)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := map[string][]*dbobjectv1.DatabaseObject{}

	for _, obj := range objs {
		permsToAdd := map[string]string{}

		// add allowed permissions
		for _, perm := range allow {
			if databasePermissionMatch(perm, obj) {
				for _, permission := range perm.Permissions {
					permsToAdd[strings.TrimSpace(strings.ToUpper(permission))] = permission
				}
			}
		}

		// remove denied permissions
		for _, perm := range deny {
			// check if there is any work left to do
			if len(permsToAdd) == 0 {
				break
			}
			if databasePermissionMatch(perm, obj) {
				for _, permission := range perm.Permissions {
					// wildcard clears the permissions
					if permission == types.Wildcard {
						clear(permsToAdd)
						break
					}

					delete(permsToAdd, strings.TrimSpace(strings.ToUpper(permission)))
				}
			}
		}

		for _, perm := range permsToAdd {
			out[perm] = append(out[perm], obj)
		}
	}

	return out, nil
}
