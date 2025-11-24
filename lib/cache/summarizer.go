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
	"iter"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// InferenceModel cache

type inferenceModelIndex struct{}

var inferenceModelNameIndex = inferenceModelIndex{}

func newInferenceModelCollection(upstream services.Summarizer, w types.WatchKind) (*collection[*summarizerv1.InferenceModel, inferenceModelIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Summarizer")
	}

	return &collection[*summarizerv1.InferenceModel, inferenceModelIndex]{
		store: newStore(
			types.KindInferenceModel,
			proto.CloneOf[*summarizerv1.InferenceModel],
			map[inferenceModelIndex]func(*summarizerv1.InferenceModel) string{
				inferenceModelNameIndex: func(r *summarizerv1.InferenceModel) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*summarizerv1.InferenceModel, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListInferenceModels))
			return out, trace.Wrap(err)
		},
		watch: w,
	}, nil
}

func (c *Cache) GetInferenceModel(ctx context.Context, name string) (*summarizerv1.InferenceModel, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetInferenceModel")
	defer span.End()

	getter := genericGetter[*summarizerv1.InferenceModel, inferenceModelIndex]{
		cache:       c,
		collection:  c.collections.inferenceModels,
		index:       inferenceModelNameIndex,
		upstreamGet: c.Config.Summarizer.GetInferenceModel,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

func (c *Cache) ListInferenceModels(ctx context.Context, pageSize int, pageToken string) ([]*summarizerv1.InferenceModel, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListInferenceModels")
	defer span.End()

	lister := genericLister[*summarizerv1.InferenceModel, inferenceModelIndex]{
		cache:        c,
		collection:   c.collections.inferenceModels,
		index:        inferenceModelNameIndex,
		upstreamList: c.Config.Summarizer.ListInferenceModels,
		nextToken: func(t *summarizerv1.InferenceModel) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// InferenceSecret cache

type inferenceSecretIndex struct{}

var inferenceSecretNameIndex = inferenceSecretIndex{}

func newInferenceSecretCollection(upstream services.Summarizer, w types.WatchKind) (*collection[*summarizerv1.InferenceSecret, inferenceSecretIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Summarizer")
	}

	return &collection[*summarizerv1.InferenceSecret, inferenceSecretIndex]{
		store: newStore(
			types.KindInferenceSecret,
			proto.CloneOf[*summarizerv1.InferenceSecret],
			map[inferenceSecretIndex]func(*summarizerv1.InferenceSecret) string{
				inferenceSecretNameIndex: func(r *summarizerv1.InferenceSecret) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*summarizerv1.InferenceSecret, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListInferenceSecrets))
			return out, trace.Wrap(err)
		},
		watch: w,
	}, nil
}

func (c *Cache) GetInferenceSecret(ctx context.Context, name string) (*summarizerv1.InferenceSecret, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetInferenceSecret")
	defer span.End()

	getter := genericGetter[*summarizerv1.InferenceSecret, inferenceSecretIndex]{
		cache:       c,
		collection:  c.collections.inferenceSecrets,
		index:       inferenceSecretNameIndex,
		upstreamGet: c.Config.Summarizer.GetInferenceSecret,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

func (c *Cache) ListInferenceSecrets(ctx context.Context, pageSize int, pageToken string) ([]*summarizerv1.InferenceSecret, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListInferenceSecrets")
	defer span.End()

	lister := genericLister[*summarizerv1.InferenceSecret, inferenceSecretIndex]{
		cache:        c,
		collection:   c.collections.inferenceSecrets,
		index:        inferenceSecretNameIndex,
		upstreamList: c.Config.Summarizer.ListInferenceSecrets,
		nextToken: func(t *summarizerv1.InferenceSecret) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// InferencePolicy cache

type inferencePolicyIndex struct{}

var inferencePolicyNameIndex = inferencePolicyIndex{}

func newInferencePolicyCollection(upstream services.Summarizer, w types.WatchKind) (*collection[*summarizerv1.InferencePolicy, inferencePolicyIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Summarizer")
	}

	return &collection[*summarizerv1.InferencePolicy, inferencePolicyIndex]{
		store: newStore(
			types.KindInferencePolicy,
			proto.CloneOf[*summarizerv1.InferencePolicy],
			map[inferencePolicyIndex]func(*summarizerv1.InferencePolicy) string{
				inferencePolicyNameIndex: func(r *summarizerv1.InferencePolicy) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*summarizerv1.InferencePolicy, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListInferencePolicies))
			return out, trace.Wrap(err)
		},
		watch: w,
	}, nil
}

func (c *Cache) GetInferencePolicy(ctx context.Context, name string) (*summarizerv1.InferencePolicy, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetInferencePolicy")
	defer span.End()

	getter := genericGetter[*summarizerv1.InferencePolicy, inferencePolicyIndex]{
		cache:       c,
		collection:  c.collections.inferencePolicies,
		index:       inferencePolicyNameIndex,
		upstreamGet: c.Config.Summarizer.GetInferencePolicy,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

func (c *Cache) ListInferencePolicies(ctx context.Context, pageSize int, pageToken string) ([]*summarizerv1.InferencePolicy, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListInferencePolicies")
	defer span.End()

	lister := genericLister[*summarizerv1.InferencePolicy, inferencePolicyIndex]{
		cache:        c,
		collection:   c.collections.inferencePolicies,
		index:        inferencePolicyNameIndex,
		upstreamList: c.Config.Summarizer.ListInferencePolicies,
		nextToken: func(t *summarizerv1.InferencePolicy) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

func (c *Cache) AllInferencePolicies(ctx context.Context) iter.Seq2[*summarizerv1.InferencePolicy, error] {
	return func(yield func(*summarizerv1.InferencePolicy, error) bool) {
		rg, err := acquireReadGuard(c, c.collections.inferencePolicies)
		if err != nil {
			yield(nil, trace.Wrap(err))
			return
		}
		defer rg.Release()

		if !rg.ReadCache() {
			for policy, err := range c.Config.Summarizer.AllInferencePolicies(ctx) {
				if !yield(policy, err) {
					return
				}
			}
			return
		}

		for policy := range rg.store.resources(inferencePolicyNameIndex, "", "") {
			if !yield(proto.CloneOf(policy), nil) {
				return
			}
		}
	}
}

// SearchModel cache

type searchModelIndex struct{}

var searchModelNameIndex = searchModelIndex{}

func newSearchModelCollection(upstream services.Summarizer, w types.WatchKind) (*collection[*summarizerv1.SearchModel, searchModelIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Summarizer")
	}

	return &collection[*summarizerv1.SearchModel, searchModelIndex]{
		store: newStore(
			types.KindSearchModel,
			proto.CloneOf[*summarizerv1.SearchModel],
			map[searchModelIndex]func(*summarizerv1.SearchModel) string{
				searchModelNameIndex: func(r *summarizerv1.SearchModel) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*summarizerv1.SearchModel, error) {
			model, err := upstream.GetSearchModel(ctx)
			if err != nil {
				if trace.IsNotFound(err) {
					return nil, nil
				}
				return nil, trace.Wrap(err)
			}
			return []*summarizerv1.SearchModel{model}, nil
		},
		watch:     w,
		singleton: true,
	}, nil
}

func (c *Cache) GetSearchModel(ctx context.Context) (*summarizerv1.SearchModel, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSearchModel")
	defer span.End()

	getter := genericGetter[*summarizerv1.SearchModel, searchModelIndex]{
		cache:      c,
		collection: c.collections.searchModels,
		index:      searchModelNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (*summarizerv1.SearchModel, error) {
			return c.Config.Summarizer.GetSearchModel(ctx)
		},
	}
	out, err := getter.get(ctx, types.MetaNameSearchModel)
	return out, trace.Wrap(err)
}
