// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCertAuthorityOverrideID_FullName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   *CertAuthorityOverrideID
		want string
	}{
		{
			name: "nil ID",
			id:   nil,
			want: "",
		},
		{
			name: "empty ID",
			id:   &CertAuthorityOverrideID{},
			want: "",
		},
		{
			name: "ClusterName only",
			id: &CertAuthorityOverrideID{
				ClusterName: "zarquon",
			},
			want: "/zarquon",
		},
		{
			name: "CAType only",
			id: &CertAuthorityOverrideID{
				CAType: string(UserCA),
			},
			want: "user/",
		},
		{
			name: "ok",
			id: &CertAuthorityOverrideID{
				ClusterName: "zarquon",
				CAType:      string(DatabaseClientCA),
			},
			want: "db_client/zarquon",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, test.id.FullName(), "DisplayName mismatch")
		})
	}
}
