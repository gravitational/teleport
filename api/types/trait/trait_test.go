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

package trait

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestMerge(t *testing.T) {
	testCases := []struct {
		name        string
		dst         Traits
		src         Traits
		expectedDst Traits
	}{
		{
			name: "typical",
			dst: Traits{
				"only_dst": []string{"dst1"},
				"logins":   []string{"root", "ec2-user"},
			},
			src: Traits{
				"only_src": []string{"src1"},
				"logins":   []string{"ubuntu", "ec2-user"},
			},
			expectedDst: Traits{
				"only_src": []string{"src1"},
				"only_dst": []string{"dst1"},
				"logins":   []string{"ec2-user", "root", "ubuntu"},
			},
		},
		{
			name: "empty_dst",
			dst:  Traits{},
			src: Traits{
				"only_src": []string{"src1"},
			},
			expectedDst: Traits{
				"only_src": []string{"src1"},
			},
		},
	}
	for _, tt := range testCases {
		Merge(tt.dst, tt.src)
		require.Empty(t, cmp.Diff(tt.expectedDst, tt.dst))
	}
}
