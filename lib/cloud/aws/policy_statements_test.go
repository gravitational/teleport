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

package aws

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatementForBedrockSessionSummaries(t *testing.T) {
	tests := []struct {
		name             string
		accountID        string
		resource         string
		expectedResource string
	}{
		{
			name:             "bedrock ARN is preserved",
			accountID:        "123456789012",
			resource:         "arn:aws:bedrock:us-east-1:123456789012:foundation-model/anthropic.claude-v2",
			expectedResource: "arn:aws:bedrock:us-east-1:123456789012:foundation-model/anthropic.claude-v2",
		},
		{
			name:             "inference profile ARN is preserved",
			accountID:        "123456789012",
			resource:         "arn:aws:bedrock:us-east-1:123456789012:inference-profile/my-profile",
			expectedResource: "arn:aws:bedrock:us-east-1:123456789012:inference-profile/my-profile",
		},
		{
			name:             "model ID converted to ARN with wildcard region",
			accountID:        "123456789012",
			resource:         "anthropic.claude-v2",
			expectedResource: "arn:aws:bedrock:*:123456789012:foundation-model/anthropic.claude-v2",
		},
		{
			name:             "wildcard model ID",
			accountID:        "123456789012",
			resource:         "*",
			expectedResource: "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement := StatementForBedrockSessionSummaries(tt.accountID, tt.resource)

			require.Equal(t, &Statement{
				Effect:    EffectAllow,
				Actions:   SliceOrString{"bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"},
				Resources: SliceOrString{tt.expectedResource},
			}, statement)
		})
	}
}
