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
