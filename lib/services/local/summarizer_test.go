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

package local

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func newInferenceModel(name string) *summarizerv1.InferenceModel {
	return summarizer.NewInferenceModel(name, &summarizerv1.InferenceModelSpec{
		Provider: &summarizerv1.InferenceModelSpec_Openai{
			Openai: &summarizerv1.OpenAIProvider{
				OpenaiModelId: "gpt-4o",
			},
		},
	})
}

func newInferenceSecret(name string) *summarizerv1.InferenceSecret {
	return summarizer.NewInferenceSecret(name, &summarizerv1.InferenceSecretSpec{
		Value: "super-secret-value",
	})
}

func newInferencePolicy(name string) *summarizerv1.InferencePolicy {
	return summarizer.NewInferencePolicy(name, &summarizerv1.InferencePolicySpec{
		Kinds: []string{string(types.SSHSessionKind)},
		Model: "dummy-model",
	})
}

func setupSummarizerTest(
	t *testing.T,
) (context.Context, *SummarizerService) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	service, err := NewSummarizerService(SummarizerServiceConfig{
		Backend: backend.NewSanitizer(mem),
	})
	require.NoError(t, err)
	return ctx, service
}

func TestSummarizerService_CreateInferenceModel(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newInferenceModel("dummy-model")
		got, err := service.CreateInferenceModel(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*summarizerv1.InferenceModel),
		)
		require.NoError(t, err)
		assert.NotEmpty(t, got.Metadata.Revision)
		assert.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})
	t.Run("invalid", func(t *testing.T) {
		m := newInferenceModel("invalid-model")
		m.Spec.GetOpenai().OpenaiModelId = ""
		_, err := service.CreateInferenceModel(
			ctx,
			proto.Clone(m).(*summarizerv1.InferenceModel),
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, trace.BadParameter("spec.openai.openai_model_id is required"))
	})
	t.Run("no Bedrock unless enabled", func(t *testing.T) {
		m := summarizer.NewInferenceModel("bedrock-model", &summarizerv1.InferenceModelSpec{
			Provider: &summarizerv1.InferenceModelSpec_Bedrock{
				Bedrock: &summarizerv1.BedrockProvider{
					Region:         "us-east-1",
					BedrockModelId: "amazon.nova-pro-v1:0",
				},
			},
		})
		_, err := service.CreateInferenceModel(ctx, m)
		require.Error(t, err)
		assert.ErrorIs(t, err, trace.BadParameter("Amazon Bedrock models are unavailable in Teleport Cloud"))
	})
	t.Run("no upsert", func(t *testing.T) {
		res := newInferenceModel("no-upsert")
		_, err := service.CreateInferenceModel(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*summarizerv1.InferenceModel),
		)
		require.NoError(t, err)
		_, err = service.CreateInferenceModel(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*summarizerv1.InferenceModel),
		)
		require.Error(t, err)
		assert.True(t, trace.IsAlreadyExists(err))
	})
}

func TestSummarizerService_CreateInferenceModel_BedrockAllowed(t *testing.T) {
	// Perform a similar setup procedure, but enable Bedrock.
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	service, err := NewSummarizerService(SummarizerServiceConfig{
		Backend:       backend.NewSanitizer(mem),
		EnableBedrock: true,
	})
	require.NoError(t, err)

	want := summarizer.NewInferenceModel("bedrock-model", &summarizerv1.InferenceModelSpec{
		Provider: &summarizerv1.InferenceModelSpec_Bedrock{
			Bedrock: &summarizerv1.BedrockProvider{
				Region:         "us-east-1",
				BedrockModelId: "amazon.nova-pro-v1:0",
			},
		},
	})

	got, err := service.CreateInferenceModel(
		ctx,
		// Clone to avoid Marshaling modifying want
		proto.Clone(want).(*summarizerv1.InferenceModel),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, got.Metadata.Revision)
	assert.Empty(t, cmp.Diff(
		want,
		got,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))
}

func TestSummarizerService_UpsertInferenceModel(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	want := newInferenceModel("dummy-model")
	got, err := service.UpsertInferenceModel(
		ctx,
		// Clone to avoid Marshaling modifying want
		proto.Clone(want).(*summarizerv1.InferenceModel),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, got.Metadata.Revision)
	assert.Empty(t, cmp.Diff(
		want,
		got,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	// Ensure we can upsert over an existing resource
	_, err = service.UpsertInferenceModel(
		ctx,
		// Clone to avoid Marshaling modifying want
		proto.Clone(want).(*summarizerv1.InferenceModel),
	)
	require.NoError(t, err)
}

func TestSummarizerService_ListInferenceModels(t *testing.T) {
	ctx, service := setupSummarizerTest(t)
	// Create entities to list
	createdObjects := []*summarizerv1.InferenceModel{}
	// Create 49 entities to test an incomplete page at the end.
	for i := range 49 {
		created, err := service.CreateInferenceModel(
			ctx,
			newInferenceModel(fmt.Sprintf("model-%d", i)),
		)
		require.NoError(t, err)
		createdObjects = append(createdObjects, created)
	}
	t.Run("default page size", func(t *testing.T) {
		page, nextToken, err := service.ListInferenceModels(ctx, 0, "")
		require.NoError(t, err)
		assert.Len(t, page, 49)
		assert.Empty(t, nextToken)

		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			assert.True(t, slices.ContainsFunc(page, func(model *summarizerv1.InferenceModel) bool {
				return proto.Equal(created, model)
			}))
		}
	})
	t.Run("pagination", func(t *testing.T) {
		fetched := []*summarizerv1.InferenceModel{}
		token := ""
		iterations := 0
		for {
			iterations++
			page, nextToken, err := service.ListInferenceModels(ctx, 10, token)
			require.NoError(t, err)
			fetched = append(fetched, page...)
			if nextToken == "" {
				break
			}
			token = nextToken
		}
		assert.Equal(t, 5, iterations)

		assert.Len(t, fetched, 49)
		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			assert.True(t, slices.ContainsFunc(fetched, func(model *summarizerv1.InferenceModel) bool {
				return proto.Equal(created, model)
			}))
		}
	})
}

func TestSummarizerService_GetInferenceModel(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newInferenceModel("dummy-model")
		_, err := service.CreateInferenceModel(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*summarizerv1.InferenceModel),
		)
		require.NoError(t, err)
		got, err := service.GetInferenceModel(ctx, "dummy-model")
		require.NoError(t, err)
		assert.NotEmpty(t, got.Metadata.Revision)
		assert.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})
	t.Run("not found", func(t *testing.T) {
		_, err := service.GetInferenceModel(ctx, "foobar")
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err))
	})
}

func TestSummarizerService_DeleteInferenceModel(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		_, err := service.CreateInferenceModel(
			ctx,
			newInferenceModel("dummy-model"),
		)
		require.NoError(t, err)

		_, err = service.GetInferenceModel(ctx, "dummy-model")
		require.NoError(t, err)

		err = service.DeleteInferenceModel(ctx, "dummy-model")
		require.NoError(t, err)

		_, err = service.GetInferenceModel(ctx, "dummy-model")
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err))
	})
	t.Run("not found", func(t *testing.T) {
		err := service.DeleteInferenceModel(ctx, "foobar")
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err))
	})
}

func TestSummarizerService_UpdateInferenceModel(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		// Create resource for us to Update since we can't update a non-existent resource.
		created, err := service.CreateInferenceModel(
			ctx,
			newInferenceModel("dummy-model"),
		)
		require.NoError(t, err)
		want := proto.Clone(created).(*summarizerv1.InferenceModel)
		want.Spec.GetOpenai().BaseUrl = "https://localhost:4000"

		updated, err := service.UpdateInferenceModel(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*summarizerv1.InferenceModel),
		)
		require.NoError(t, err)
		assert.NotEqual(t, created.Metadata.Revision, updated.Metadata.Revision)
		assert.Empty(t, cmp.Diff(
			want,
			updated,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))

		got, err := service.GetInferenceModel(ctx, "dummy-model")
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
		assert.Equal(t, updated.Metadata.Revision, got.Metadata.Revision)
	})
	t.Run("no create", func(t *testing.T) {
		_, err := service.UpdateInferenceModel(
			ctx,
			newInferenceModel("non-existing-model"),
		)
		require.Error(t, err)
	})
}

func TestSummarizerService_CreateInferenceSecret(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newInferenceSecret("dummy-secret")
		got, err := service.CreateInferenceSecret(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*summarizerv1.InferenceSecret),
		)
		require.NoError(t, err)
		assert.NotEmpty(t, got.Metadata.Revision)
		assert.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})
	t.Run("invalid", func(t *testing.T) {
		s := newInferenceSecret("invalid-secret")
		s.Spec.Value = ""
		_, err := service.CreateInferenceSecret(
			ctx,
			proto.Clone(s).(*summarizerv1.InferenceSecret),
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, trace.BadParameter("spec.value is required"))
	})
	t.Run("no upsert", func(t *testing.T) {
		res := newInferenceSecret("no-upsert")
		_, err := service.CreateInferenceSecret(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*summarizerv1.InferenceSecret),
		)
		require.NoError(t, err)
		_, err = service.CreateInferenceSecret(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*summarizerv1.InferenceSecret),
		)
		require.Error(t, err)
		assert.True(t, trace.IsAlreadyExists(err))
	})
}

func TestSummarizerService_UpsertInferenceSecret(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	want := newInferenceSecret("dummy-secret")
	got, err := service.UpsertInferenceSecret(
		ctx,
		// Clone to avoid Marshaling modifying want
		proto.Clone(want).(*summarizerv1.InferenceSecret),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, got.Metadata.Revision)
	assert.Empty(t, cmp.Diff(
		want,
		got,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	// Ensure we can upsert over an existing resource
	_, err = service.UpsertInferenceSecret(
		ctx,
		// Clone to avoid Marshaling modifying want
		proto.Clone(want).(*summarizerv1.InferenceSecret),
	)
	require.NoError(t, err)
}

func TestSummarizerService_ListInferenceSecrets(t *testing.T) {
	ctx, service := setupSummarizerTest(t)
	// Create entities to list
	createdObjects := []*summarizerv1.InferenceSecret{}
	// Create 49 entities to test an incomplete page at the end.
	for i := range 49 {
		created, err := service.CreateInferenceSecret(
			ctx,
			newInferenceSecret(fmt.Sprintf("secret-%d", i)),
		)
		require.NoError(t, err)
		createdObjects = append(createdObjects, created)
	}
	t.Run("default page size", func(t *testing.T) {
		page, nextToken, err := service.ListInferenceSecrets(ctx, 0, "")
		require.NoError(t, err)
		assert.Len(t, page, 49)
		assert.Empty(t, nextToken)

		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			assert.True(t, slices.ContainsFunc(page, func(secret *summarizerv1.InferenceSecret) bool {
				return proto.Equal(created, secret)
			}))
		}
	})
	t.Run("pagination", func(t *testing.T) {
		fetched := []*summarizerv1.InferenceSecret{}
		token := ""
		iterations := 0
		for {
			iterations++
			page, nextToken, err := service.ListInferenceSecrets(ctx, 10, token)
			require.NoError(t, err)
			fetched = append(fetched, page...)
			if nextToken == "" {
				break
			}
			token = nextToken
		}
		assert.Equal(t, 5, iterations)

		assert.Len(t, fetched, 49)
		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			assert.True(t, slices.ContainsFunc(fetched, func(secret *summarizerv1.InferenceSecret) bool {
				return proto.Equal(created, secret)
			}))
		}
	})
}

func TestSummarizerService_GetInferenceSecret(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newInferenceSecret("dummy-secret")
		_, err := service.CreateInferenceSecret(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*summarizerv1.InferenceSecret),
		)
		require.NoError(t, err)
		got, err := service.GetInferenceSecret(ctx, "dummy-secret")
		require.NoError(t, err)
		assert.NotEmpty(t, got.Metadata.Revision)
		assert.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})
	t.Run("not found", func(t *testing.T) {
		_, err := service.GetInferenceSecret(ctx, "foobar")
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err))
	})
}

func TestSummarizerService_DeleteInferenceSecret(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		_, err := service.CreateInferenceSecret(
			ctx,
			newInferenceSecret("dummy-secret"),
		)
		require.NoError(t, err)

		_, err = service.GetInferenceSecret(ctx, "dummy-secret")
		require.NoError(t, err)

		err = service.DeleteInferenceSecret(ctx, "dummy-secret")
		require.NoError(t, err)

		_, err = service.GetInferenceSecret(ctx, "dummy-secret")
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err))
	})
	t.Run("not found", func(t *testing.T) {
		err := service.DeleteInferenceSecret(ctx, "foobar")
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err))
	})
}

func TestSummarizerService_UpdateInferenceSecret(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		// Create resource for us to Update since we can't update a non-existent resource.
		created, err := service.CreateInferenceSecret(
			ctx,
			newInferenceSecret("dummy-secret"),
		)
		require.NoError(t, err)
		want := proto.Clone(created).(*summarizerv1.InferenceSecret)
		want.Spec.Value = "new-secret-value"

		updated, err := service.UpdateInferenceSecret(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*summarizerv1.InferenceSecret),
		)
		require.NoError(t, err)
		assert.NotEqual(t, created.Metadata.Revision, updated.Metadata.Revision)
		assert.Empty(t, cmp.Diff(
			want,
			updated,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))

		got, err := service.GetInferenceSecret(ctx, "dummy-secret")
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
		assert.Equal(t, updated.Metadata.Revision, got.Metadata.Revision)
	})
	t.Run("no create", func(t *testing.T) {
		_, err := service.UpdateInferenceSecret(
			ctx,
			newInferenceSecret("non-existing-secret"),
		)
		require.Error(t, err)
	})
}

func TestSummarizerService_CreateInferencePolicy(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newInferencePolicy("dummy-policy")
		got, err := service.CreateInferencePolicy(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*summarizerv1.InferencePolicy),
		)
		require.NoError(t, err)
		assert.NotEmpty(t, got.Metadata.Revision)
		assert.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})
	t.Run("invalid", func(t *testing.T) {
		p := newInferencePolicy("invalid-policy")
		p.Spec.Filter = "$%^@$"
		_, err := service.CreateInferencePolicy(
			ctx,
			proto.Clone(p).(*summarizerv1.InferencePolicy),
		)
		require.Error(t, err)
		assert.ErrorContains(t, err, "spec.filter has to be a valid predicate")
	})
	t.Run("no upsert", func(t *testing.T) {
		res := newInferencePolicy("no-upsert")
		_, err := service.CreateInferencePolicy(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*summarizerv1.InferencePolicy),
		)
		require.NoError(t, err)
		_, err = service.CreateInferencePolicy(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*summarizerv1.InferencePolicy),
		)
		require.Error(t, err)
		assert.True(t, trace.IsAlreadyExists(err))
	})
}

func TestSummarizerService_UpsertInferencePolicy(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	want := newInferencePolicy("dummy-policy")
	got, err := service.UpsertInferencePolicy(
		ctx,
		// Clone to avoid Marshaling modifying want
		proto.Clone(want).(*summarizerv1.InferencePolicy),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, got.Metadata.Revision)
	assert.Empty(t, cmp.Diff(
		want,
		got,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	// Ensure we can upsert over an existing resource
	_, err = service.UpsertInferencePolicy(
		ctx,
		// Clone to avoid Marshaling modifying want
		proto.Clone(want).(*summarizerv1.InferencePolicy),
	)
	require.NoError(t, err)
}

func TestSummarizerService_ListInferencePolicies(t *testing.T) {
	ctx, service := setupSummarizerTest(t)
	// Create entities to list
	createdObjects := []*summarizerv1.InferencePolicy{}
	// Create 49 entities to test an incomplete page at the end.
	for i := range 49 {
		created, err := service.CreateInferencePolicy(
			ctx,
			newInferencePolicy(fmt.Sprintf("policy-%d", i)),
		)
		require.NoError(t, err)
		createdObjects = append(createdObjects, created)
	}
	t.Run("default page size", func(t *testing.T) {
		page, nextToken, err := service.ListInferencePolicies(ctx, 0, "")
		require.NoError(t, err)
		assert.Len(t, page, 49)
		assert.Empty(t, nextToken)

		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			assert.True(t, slices.ContainsFunc(page, func(policy *summarizerv1.InferencePolicy) bool {
				return proto.Equal(created, policy)
			}))
		}
	})
	t.Run("pagination", func(t *testing.T) {
		fetched := []*summarizerv1.InferencePolicy{}
		token := ""
		iterations := 0
		for {
			iterations++
			page, nextToken, err := service.ListInferencePolicies(ctx, 10, token)
			require.NoError(t, err)
			fetched = append(fetched, page...)
			if nextToken == "" {
				break
			}
			token = nextToken
		}
		assert.Equal(t, 5, iterations)

		assert.Len(t, fetched, 49)
		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			assert.True(t, slices.ContainsFunc(fetched, func(policy *summarizerv1.InferencePolicy) bool {
				return proto.Equal(created, policy)
			}))
		}
	})
}

func TestSummarizerService_GetInferencePolicy(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newInferencePolicy("dummy-policy")
		_, err := service.CreateInferencePolicy(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*summarizerv1.InferencePolicy),
		)
		require.NoError(t, err)
		got, err := service.GetInferencePolicy(ctx, "dummy-policy")
		require.NoError(t, err)
		assert.NotEmpty(t, got.Metadata.Revision)
		assert.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})
	t.Run("not found", func(t *testing.T) {
		_, err := service.GetInferencePolicy(ctx, "foobar")
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err))
	})
}

func TestSummarizerService_DeleteInferencePolicy(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		_, err := service.CreateInferencePolicy(
			ctx,
			newInferencePolicy("dummy-policy"),
		)
		require.NoError(t, err)

		_, err = service.GetInferencePolicy(ctx, "dummy-policy")
		require.NoError(t, err)

		err = service.DeleteInferencePolicy(ctx, "dummy-policy")
		require.NoError(t, err)

		_, err = service.GetInferencePolicy(ctx, "dummy-policy")
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err))
	})
	t.Run("not found", func(t *testing.T) {
		err := service.DeleteInferencePolicy(ctx, "foobar")
		require.Error(t, err)
		assert.True(t, trace.IsNotFound(err))
	})
}

func TestSummarizerService_UpdateInferencePolicy(t *testing.T) {
	ctx, service := setupSummarizerTest(t)

	t.Run("ok", func(t *testing.T) {
		// Create resource for us to Update since we can't update a non-existent resource.
		created, err := service.CreateInferencePolicy(
			ctx,
			newInferencePolicy("dummy-policy"),
		)
		require.NoError(t, err)
		want := proto.Clone(created).(*summarizerv1.InferencePolicy)
		want.Spec.Kinds = []string{string(types.SSHSessionKind), string(types.DatabaseSessionKind)}

		updated, err := service.UpdateInferencePolicy(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*summarizerv1.InferencePolicy),
		)
		require.NoError(t, err)
		assert.NotEqual(t, created.Metadata.Revision, updated.Metadata.Revision)
		assert.Empty(t, cmp.Diff(
			want,
			updated,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))

		got, err := service.GetInferencePolicy(ctx, "dummy-policy")
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
		assert.Equal(t, updated.Metadata.Revision, got.Metadata.Revision)
	})
	t.Run("no create", func(t *testing.T) {
		_, err := service.UpdateInferencePolicy(
			ctx,
			newInferencePolicy("non-existing-policy"),
		)
		require.Error(t, err)
	})
}

func TestSummarizerService_AllInferencePolicies(t *testing.T) {
	ctx, service := setupSummarizerTest(t)
	// Create entities to retrieve
	createdObjects := []*summarizerv1.InferencePolicy{}
	for i := range 5 {
		created, err := service.CreateInferencePolicy(
			ctx,
			newInferencePolicy(fmt.Sprintf("policy-%d", i)),
		)
		require.NoError(t, err)
		createdObjects = append(createdObjects, created)
	}

	fetched := []*summarizerv1.InferencePolicy{}
	for policy, err := range service.AllInferencePolicies(ctx) {
		require.NoError(t, err)
		fetched = append(fetched, policy)
	}
	assert.Len(t, fetched, 5)

	// Expect that we get all the things we have created
	for _, created := range createdObjects {
		assert.True(t, slices.ContainsFunc(fetched, func(policy *summarizerv1.InferencePolicy) bool {
			return proto.Equal(created, policy)
		}))
	}
}
