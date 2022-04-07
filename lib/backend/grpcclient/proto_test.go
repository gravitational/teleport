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

package grpcclient

import (
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/stretchr/testify/require"

	bpb "github.com/gravitational/teleport/api/backend/proto"
)

func TestGrpcItemsToBackendItems(t *testing.T) {
	testCases := []struct {
		desc string
		in   []*bpb.Item
		want []backend.Item
	}{
		{
			desc: "test a single item",
			in: []*bpb.Item{
				{Key: []byte("hello"), Value: []byte("world")},
			},
			want: []backend.Item{
				{Key: []byte("hello"), Value: []byte("world")},
			},
		},
		{
			desc: "test multiple items",
			in: []*bpb.Item{
				{Key: []byte("hello"), Value: []byte("world")},
				{Key: []byte("another element"), Value: []byte("here")},
			},
			want: []backend.Item{
				{Key: []byte("hello"), Value: []byte("world")},
				{Key: []byte("another element"), Value: []byte("here")},
			},
		},
		{
			desc: "test no items",
			in:   []*bpb.Item{},
			want: []backend.Item{},
		},
		{
			desc: "test nil items",
			in:   nil,
			want: []backend.Item{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := grpcItemsToBackendItems(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
