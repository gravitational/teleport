package summarizer

import (
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewInferenceModel creates a new InferenceModel resource with the given name
// and spec.
func NewInferenceModel(name string, spec *summarizerv1.InferenceModelSpec) *summarizerv1.InferenceModel {
	return &summarizerv1.InferenceModel{
		Kind:    types.KindInferenceModel,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}
}

// NewInferenceSecret creates a new InferenceSecret resource with the given name
// and spec.
func NewInferenceSecret(name string, spec *summarizerv1.InferenceSecretSpec) *summarizerv1.InferenceSecret {
	return &summarizerv1.InferenceSecret{
		Kind:    types.KindInferenceSecret,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}
}

// NewInferencePolicy creates a new InferencePolicy resource with the given name
// and spec.
func NewInferencePolicy(name string, spec *summarizerv1.InferencePolicySpec) *summarizerv1.InferencePolicy {
	return &summarizerv1.InferencePolicy{
		Kind:    types.KindInferencePolicy,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}
}
