/*

 Copyright 2022 Gravitational, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.


*/

package snowflake

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_extractAccountName(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name: "correct AWS address",
			uri:  "abc123.us-east-2.aws.snowflakecomputing.com",
			want: "abc123.us-east-2.aws",
		},
		{
			name: "correct AWS address",
			uri:  "abc123.snowflakecomputing.com",
			want: "abc123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractAccountName(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractAccountName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}
