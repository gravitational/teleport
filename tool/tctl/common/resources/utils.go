/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resources

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/iterutils"
	"github.com/gravitational/teleport/lib/services"
)

const managedByStaticDeleteMsg = `This resource is managed by static configuration. In order to reset it to defaults, remove relevant configuration from teleport.yaml and restart the servers.`

func checkCreateResourceWithOrigin(storedRes types.ResourceWithOrigin, resDesc string, force, confirm bool) error {
	if exists := (storedRes.Origin() != types.OriginDefaults); exists && !force {
		return trace.AlreadyExists("non-default %s already exists", resDesc)
	}
	return checkUpdateResourceWithOrigin(storedRes, resDesc, confirm)
}

func checkUpdateResourceWithOrigin(storedRes types.ResourceWithOrigin, resDesc string, confirm bool) error {
	managedByStatic := storedRes.Origin() == types.OriginConfigFile
	if managedByStatic && !confirm {
		return trace.BadParameter(`The %s resource is managed by static configuration. We recommend removing configuration from teleport.yaml, restarting the servers and trying this command again.

If you would still like to proceed, re-run the command with the --confirm flag.`, resDesc)
	}
	return nil
}

// AltResourceNameFunc is a func that returns an alternative name for a resource.
type AltResourceNameFunc[T types.ResourceWithLabels] func(T) string

// FilterByNameOrDiscoveredName filters resources by name or
// "discovered name". It prefers exact name filtering first - if none of the
// resource names match exactly (i.e. all of the resources are filtered out),
// then it retries and filters the resources by "discovered name" of resource
// name instead, which comes from an auto-discovery label.
func FilterByNameOrDiscoveredName[T types.ResourceWithLabels](resources []T, prefixOrName string, extra ...AltResourceNameFunc[T]) []T {
	// prefer exact names
	out := filterResourcesByName(resources, prefixOrName, extra...)
	if len(out) == 0 {
		// fallback to looking for discovered name label matches.
		out = filterByDiscoveredName(resources, prefixOrName)
	}
	return out
}

// filterResourcesByName filters resources by exact name match.
func filterResourcesByName[T types.ResourceWithLabels](resources []T, name string, altNameFns ...AltResourceNameFunc[T]) []T {
	return filterResources(resources, func(r T) bool {
		if r.GetName() == name {
			return true
		}
		for _, altName := range altNameFns {
			if altName(r) == name {
				return true
			}
		}
		return false
	})
}

// filterByDiscoveredName filters resources that have a "discovered name" label
// that matches the given name.
func filterByDiscoveredName[T types.ResourceWithLabels](resources []T, name string) []T {
	return filterResources(resources, func(r T) bool {
		discoveredName, ok := r.GetLabel(types.DiscoveredNameLabel)
		return ok && discoveredName == name
	})
}

func filterResources[T types.ResourceWithLabels](resources []T, keepFn func(T) bool) []T {
	return slices.Collect(iterutils.Filter(keepFn, slices.Values(resources)))
}

// GetOneResourceNameToDelete checks a list of resources to ensure there is
// exactly one resource name among them, and returns that name or an error.
// Heartbeat resources can have the same name but different host ID, so this
// still allows a user to delete multiple heartbeats of the same name, for
// example `$ tctl rm db_server/someDB`.
func GetOneResourceNameToDelete[T types.ResourceWithLabels](rs []T, ref services.Ref, resDesc string) (string, error) {
	seen := make(map[string]struct{})
	for _, r := range rs {
		seen[r.GetName()] = struct{}{}
	}
	switch len(seen) {
	case 1: // need exactly one.
		return rs[0].GetName(), nil
	case 0:
		return "", trace.NotFound("%v %q not found", resDesc, ref.Name)
	default:
		names := make([]string, 0, len(rs))
		for _, r := range rs {
			names = append(names, r.GetName())
		}
		msg := formatAmbiguousDeleteMessage(ref, resDesc, names)
		return "", trace.BadParameter("%s", msg)
	}
}

// formatAmbiguousDeleteMessage returns a formatted message when a user is
// attempting to delete multiple resources by an ambiguous prefix of the
// resource names.
func formatAmbiguousDeleteMessage(ref services.Ref, resDesc string, names []string) string {
	slices.Sort(names)
	// choose an actual resource for the example in the error.
	exampleRef := ref
	exampleRef.Name = names[0]
	return fmt.Sprintf(`%s matches multiple auto-discovered %vs:
%v

Use the full resource name that was generated by the Teleport Discovery service, for example:
$ tctl rm %s`,
		ref.String(), resDesc, strings.Join(names, "\n"), exampleRef.String())
}

// makeNamePredicate makes a predicate expression that can be used for
// filtering resources by name. Returns empty if name is empty.
func makeNamePredicate(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return fmt.Sprintf(`name == %q`, name)
}

// PrintMetadataLabels formats resource metadata labels as a key=value pair string.
func PrintMetadataLabels(labels map[string]string) string {
	var sb strings.Builder
	sb.Grow(len(labels) * 4)
	i := 0
	for key, value := range labels {
		sb.WriteString(key + "=" + value)

		if i < len(labels)-1 {
			sb.WriteRune(',')
		}
		i++
	}
	return sb.String()
}
