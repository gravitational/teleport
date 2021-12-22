/*
Copyright 2021 Gravitational, Inc.

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

package bot

import (
	"testing"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"

	"github.com/stretchr/testify/require"
)

// TestSkipUpdate checks if PRs are appropriately skipped over.
func TestSkipUpdate(t *testing.T) {
	tests := []struct {
		desc       string
		pr         github.PullRequest
		skipUpdate bool
	}{
		{
			desc: "fork-skip",
			pr: github.PullRequest{
				Author:     "foo",
				Repository: "bar",
				UnsafeHead: "baz/qux",
				UnsafeBase: "master",
				Fork:       true,
				AutoMerge:  true,
				Mergeable:  false,
			},
			skipUpdate: true,
		},
		{
			desc: "non-master-skip",
			pr: github.PullRequest{
				Author:     "foo",
				Repository: "bar",
				UnsafeHead: "baz/qux",
				UnsafeBase: "branch/v0",
				Fork:       false,
				AutoMerge:  true,
				Mergeable:  false,
			},
			skipUpdate: true,
		},
		{
			desc: "no-auto-merge-skip",
			pr: github.PullRequest{
				Author:     "foo",
				Repository: "bar",
				UnsafeHead: "baz/qux",
				UnsafeBase: "master",
				Fork:       false,
				AutoMerge:  false,
				Mergeable:  false,
			},
			skipUpdate: true,
		},
		{
			desc: "mergeable-skip",
			pr: github.PullRequest{
				Author:     "foo",
				Repository: "bar",
				UnsafeHead: "baz/qux",
				UnsafeBase: "master",
				Fork:       false,
				AutoMerge:  true,
				Mergeable:  true,
			},
			skipUpdate: true,
		},
		{
			desc: "allow",
			pr: github.PullRequest{
				Author:     "foo",
				Repository: "bar",
				UnsafeHead: "baz/qux",
				UnsafeBase: "master",
				Fork:       false,
				AutoMerge:  true,
				Mergeable:  false,
			},
			skipUpdate: false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ok := skipUpdate(test.pr)
			require.Equal(t, ok, test.skipUpdate)
		})
	}
}
