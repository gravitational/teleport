// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package cache

import (
	"context"
	"testing"
	"testing/synctest"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/summarizer"
)

func TestInferenceModelCache(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*summarizerv1.InferenceModel]{
		newResource: func(name string) (*summarizerv1.InferenceModel, error) {
			return summarizer.NewInferenceModel(name, summarizerv1.InferenceModelSpec_builder{
				Openai: summarizerv1.OpenAIProvider_builder{
					OpenaiModelId: "gpt-4o",
				}.Build(),
			}.Build()), nil
		},
		create: func(ctx context.Context, item *summarizerv1.InferenceModel) error {
			_, err := p.summarizer.CreateInferenceModel(ctx, item)
			return err
		},
		list:      p.summarizer.ListInferenceModels,
		cacheList: p.cache.ListInferenceModels,
		deleteAll: p.summarizer.DeleteAllInferenceModels,
	})
}

func TestInferenceSecretCache(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*summarizerv1.InferenceSecret]{
		newResource: func(name string) (*summarizerv1.InferenceSecret, error) {
			return summarizer.NewInferenceSecret(name, summarizerv1.InferenceSecretSpec_builder{
				Value: "super-secret-value",
			}.Build()), nil
		},
		create: func(ctx context.Context, item *summarizerv1.InferenceSecret) error {
			_, err := p.summarizer.CreateInferenceSecret(ctx, item)
			return err
		},
		list:      p.summarizer.ListInferenceSecrets,
		cacheList: p.cache.ListInferenceSecrets,
		deleteAll: p.summarizer.DeleteAllInferenceSecrets,
	})
}

func TestInferencePolicyCache(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*summarizerv1.InferencePolicy]{
		newResource: func(name string) (*summarizerv1.InferencePolicy, error) {
			return summarizer.NewInferencePolicy(name, summarizerv1.InferencePolicySpec_builder{
				Kinds: []string{string(types.SSHSessionKind)},
				Model: "test-model",
			}.Build()), nil
		},
		create: func(ctx context.Context, item *summarizerv1.InferencePolicy) error {
			_, err := p.summarizer.CreateInferencePolicy(ctx, item)
			return err
		},
		list:      p.summarizer.ListInferencePolicies,
		cacheList: p.cache.ListInferencePolicies,
		deleteAll: p.summarizer.DeleteAllInferencePolicies,
	})
}

func TestRetrievalModelCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)

		testSingleton153(t, p, testSingletonFuncs153[*summarizerv1.RetrievalModel]{
			newResource: func() *summarizerv1.RetrievalModel {
				return summarizer.NewRetrievalModel(summarizerv1.RetrievalModelSpec_builder{
					Openai: summarizerv1.OpenAIProvider_builder{
						OpenaiModelId:   "text-embedding-3-small",
						ApiKeySecretRef: "some",
					}.Build(),
					InferenceModelName: "some",
				}.Build())
			},
			create:   p.summarizer.CreateRetrievalModel,
			update:   p.summarizer.UpdateRetrievalModel,
			get:      p.summarizer.GetRetrievalModel,
			cacheGet: p.cache.GetRetrievalModel,
			delete:   p.summarizer.DeleteRetrievalModel,
		})
	})
}
