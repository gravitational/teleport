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

package awscommon

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidIntegrationName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "lowercase alphanumeric", input: "myintegration"},
		{name: "with hyphen", input: "my-integration"},
		{name: "leading digit", input: "1integration"},
		{name: "uppercase rejected", input: "Integration", wantErr: "must be a valid DNS label"},
		{name: "trailing hyphen rejected", input: "INVALID-", wantErr: "must be a valid DNS label"},
		{name: "empty rejected", input: "", wantErr: "must be a valid DNS label"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidIntegrationName(tt.input)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
