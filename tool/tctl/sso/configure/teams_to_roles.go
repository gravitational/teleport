// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
