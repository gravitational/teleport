/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package handler

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

func TestNewAPIApp(t *testing.T) {
	t.Run("LLM app exposes its inference format and provider", func(t *testing.T) {
		app, err := types.NewAppV3(types.Metadata{Name: "anthropic"}, types.AppSpecV3{
			URI: "llm://",
			LLM: &types.LLM{Format: "anthropic", Provider: "bedrock"},
		})
		require.NoError(t, err)

		apiApp := newAPIApp(clusters.App{
			URI: uri.NewClusterURI("foo").AppendApp("anthropic"),
			App: app,
		})

		require.Equal(t, "anthropic", apiApp.GetLlmFormat())
		require.Equal(t, "bedrock", apiApp.GetLlmProvider())
	})

	t.Run("non-LLM app has an empty format and provider", func(t *testing.T) {
		app, err := types.NewAppV3(types.Metadata{Name: "foo"}, types.AppSpecV3{
			URI: "tcp://localhost:8080",
		})
		require.NoError(t, err)

		apiApp := newAPIApp(clusters.App{
			URI: uri.NewClusterURI("foo").AppendApp("foo"),
			App: app,
		})

		require.Empty(t, apiApp.GetLlmFormat())
		require.Empty(t, apiApp.GetLlmProvider())
	})
}
