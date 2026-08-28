// Copyright 2025 Gravitational, Inc.
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

package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaxClientRecvMsgSize(t *testing.T) {

	testCases := []struct {
		desc  string
		size  string
		bytes int
	}{
		{
			desc:  "Decimal",
			size:  "1234",
			bytes: 1234,
		},
		{
			desc:  "Unset",
			size:  "",
			bytes: defaultClientRecvSize,
		},
		{
			desc:  "Unhandled units",
			size:  "4TB",
			bytes: defaultClientRecvSize,
		},
		{
			desc:  "Too large",
			size:  "20GB",
			bytes: defaultClientRecvSize,
		},
		{
			desc:  "Rubbish",
			size:  "foobar",
			bytes: defaultClientRecvSize,
		},
		{
			desc:  "Human mib",
			size:  "8mib",
			bytes: 8 * 1024 * 1024,
		},
		{
			desc:  "Human kib",
			size:  "8kib",
			bytes: 8 * 1024,
		},
		{
			desc:  "Human mb",
			size:  "8mb",
			bytes: 8 * 1000 * 1000,
		},
		{
			desc:  "Floats",
			size:  "2.5kb",
			bytes: 2500,
		},
		{
			desc:  "Human kb",
			size:  "8kb",
			bytes: 8 * 1000,
		},
		{
			desc:  "Human m",
			size:  "8m",
			bytes: 8 * 1000 * 1000,
		},
		{
			desc:  "Human k",
			size:  "8k",
			bytes: 8 * 1000,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			t.Setenv("TELEPORT_UNSTABLE_GRPC_RECV_SIZE", tt.size)
			assert.Equal(t, tt.bytes, MaxClientRecvMsgSize())
		})
	}
}
