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

package aws_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/modules"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

func TestUseFIPSForDynamoDB(t *testing.T) {
	// Don't t.Parallel(), uses both modules.SetTestModules and t.Setenv.

	tests := []struct {
		name string
		fips bool
		env  map[string]string
		want bool
	}{
		{
			name: "non-FIPS binary",
			want: false,
		},
		{
			name: "non-FIPS binary with env skip",
			env: map[string]string{
				awsutils.EnvVarDisableDynamoDBFIPS: "yes",
			},
			want: false,
		},
		{
			name: "FIPS binary",
			fips: true,
			want: true,
		},
		{
			name: "FIPS binary with env skip",
			fips: true,
			env: map[string]string{
				awsutils.EnvVarDisableDynamoDBFIPS: "yes",
			},
			want: false,
		},
		{
			name: "FIPS binary with env skip (strconv.ParseBool)",
			fips: true,
			env: map[string]string{
				awsutils.EnvVarDisableDynamoDBFIPS: "1",
			},
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modules.SetTestModules(t, &modules.TestModules{
				FIPS: test.fips,
			})
			for k, v := range test.env {
				t.Setenv(k, v)
			}

			assert.Equal(t, test.want, awsutils.UseFIPSForDynamoDB(), "UseFIPSForDynamoDB mismatch")
		})
	}
}
