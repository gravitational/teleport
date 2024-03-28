// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package flags

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// claimsToRolesParser parsers 'claim,value,role1,role2,...' values into types.ClaimMapping entries. Cumulative, can handle multiple entries.
type claimsToRolesParser struct {
	mappings *[]types.ClaimMapping
}

func (a *claimsToRolesParser) String() string {
	return fmt.Sprintf("%v", a.mappings)
}

func (a *claimsToRolesParser) Set(s string) error {
	splits := strings.Split(s, ",")

	if len(splits) < 3 {
		return trace.BadParameter("Too few elements separated with comma. Syntax: 'claim,value,role1,role2,...'.")
	}

	claim := splits[0]
	value := splits[1]
	roles := splits[2:]

	mapping := types.ClaimMapping{
		Claim: claim,
		Value: value,
		Roles: roles,
	}

	*a.mappings = append(*a.mappings, mapping)

	return nil
}

// IsCumulative returns true if flag is repeatable. This is checked by kingpin library.
func (a *claimsToRolesParser) IsCumulative() bool {
	return true
}

// NewClaimsToRolesParser returns new parser, which can parse strings such as 'claim,value,role1,role2,...' into types.ClaimMapping entries. It can be called multiple times, making the flag repeatable.
func NewClaimsToRolesParser(field *[]types.ClaimMapping) kingpin.Value {
	return &claimsToRolesParser{mappings: field}
}
