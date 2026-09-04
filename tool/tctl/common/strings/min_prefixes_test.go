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

package strings_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	tctlstrings "github.com/gravitational/teleport/tool/tctl/common/strings"
)

func TestFindMinPrefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in, want []string
	}{
		{
			name: "single hash",
			in: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
			},
			want: []string{
				"f4522365",
			},
		},
		{
			name: "multiple hashes",
			in: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"11b52b511de1f0d8c4b5e5a3beb053fb5497727d696de6dae338560e4e2f8e0c",
			},
			want: []string{
				"f4522365",
				"11b52b51",
			},
		},
		{
			name: "conflicts",
			in: []string{
				"bananallama11111",
				"bananallama21111",
				"bananallama31111",
			},
			want: []string{
				"bananallama1",
				"bananallama2",
				"bananallama3",
			},
		},
		{
			name: "duplicate hashes",
			in: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"11b52b511de1f0d8c4b5e5a3beb053fb5497727d696de6dae338560e4e2f8e0c",
			},
			want: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc",
				"11b52b511de1f0d8c4b5e5a3beb053fb5497727d696de6dae338560e4e2f8e0c",
			},
		},
		{
			name: "uneven/small hashes",
			in: []string{
				"f4522365888fdddcf3c854e79e5928447fe1a2388353efb2f0d30db8ba7c81bc", // normal len
				"aaaaaaa",       // <8 characters, aka <minLen.
				"bananallama1a", // clashes below.
				"bananallama2a", // clashes above.
			},
			want: []string{
				"f4522365888f", // trimmed
				"aaaaaaa",      // original (<minLen)
				"bananallama1", // trimmed
				"bananallama2", // trimmed
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := tctlstrings.FindMinPrefixes(test.in)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("FindMinPrefixes mismatch (-want +got)\n%s", diff)
			}
		})
	}
}
