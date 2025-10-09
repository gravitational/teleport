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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestValidateInferenceModel(t *testing.T) {
	t.Parallel()
	validOpenAI := NewInferenceModel("my-model", &summarizerv1.InferenceModelSpec{
		Provider: &summarizerv1.InferenceModelSpec_Openai{
			Openai: &summarizerv1.OpenAIProvider{
				OpenaiModelId: "gpt-4o",
			},
		},
	})
	validBedrock := NewInferenceModel("my-model", &summarizerv1.InferenceModelSpec{
		Provider: &summarizerv1.InferenceModelSpec_Bedrock{
			Bedrock: &summarizerv1.BedrockProvider{
				BedrockModelId: "amazon.nova-lite-v1:0",
				Region:         "us-west-2",
			},
		},
	})
	require.NoError(t, ValidateInferenceModel(validOpenAI))
	require.NoError(t, ValidateInferenceModel(validBedrock))

	cases := []struct {
		base *summarizerv1.InferenceModel
		fn   func(m *summarizerv1.InferenceModel)
		msg  string
	}{
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.Kind = "other" },
			msg:  "kind must be inference_model, got other",
		},
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.SubKind = "foo" },
			msg:  "subkind must be empty",
		},
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.Version = "" },
			msg:  "version is required",
		},
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.Version = types.V2 },
			msg:  "unsupported version v2, supported: v1",
		},
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.Metadata = nil },
			msg:  "metadata is required",
		},
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.Metadata.Name = "" },
			msg:  "metadata.name is required",
		},
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.Metadata.Name = "teleport-cloud-default" },
			msg:  "metadata.name \"teleport-cloud-default\" is reserved",
		},
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.Spec = nil },
			msg:  "spec is required",
		},
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.Spec.Provider = nil },
			msg:  "missing or unsupported inference provider in spec, supported providers: openai",
		},
		{
			base: validOpenAI,
			fn:   func(m *summarizerv1.InferenceModel) { m.Spec.GetOpenai().OpenaiModelId = "" },
			msg:  "spec.openai.openai_model_id is required",
		},
		{
			base: validBedrock,
			fn:   func(m *summarizerv1.InferenceModel) { m.Spec.GetBedrock().BedrockModelId = "" },
			msg:  "spec.bedrock.bedrock_model_id is required",
		},
		{
			base: validBedrock,
			fn:   func(m *summarizerv1.InferenceModel) { m.Spec.GetBedrock().Region = "" },
			msg:  "spec.bedrock.region is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			m := proto.CloneOf(tc.base)
			tc.fn(m)
			assert.ErrorIs(t, ValidateInferenceModel(m), &trace.BadParameterError{Message: tc.msg})
		})
	}
}

func TestValidateInferenceSecret(t *testing.T) {
	t.Parallel()
	valid := NewInferenceSecret("my-secret", &summarizerv1.InferenceSecretSpec{
		Value: "super-secret-value",
	})
	require.NoError(t, ValidateInferenceSecret(valid))

	cases := []struct {
		fn  func(s *summarizerv1.InferenceSecret)
		msg string
	}{
		{
			fn:  func(s *summarizerv1.InferenceSecret) { s.Kind = "other" },
			msg: "kind must be inference_secret, got other",
		},
		{
			fn:  func(s *summarizerv1.InferenceSecret) { s.SubKind = "foo" },
			msg: "subkind must be empty",
		},
		{
			fn:  func(s *summarizerv1.InferenceSecret) { s.Version = "" },
			msg: "version is required",
		},
		{
			fn:  func(s *summarizerv1.InferenceSecret) { s.Version = types.V2 },
			msg: "unsupported version v2, supported: v1",
		},
		{
			fn:  func(s *summarizerv1.InferenceSecret) { s.Metadata = nil },
			msg: "metadata is required",
		},
		{
			fn:  func(s *summarizerv1.InferenceSecret) { s.Metadata.Name = "" },
			msg: "metadata.name is required",
		},
		{
			fn:  func(s *summarizerv1.InferenceSecret) { s.Spec = nil },
			msg: "spec is required",
		},
		{
			fn:  func(s *summarizerv1.InferenceSecret) { s.Spec.Value = "" },
			msg: "spec.value is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			s := proto.CloneOf(valid)
			tc.fn(s)
			assert.ErrorIs(t, ValidateInferenceSecret(s), &trace.BadParameterError{Message: tc.msg})
		})
	}
}

func TestValidateInferencePolicy(t *testing.T) {
	t.Parallel()
	valid := NewInferencePolicy("my-policy", &summarizerv1.InferencePolicySpec{
		Kinds:  []string{"ssh", "k8s", "db"},
		Filter: `equals(resource.metadata.labels["env"], "prod") || equals(user.metadata.name, "admin")`,
		Model:  "my-model",
	})
	require.NoError(t, ValidateInferencePolicy(valid))
	// Empty filter should also be valid.
	valid.Spec.Filter = ""
	require.NoError(t, ValidateInferencePolicy(valid))

	cases := []struct {
		fn  func(p *summarizerv1.InferencePolicy)
		msg string
	}{
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.Kind = "other" },
			msg: "kind must be inference_policy, got other",
		},
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.SubKind = "foo" },
			msg: "subkind must be empty",
		},
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.Version = "" },
			msg: "version is required",
		},
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.Version = types.V2 },
			msg: "unsupported version v2, supported: v1",
		},
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.Metadata = nil },
			msg: "metadata is required",
		},
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.Metadata.Name = "" },
			msg: "metadata.name is required",
		},
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.Spec = nil },
			msg: "spec is required",
		},
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.Spec.Kinds = nil },
			msg: "spec.kinds are required",
		},
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.Spec.Kinds = []string{"foo"} },
			msg: "unsupported kind in spec.kinds: foo, supported: ssh, k8s, db",
		},
		{
			fn:  func(p *summarizerv1.InferencePolicy) { p.Spec.Model = "" },
			msg: "spec.model is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			p := proto.CloneOf(valid)
			tc.fn(p)
			assert.ErrorContains(t, ValidateInferencePolicy(p), tc.msg)
		})
	}
}
