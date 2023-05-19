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
