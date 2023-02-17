package main

import (
	"encoding/json"
	"fmt"

	login "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
)

func main() {
	p := login.LoginRule{
		Metadata: &types.Metadata{
			Name: "test",
		},
		Version:          "v1",
		TraitsExpression: "test-expression",
		TraitsMap: map[string]*wrappers.StringValues{
			"a": {Values: []string{"b", "c"}},
			"d": {Values: []string{"e", "f"}},
			"g": {Values: []string{"h"}},
		},
	}

	b, _ := json.MarshalIndent(p, "", "  ")
	fmt.Println(string(b))
}
