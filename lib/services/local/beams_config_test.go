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

package local

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
)

func TestBeamsConfigService(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := NewBeamsConfigService(bk)
	require.NoError(t, err)

	// Get returns virtual default when nothing is stored.
	virtual, err := service.GetBeamsConfig(ctx)
	require.NoError(t, err)
	require.Empty(t, virtual.GetMetadata().GetRevision())
	require.Equal(t, "anthropic", virtual.GetSpec().GetLlm().GetAnthropic().GetAppName())
	require.Equal(t, "openai", virtual.GetSpec().GetLlm().GetOpenai().GetAppName())

	// Create
	config := services.DefaultBeamsConfig()
	config.GetSpec().GetLlm().SetAnthropic(beamsv1.LLMEndpointConfig_builder{
		AppName: "my-anthropic",
	}.Build())
	created, err := service.CreateBeamsConfig(ctx, config)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetMetadata().GetRevision())
	require.Equal(t, "my-anthropic", created.GetSpec().GetLlm().GetAnthropic().GetAppName())

	// Create duplicate fails
	_, err = service.CreateBeamsConfig(ctx, config)
	require.True(t, trace.IsAlreadyExists(err))

	// Get returns stored resource, not virtual default.
	got, err := service.GetBeamsConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, created.GetMetadata().GetRevision(), got.GetMetadata().GetRevision())
	require.Equal(t, "my-anthropic", got.GetSpec().GetLlm().GetAnthropic().GetAppName())

	// Update
	toUpdate := proto.Clone(got).(*beamsv1.BeamsConfig)
	toUpdate.GetSpec().GetLlm().GetAnthropic().SetAppName("anthropic-byo-bedrock")
	updated, err := service.UpdateBeamsConfig(ctx, toUpdate)
	require.NoError(t, err)
	require.Equal(t, "anthropic-byo-bedrock", updated.GetSpec().GetLlm().GetAnthropic().GetAppName())

	// Update with stale revision fails.
	_, err = service.UpdateBeamsConfig(ctx, got)
	var compareFailedError *trace.CompareFailedError
	require.ErrorAs(t, err, &compareFailedError)

	// Delete
	err = service.DeleteBeamsConfig(ctx)
	require.NoError(t, err)

	// Get after delete returns virtual default.
	afterDelete, err := service.GetBeamsConfig(ctx)
	require.NoError(t, err)
	require.Empty(t, afterDelete.GetMetadata().GetRevision())

	// Delete non-existent returns not found
	err = service.DeleteBeamsConfig(ctx)
	require.True(t, trace.IsNotFound(err))
}

func TestBeamsConfigValidation(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := NewBeamsConfigService(bk)
	require.NoError(t, err)

	for _, tc := range []struct {
		name    string
		modify  func(*beamsv1.BeamsConfig)
		wantErr bool
	}{
		{
			name:    "invalid kind",
			modify:  func(c *beamsv1.BeamsConfig) { c.SetKind("wrong") },
			wantErr: true,
		},
		{
			name:    "invalid version",
			modify:  func(c *beamsv1.BeamsConfig) { c.SetVersion("v99") },
			wantErr: true,
		},
		{
			name:    "invalid name",
			modify:  func(c *beamsv1.BeamsConfig) { c.GetMetadata().Name = "wrong-name" },
			wantErr: true,
		},
		{
			name:    "expiry not allowed",
			modify:  func(c *beamsv1.BeamsConfig) { c.GetMetadata().Expires = timestamppb.Now() },
			wantErr: true,
		},
		{
			name:   "valid config",
			modify: func(c *beamsv1.BeamsConfig) {},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := services.DefaultBeamsConfig()
			tc.modify(config)
			_, err := service.CreateBeamsConfig(t.Context(), config)
			if tc.wantErr {
				require.True(t, trace.IsBadParameter(err), "expected BadParameter, got: %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBeamsConfigParser(t *testing.T) {
	t.Parallel()
	parser := newBeamsConfigParser()

	t.Run("delete", func(t *testing.T) {
		event := backend.Event{
			Type: types.OpDelete,
			Item: backend.Item{
				Key: backend.NewKey(beamsConfigPrefix, types.MetaNameBeamsConfig),
			},
		}
		require.True(t, parser.match(event.Item.Key))
		resource, err := parser.parse(event)
		require.NoError(t, err)
		require.Equal(t, types.MetaNameBeamsConfig, resource.GetMetadata().Name)
		require.Equal(t, types.KindBeamsConfig, resource.GetKind())
	})

	t.Run("put", func(t *testing.T) {
		config := services.DefaultBeamsConfig()
		data, err := services.MarshalProtoResource[*beamsv1.BeamsConfig](config)
		require.NoError(t, err)

		event := backend.Event{
			Type: types.OpPut,
			Item: backend.Item{
				Key:   backend.NewKey(beamsConfigPrefix, types.MetaNameBeamsConfig),
				Value: data,
			},
		}
		require.True(t, parser.match(event.Item.Key))
		resource, err := parser.parse(event)
		require.NoError(t, err)
		require.Equal(t, types.KindBeamsConfig, resource.GetKind())
	})
}
