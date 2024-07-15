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

// attributesToRolesParser parsers 'name,value,role1,role2,...' values into types.AttributeMapping entries. Cumulative, can handle multiple entries.
type attributesToRolesParser struct {
	mappings *[]types.AttributeMapping
}

func (a *attributesToRolesParser) String() string {
	return fmt.Sprintf("%v", a.mappings)
}

func (a *attributesToRolesParser) Set(s string) error {
	splits := strings.Split(s, ",")

	if len(splits) < 3 {
		return trace.BadParameter("Too few elements separated with comma. use syntax: 'name,value,role1,role2,...'.")
	}

	name := splits[0]
	value := splits[1]
	roles := splits[2:]

	mapping := types.AttributeMapping{
		Name:  name,
		Value: value,
		Roles: roles,
	}

	*a.mappings = append(*a.mappings, mapping)

	return nil
}

// IsCumulative returns true if flag is repeatable. This is checked by kingpin library.
func (a *attributesToRolesParser) IsCumulative() bool {
	return true
}

// NewAttributesToRolesParser returns new parser, which can parse strings such as 'name,value,role1,role2,...' into types.AttributeMapping entries. It can be called multiple times, making the flag repeatable.
func NewAttributesToRolesParser(field *[]types.AttributeMapping) kingpin.Value {
	return &attributesToRolesParser{mappings: field}
}
