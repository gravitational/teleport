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
