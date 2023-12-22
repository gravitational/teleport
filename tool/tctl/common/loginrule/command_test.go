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
				"logins": {
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
