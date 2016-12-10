/*
Copyright 2015 Gravitational, Inc.

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
	"encoding/json"
	"testing"
	"time"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestRoleParsing(t *testing.T) { TestingT(t) }

type RoleSuite struct {
}

var _ = Suite(&RoleSuite{})

func (s *RoleSuite) TestRoleParse(c *C) {
	testCases := []struct {
		in    string
		role  RoleResource
		error error
	}{
		{
			in:    ``,
			error: trace.BadParameter("empty input"),
		},
		{
			in:    `{}`,
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			in:    `{"kind": "role"}`,
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			in: `{"kind": "role", "metadata": {"name": "name1"}, "spec": {}}`,
			role: RoleResource{
				Kind:    KindRole,
				Version: V1,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: DefaultNamespace,
				},
				Spec: RoleSpec{},
			},
		},
		{
			in: `{
              "kind": "role", 
              "metadata": {"name": "name1"}, 
              "spec": {
                 "max_session_ttl": "20h",
                 "node_labels": {"a": "b"},
                 "namespaces": ["system", "default"],
                 "resources": {
                    "role": ["read", "write"]
                 }
              }
            }`,
			role: RoleResource{
				Kind:    KindRole,
				Version: V1,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: DefaultNamespace,
				},
				Spec: RoleSpec{
					MaxSessionTTL: Duration{20 * time.Hour},
					NodeLabels:    map[string]string{"a": "b"},
					Namespaces:    []string{"system", "default"},
					Resources:     map[string][]string{"role": {ActionRead, ActionWrite}},
				},
			},
		},
		{
			in: `kind: role
metadata:
  name: name1
spec:
  max_session_ttl: 20h
  node_labels:
    a: b
  namespaces: ["system", "default"]
  resources:
    role: [read, write]
`,
			role: RoleResource{
				Kind:    KindRole,
				Version: V1,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: DefaultNamespace,
				},
				Spec: RoleSpec{
					MaxSessionTTL: Duration{20 * time.Hour},
					NodeLabels:    map[string]string{"a": "b"},
					Namespaces:    []string{"system", "default"},
					Resources:     map[string][]string{"role": {ActionRead, ActionWrite}},
				},
			},
		},
	}
	for i, tc := range testCases {
		comment := Commentf("test case %v", i)
		role, err := UnmarshalRoleResource([]byte(tc.in))
		if tc.error != nil {
			c.Assert(err, NotNil, comment)
		} else {
			c.Assert(err, IsNil, comment)
			c.Assert(*role, DeepEquals, tc.role, comment)

			out, err := json.Marshal(*role)
			c.Assert(err, IsNil, comment)

			role2, err := UnmarshalRoleResource(out)
			c.Assert(err, IsNil, comment)
			c.Assert(*role2, DeepEquals, tc.role, comment)
		}
	}
}
