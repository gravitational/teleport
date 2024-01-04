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

package configure

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// teamsToRolesParser parsers 'name,value,role1,role2,...' values into types.AttributeMapping entries. Cumulative, can handle multiple entries.
type teamsToRolesParser struct {
	mappings *[]types.TeamRolesMapping
}

func (p *teamsToRolesParser) String() string {
	return fmt.Sprintf("%q", p.mappings)
}

func (p *teamsToRolesParser) Set(s string) error {
	splits := strings.Split(s, ",")

	if len(splits) < 3 {
		return trace.BadParameter("Too few elements separated with comma. use syntax: 'organization,team,role1,role2,...'.")
	}

	org := splits[0]
	team := splits[1]
	roles := splits[2:]

	mapping := types.TeamRolesMapping{
		Organization: org,
		Team:         team,
		Roles:        roles,
	}

	*p.mappings = append(*p.mappings, mapping)

	return nil
}

// IsCumulative returns true if flag is repeatable. This is checked by kingpin library.
func (p *teamsToRolesParser) IsCumulative() bool {
	return true
}

// newTeamsToRolesParser returns a cumulative flag parser for []types.TeamMapping.
func newTeamsToRolesParser(field *[]types.TeamRolesMapping) kingpin.Value {
	return &teamsToRolesParser{mappings: field}
}
