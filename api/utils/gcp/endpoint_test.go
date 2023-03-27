// Copyright 2022 Gravitational, Inc
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

package gcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsGCPEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{
			name:     "compute googleapis",
			hostname: "compute.googleapis.com",
			want:     true,
		},
		{
			name:     "top level googleapis",
			hostname: "googleapis.com",
			want:     false,
		},
		{
			name:     "localhost",
			hostname: "localhost",
			want:     false,
		},
		{
			name:     "fake googleapis",
			hostname: "compute.googleapis.com.fake.com",
			want:     false,
		},
		{
			name:     "top level-like fake googleapis",
			hostname: "googleapis.com.fake.com",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsGCPEndpoint(tt.hostname))
		})
	}
}
