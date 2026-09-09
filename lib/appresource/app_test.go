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

package appresource

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGovernsApp(t *testing.T) {
	tests := []struct {
		name    string
		spec    types.AppSpecV3
		subKind string
		want    bool
	}{
		{name: "http", spec: types.AppSpecV3{URI: "http://localhost:18080"}, want: true},
		{name: "tcp", spec: types.AppSpecV3{URI: "tcp://localhost:5432"}, want: false},
		{name: "mcp", spec: types.AppSpecV3{URI: "mcp+stdio://everything", MCP: &types.MCP{Command: "docker", RunAsHostUser: "teleport"}}, want: false},
		{name: "aws console", spec: types.AppSpecV3{URI: "https://console.aws.amazon.com"}, want: false},
		{name: "azure", spec: types.AppSpecV3{URI: "https://management.azure.com", Cloud: types.CloudAzure}, want: false},
		{name: "gcp", spec: types.AppSpecV3{URI: "https://console.cloud.google.com", Cloud: types.CloudGCP}, want: false},
		{name: "llm", spec: types.AppSpecV3{LLM: &types.LLM{Format: types.LLMFormatAnthropic, Provider: types.LLMProviderAnthropic}}, want: false},
		{name: "identity center", spec: types.AppSpecV3{URI: "https://example.awsapps.com/start"}, subKind: types.KindIdentityCenterAccount, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := types.NewAppV3(types.Metadata{Name: "app"}, tt.spec)
			require.NoError(t, err)
			if tt.subKind != "" {
				app.SetSubKind(tt.subKind)
			}
			require.Equal(t, tt.want, GovernsApp(app))
		})
	}
}
