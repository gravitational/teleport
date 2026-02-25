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

package generators

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
)

func TestPascalToUpperSnake(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"AccessPolicy", "ACCESS_POLICY"},
		{"Webhook", "WEBHOOK"},
		{"CrownJewel", "CROWN_JEWEL"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.expected, pascalToUpperSnake(tt.input))
		})
	}
}

func TestArticle(t *testing.T) {
	require.Equal(t, "an", article("Access Policy"))
	require.Equal(t, "a", article("Webhook"))
	require.Equal(t, "an", article("Elephant"))
	require.Equal(t, "a", article("Crown Jewel"))
}

func TestPascalToDisplayName(t *testing.T) {
	require.Equal(t, "Access Policy", pascalToDisplayName("AccessPolicy"))
	require.Equal(t, "Webhook", pascalToDisplayName("Webhook"))
	require.Equal(t, "Crown Jewel", pascalToDisplayName("CrownJewel"))
}

func TestGenerateWebEventsTS(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:       "webhook",
			KindPascal: "Webhook",
			Audit: spec.AuditConfig{
				EmitOnCreate: true,
				EmitOnUpdate: true,
				EmitOnDelete: true,
				CodePrefix:   "WH",
			},
			Operations: spec.OperationSet{
				Create: true, Update: true, Delete: true, Get: true, List: true,
			},
		},
	}
	out, err := GenerateWebEventsTS(specs)
	require.NoError(t, err)
	require.Contains(t, out, "WEBHOOK_CREATE: 'WH001I'")
	require.Contains(t, out, "WEBHOOK_UPDATE: 'WH002I'")
	require.Contains(t, out, "WEBHOOK_DELETE: 'WH003I'")
	require.Contains(t, out, "Webhook Created")
	require.Contains(t, out, "created a webhook")
	require.Contains(t, out, "resource.webhook.create")
	// EmitOnGet is false so no GET event
	require.NotContains(t, out, "WEBHOOK_GET")
}

func TestGenerateWebEventsTSEmpty(t *testing.T) {
	out, err := GenerateWebEventsTS(nil)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(out, "// Code generated"))
	require.Contains(t, out, "generatedEventCodes = {}")
}
