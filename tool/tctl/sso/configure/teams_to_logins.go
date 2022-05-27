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

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// teamsToLoginsParser parsers 'name,value,role1,role2,...' values into types.AttributeMapping entries. Cumulative, can handle multiple entries.
type teamsToLoginsParser struct {
	mappings *[]types.TeamMapping
}

func (p *teamsToLoginsParser) String() string {
	return fmt.Sprintf("%q", p.mappings)
}

func (p *teamsToLoginsParser) Set(s string) error {
	splits := strings.Split(s, ",")

	if len(splits) < 3 {
		return trace.BadParameter("Too few elements separated with comma. use syntax: 'name,value,role1,role2,...'.")
	}

	name := splits[0]
	value := splits[1]
	roles := splits[2:]

	mapping := types.TeamMapping{
		Organization: name,
		Team:         value,
		Logins:       roles, // note: logins is legacy name, 'roles' is accurate now.
	}

	*p.mappings = append(*p.mappings, mapping)

	return nil
}

// IsCumulative returns true if flag is repeatable. This is checked by kingpin library.
func (p *teamsToLoginsParser) IsCumulative() bool {
	return true
}

// newTeamsToLoginsParser returns a cumulative flag parser for []types.TeamMapping.
func newTeamsToLoginsParser(field *[]types.TeamMapping) kingpin.Value {
	return &teamsToLoginsParser{mappings: field}
}
