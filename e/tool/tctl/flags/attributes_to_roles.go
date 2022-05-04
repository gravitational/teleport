package flags

import (
	"fmt"
	"strings"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// attributesToRolesParser parsers 'name,value,role1,role2,...' values into types.AttributeMapping entries. Cumulative, can handle multiple entries.
type attributesToRolesParser []types.AttributeMapping

func (a *attributesToRolesParser) String() string {
	return fmt.Sprintf("%q", (*[]types.AttributeMapping)(a))
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

	*a = append(*a, mapping)

	return nil
}

func (a *attributesToRolesParser) IsCumulative() bool {
	return true
}

// NewAttributesToRolesParser returns a cumulative flag parser for []types.AttributeMapping.
func NewAttributesToRolesParser(field *[]types.AttributeMapping) kingpin.Value {
	return (*attributesToRolesParser)(field)
}
