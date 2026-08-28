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
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/services"
)

func TestBeamsConfigCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)

		ctx := t.Context()

		// Initially returns virtual default.
		virtual, err := p.cache.GetBeamsConfig(ctx)
		require.NoError(t, err)
		require.Equal(t, "anthropic", virtual.GetSpec().GetLlm().GetAnthropic().GetAppName())
		require.Equal(t, "openai", virtual.GetSpec().GetLlm().GetOpenai().GetAppName())

		// Create directly in backend.
		config := services.DefaultBeamsConfig()
		config.GetSpec().GetLlm().SetAnthropic(
			beamsv1.LLMEndpointConfig_builder{AppName: "my-anthropic"}.Build(),
		)
		_, err = p.beamsConfig.CreateBeamsConfig(ctx, config)
		require.NoError(t, err)

		// Wait for cache to pick it up.
		synctest.Wait()
		got, err := p.cache.GetBeamsConfig(ctx)
		require.NoError(t, err)
		require.Equal(t, "my-anthropic", got.GetSpec().GetLlm().GetAnthropic().GetAppName())

		// Update and verify cache picks up the change.
		got.GetSpec().GetLlm().GetAnthropic().SetAppName("anthropic-byo-bedrock")
		_, err = p.beamsConfig.UpdateBeamsConfig(ctx, got)
		require.NoError(t, err)

		synctest.Wait()
		updated, err := p.cache.GetBeamsConfig(ctx)
		require.NoError(t, err)
		require.Equal(t, "anthropic-byo-bedrock", updated.GetSpec().GetLlm().GetAnthropic().GetAppName())
	})
}
