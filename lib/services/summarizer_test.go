// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
)

func TestValidateInferencePolicy(t *testing.T) {
	t.Parallel()

	allKinds := []string{"ssh", "k8s", "db"}

	cases := []struct {
		name         string
		kinds        []string
		filter       string
		errorMessage string
	}{
		{name: "valid empty filter", kinds: allKinds, filter: ""},
		{name: "valid user filter", kinds: allKinds, filter: `contains(user.spec.roles, "admin")`},
		{name: "valid server filter", kinds: allKinds, filter: `equals(resource.spec.hostname, "node1")`},
		{name: "valid db filter", kinds: allKinds, filter: `equals(resource.spec.protocol, "postgres")`},
		{name: "valid kube filter", kinds: allKinds, filter: `resource.metadata.labels["env"] == "prod"`},
		{name: "valid shell session filter", kinds: allKinds, filter: `contains(session.participants, "joe")`},
		{name: "valid db session filter", kinds: allKinds, filter: `session.db_protocol == "postgres"`},

		{
			name:         "invalid kinds",
			kinds:        nil,
			errorMessage: "spec.kinds are required",
		},
		{
			name:         "invalid filter syntax",
			kinds:        allKinds,
			filter:       "equals(resource.metadata.name, ",
			errorMessage: "spec.filter has to be a valid predicate",
		},
		{
			name:         "invalid user filter field",
			kinds:        allKinds,
			filter:       `user.metadata.foo == "bar"`,
			errorMessage: "field name foo is not found",
		},
		{
			name:         "invalid resource filter field",
			kinds:        allKinds,
			filter:       `resource.spec.foo == "bar"`,
			errorMessage: "field name spec.foo is not found",
		},
		{
			name:         "invalid session filter field",
			kinds:        allKinds,
			filter:       `session.foo == "bar"`,
			errorMessage: "field name foo is not found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := apisummarizer.NewInferencePolicy("my-policy", &summarizerv1.InferencePolicySpec{
				Kinds:  tc.kinds,
				Filter: tc.filter,
				Model:  "my-model",
			})
			err := ValidateInferencePolicy(p)
			if tc.errorMessage == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.errorMessage)
			}
		})
	}
}
