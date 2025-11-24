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

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

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
			return summarizer.NewInferenceModel(name, &summarizerv1.InferenceModelSpec{
				Provider: &summarizerv1.InferenceModelSpec_Openai{
					Openai: &summarizerv1.OpenAIProvider{
						OpenaiModelId: "gpt-4o",
					},
				},
			}), nil
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
			return summarizer.NewInferenceSecret(name, &summarizerv1.InferenceSecretSpec{
				Value: "super-secret-value",
			}), nil
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
			return summarizer.NewInferencePolicy(name, &summarizerv1.InferencePolicySpec{
				Kinds: []string{string(types.SSHSessionKind)},
				Model: "test-model",
			}), nil
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

func TestSearchModelCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p, err := newPack(t, func(c Config) Config {
		c.Watches = []types.WatchKind{
			{Kind: types.KindSearchModel},
		}
		return ForAuth(c)
	})
	require.NoError(t, err)
	t.Cleanup(p.Close)

	// Create test search model
	model := summarizer.NewSearchModel(&summarizerv1.SearchModelSpec{
		EmbeddingsProvider: &summarizerv1.SearchModelSpec_Openai{
			Openai: &summarizerv1.OpenAIProvider{
				OpenaiModelId: "text-embedding-3-small",
			},
		},
	})

	// Create model in backend
	created, err := p.summarizer.CreateSearchModel(ctx, model)
	require.NoError(t, err)

	// Get from cache
	out, err := p.cache.GetSearchModel(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(created, out, protocmp.Transform()))
}
