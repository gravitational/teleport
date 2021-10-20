/*
Copyright 2020-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package services

import (
	"time"

	"gopkg.in/check.v1"

	"github.com/gravitational/teleport/api/v8/defaults"
	"github.com/gravitational/teleport/api/v8/types"
	"github.com/gravitational/teleport/lib/fixtures"
)

type UserTokenSuite struct{}

var _ = check.Suite(&UserTokenSuite{})

func (r *UserTokenSuite) TestUnmarshal(c *check.C) {
	created, err := time.Parse(time.RFC3339, "2020-01-14T18:52:39.523076855Z")
	c.Assert(err, check.IsNil)

	type testCase struct {
		description string
		input       string
		expected    types.UserToken
	}

	testCases := []testCase{
		{
			description: "simple case",
			input: `
        {
          "kind": "user_token",
          "version": "v3",
          "metadata": {
            "name": "tokenId"
          },
          "spec": {
            "user": "example@example.com",
            "created": "2020-01-14T18:52:39.523076855Z",
            "url": "https://localhost"
          }
        }
      `,
			expected: &types.UserTokenV3{
				Kind:    types.KindUserToken,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "tokenId",
					Namespace: defaults.Namespace,
				},
				Spec: types.UserTokenSpecV3{
					Created: created,
					User:    "example@example.com",
					URL:     "https://localhost",
				},
			},
		},
	}

	for _, tc := range testCases {
		comment := check.Commentf("test case %q", tc.description)
		out, err := UnmarshalUserToken([]byte(tc.input))
		c.Assert(err, check.IsNil, comment)
		fixtures.DeepCompare(c, tc.expected, out)
		data, err := MarshalUserToken(out)
		c.Assert(err, check.IsNil, comment)
		out2, err := UnmarshalUserToken(data)
		c.Assert(err, check.IsNil, comment)
		fixtures.DeepCompare(c, tc.expected, out2)
	}
}
