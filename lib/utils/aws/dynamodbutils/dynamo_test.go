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

package dynamodbutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/aws/dynamodbutils"
)

func TestIsFIPSEnabled(t *testing.T) {
	// Don't t.Parallel(), uses t.Setenv and modules.SetTestModules.

	tests := []struct {
		name        string
		fips        bool
		envVarValue string // value for the _DISABLE_FIPS environment variable
		want        bool
	}{
		{
			name: "non-FIPS binary",
			want: false,
		},
		{
			name: "FIPS binary",
			fips: true,
			want: true,
		},
		{
			name:        "FIPS binary with skip",
			fips:        true,
			envVarValue: "yes",
			want:        false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TELEPORT_UNSTABLE_DISABLE_AWS_FIPS", test.envVarValue)

			modules.SetTestModules(t, &modules.TestModules{
				FIPS: test.fips,
			})

			got := dynamodbutils.IsFIPSEnabled()
			assert.Equal(t, test.want, got, "IsFIPSEnabled mismatch")
		})
	}
}
