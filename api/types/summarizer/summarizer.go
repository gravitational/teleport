// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package summarizer

import (
	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
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

func ValidateInferenceModel(m *summarizerv1.InferenceModel) error {
	switch {
	case m == nil:
		return trace.BadParameter("inference model is nil")
	case m.GetKind() != types.KindInferenceModel:
		return trace.BadParameter("kind must be %s, got %s", types.KindInferenceModel, m.GetKind())
	case m.GetSubKind() != "":
		return trace.BadParameter("subkind must be empty")
	case m.GetVersion() != types.V1:
		return trace.BadParameter("unsupported version %s", m.GetVersion())

	case m.GetMetadata() == nil:
		return trace.BadParameter("metadata is required")
	case m.GetMetadata().GetName() == "":
		return trace.BadParameter("metadata.name is required")

	case m.GetSpec() == nil:
		return trace.BadParameter("spec is required")
	}

	provider := m.GetSpec().GetProvider()
	// if provider != nil {
	// 	return trace.BadParameter("missing or unsupported provider")
	// }
	switch p := provider.(type) {
	case *summarizerv1.InferenceModelSpec_Openai:
		if p.Openai.GetOpenaiModelId() == "" {
			return trace.BadParameter("spec.openai.openai_model_id is required")
		}
	default:
		return trace.BadParameter("unsupported inference provider %T", p)
	}

	return nil
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

func ValidateInferenceSecret(s *summarizerv1.InferenceSecret) error {
	switch {
	case s == nil:
		return trace.BadParameter("inference secret is nil")
	case s.GetKind() != types.KindInferenceSecret:
		return trace.BadParameter("kind must be %s, got %s", types.KindInferenceSecret, s.GetKind())
	case s.GetSubKind() != "":
		return trace.BadParameter("subkind must be empty")
	case s.GetVersion() != types.V1:
		return trace.BadParameter("unsupported version: %s", s.GetVersion())

	case s.GetMetadata() == nil:
		return trace.BadParameter("metadata is required")
	case s.GetMetadata().GetName() == "":
		return trace.BadParameter("metadata.name is required")

	case s.GetSpec() == nil:
		return trace.BadParameter("spec is required")
	case s.GetSpec().GetValue() == "":
		return trace.BadParameter("spec.value is required")
	}

	return nil
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

func ValidateInferencePolicy(p *summarizerv1.InferencePolicy) error {
	switch {
	case p == nil:
		return trace.BadParameter("inference policy is nil")
	case p.GetKind() != types.KindInferencePolicy:
		return trace.BadParameter("kind must be %s, got %s", types.KindInferencePolicy, p.GetKind())
	case p.GetSubKind() != "":
		return trace.BadParameter("subkind must be empty")
	case p.GetVersion() != types.V1:
		return trace.BadParameter("unsupported version: %s", p.GetVersion())

	case p.GetMetadata() == nil:
		return trace.BadParameter("metadata is required")
	case p.GetMetadata().GetName() == "":
		return trace.BadParameter("metadata.name is required")

	case p.GetSpec() == nil:
		return trace.BadParameter("spec is required")
	}

	kinds := p.GetSpec().GetKinds()
	if len(kinds) == 0 {
		return trace.BadParameter("spec.kinds are required")
	}

	parser, err := services.NewWhereParser(&services.Context{})
	if err != nil {
		return trace.Wrap(err, "spec.filter has to be a valid predicate")
	}
	_, err = parser.Parse(p.GetSpec().Filter)
	if err != nil {
		return trace.Wrap(err, "spec.filter has to be a valid predicate")
	}

	return nil
}
