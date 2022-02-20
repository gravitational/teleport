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

package bot

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/env"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/review"

	"github.com/stretchr/testify/require"
)

func TestCheck(t *testing.T) {
	r, err := review.New(&review.Config{
		// Code.
		CodeReviewers: map[string]review.Reviewer{
			"1": review.Reviewer{Team: "Core", Owner: true},
			"2": review.Reviewer{Team: "Core", Owner: true},
			"3": review.Reviewer{Team: "Core", Owner: false},
			"4": review.Reviewer{Team: "Core", Owner: false},
			"5": review.Reviewer{Team: "Core", Owner: false},
		},
		CodeReviewersOmit: map[string]bool{},
		DocsReviewers:     map[string]review.Reviewer{},
		DocsReviewersOmit: map[string]bool{},
		// Admins.
		Admins: []string{
			"6",
			"7",
		},
	})
	require.NoError(t, err)

	tests := []struct {
		desc    string
		author  string
		files   []string
		reviews map[string]*github.Review
		result  bool
	}{
		{
			desc:   "no-approvals-fails",
			author: "3",
			files: []string{
				"file.go",
			},
			reviews: map[string]*github.Review{},
			result:  false,
		},
		{
			desc:   "approvals-without-tests-fails",
			author: "3",
			files: []string{
				"file.go",
			},
			reviews: map[string]*github.Review{
				"1": &github.Review{Author: "1", State: "APPROVED"},
				"4": &github.Review{Author: "4", State: "APPROVED"},
			},
			result: false,
		},
		{
			desc:   "approvals-without-tests-requires-admin-success",
			author: "3",
			files: []string{
				"file.go",
			},
			reviews: map[string]*github.Review{
				"1": &github.Review{Author: "1", State: "APPROVED"},
				"4": &github.Review{Author: "4", State: "APPROVED"},
				"6": &github.Review{Author: "6", State: "APPROVED"},
			},
			result: true,
		},
		{
			desc:   "approvals-and-tests-success",
			author: "3",
			files: []string{
				"file.go",
				"file_test.go",
			},
			reviews: map[string]*github.Review{
				"1": &github.Review{Author: "1", State: "APPROVED"},
				"4": &github.Review{Author: "4", State: "APPROVED"},
			},
			result: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			b := &Bot{
				c: &Config{
					Environment: &env.Environment{
						Organization: "foo",
						Repository:   "bar",
						Number:       0,
					},
					GitHub: &fakeGithub{
						test.files,
					},
					Review: r,
				},
			}
			err := b.checkTests(context.Background(), test.author, test.reviews)
			if test.result {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
