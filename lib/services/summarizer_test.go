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
	p := apisummarizer.NewInferencePolicy("my-policy", &summarizerv1.InferencePolicySpec{
		Kinds:  []string{"ssh", "k8s", "db"},
		Filter: `equals(resource.metadata.labels["env"], "prod") || equals(user.metadata.name, "admin")`,
		Model:  "my-model",
	})
	require.NoError(t, ValidateInferencePolicy(p))

	// Empty filter should also be valid.
	p.Spec.Filter = ""
	require.NoError(t, ValidateInferencePolicy(p))

	// Broken filter expression.
	p.Spec.Filter = "equals("
	err := ValidateInferencePolicy(p)
	assert.ErrorContains(t, err, "spec.filter has to be a valid predicate")

	// Verify that errors reported from the api/types package are also included.
	p = apisummarizer.NewInferencePolicy("my-policy", &summarizerv1.InferencePolicySpec{
		Model: "my-model",
	})
	err = ValidateInferencePolicy(p)
	assert.ErrorContains(t, err, "spec.kinds are required")
}
