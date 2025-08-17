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
