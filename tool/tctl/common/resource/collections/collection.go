/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package collections

import (
	"fmt"
	"github.com/gravitational/trace"
	"io"
	"slices"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
)

type ResourceCollection interface {
	WriteText(w io.Writer, verbose bool) error
	Resources() []types.Resource
}

// namedResourceCollection is an implementation of [ResourceCollection] that
// displays resources in a table as a list of names and nothing else.
type namedResourceCollection []types.Resource

func NewNamedResourceCollection(resources []types.Resource) ResourceCollection {
	return namedResourceCollection(resources)
}

// resources implements [ResourceCollection].
func (c namedResourceCollection) Resources() []types.Resource {
	return c
}

// writeText implements [ResourceCollection].
func (c namedResourceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, override := range c {
		t.AddRow([]string{override.GetName()})
	}
	return trace.Wrap(t.WriteTo(w))
}

func printActions(rules []types.Rule) string {
	pairs := []string{}
	for _, rule := range rules {
		pairs = append(pairs, fmt.Sprintf("%v:%v", strings.Join(rule.Resources, ","), strings.Join(rule.Verbs, ",")))
	}
	return strings.Join(pairs, ",")
}

func PrintMetadataLabels(labels map[string]string) string {
	pairs := []string{}
	for key, value := range labels {
		pairs = append(pairs, fmt.Sprintf("%v=%v", key, value))
	}
	return strings.Join(pairs, ",")
}

func printNodeLabels(labels types.Labels) string {
	pairs := []string{}
	for key, values := range labels {
		if key == types.Wildcard {
			return "<all nodes>"
		}
		pairs = append(pairs, fmt.Sprintf("%v=%v", key, values))
	}
	return strings.Join(pairs, ",")
}

func formatTeamsToLogins(mappings []types.TeamMapping) string {
	var result []string
	for _, m := range mappings {
		result = append(result, fmt.Sprintf("@%v/%v: %v",
			m.Organization, m.Team, strings.Join(m.Logins, ", ")))
	}
	return strings.Join(result, ", ")
}

func WriteJSON(c ResourceCollection, w io.Writer) error {
	return utils.WriteJSONArray(w, c.Resources())
}

func WriteYAML(c ResourceCollection, w io.Writer) error {
	return utils.WriteYAML(w, c.Resources())
}

func printSortedStringSlice(s []string) string {
	s = slices.Clone(s)
	slices.Sort(s)
	return strings.Join(s, ",")
}
