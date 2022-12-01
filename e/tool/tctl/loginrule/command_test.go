package loginrule

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
)

// TestParseLoginRules is a simple test that multiple login rules can be parsed
// from a single yaml file split into multiple documents. More comprehensive
// parse testing can be found in TestUnmarshalLoginRule.
func TestParseLoginRules(t *testing.T) {
	const fileContents = `kind: login_rule
version: v1
metadata:
  name: test_rule_1
spec:
  priority: 0
  traits_map:
    logins: [one]
---
kind: login_rule
version: v1
metadata:
  name: test_rule_2
spec:
  priority: 1
  traits_map:
    logins: [two]`

	loginRules, err := parseLoginRules(strings.NewReader(fileContents))
	require.NoError(t, err)

	makeRule := func(name, login string, priority int32) *loginrulepb.LoginRule {
		return &loginrulepb.LoginRule{
			Metadata: &types.Metadata{
				Name:      name,
				Namespace: "default",
			},
			Version:  "v1",
			Priority: priority,
			TraitsMap: map[string]*wrappers.StringValues{
				"logins": &wrappers.StringValues{
					Values: []string{login},
				},
			},
		}
	}

	require.Equal(t, []*loginrulepb.LoginRule{
		makeRule("test_rule_1", "one", 0),
		makeRule("test_rule_2", "two", 1),
	}, loginRules, "parsed login rules do not match what was expected")
}
