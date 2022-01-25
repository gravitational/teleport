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

package review

import (
	"testing"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"
	"github.com/stretchr/testify/require"
)

// TestIsInternal checks if docs and code reviewers show up as internal.
func TestIsInternal(t *testing.T) {
	tests := []struct {
		desc        string
		assignments *Assignments
		author      string
		expect      bool
	}{
		{
			desc: "code-is-internal",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
						"3": Reviewer{Team: "Core", Owner: false, GithubUsername: "user3"},
						"4": Reviewer{Team: "Core", Owner: false, GithubUsername: "user4"},
					},
					CodeReviewersOmit: map[string]bool{},
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"user5": Reviewer{Team: "Core", Owner: true},
						"user6": Reviewer{Team: "Core", Owner: true},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"user1",
						"user2",
					},
				},
			},
			author: "user1",
			expect: true,
		},
		{
			desc: "docs-is-internal",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
						"3": Reviewer{Team: "Core", Owner: false, GithubUsername: "user3"},
						"4": Reviewer{Team: "Core", Owner: false, GithubUsername: "user4"},
					},
					CodeReviewersOmit: map[string]bool{},
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"5": Reviewer{Team: "Core", Owner: true, GithubUsername: "user5"},
						"6": Reviewer{Team: "Core", Owner: true, GithubUsername: "user6"},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"user1",
						"user2",
					},
				},
			},
			author: "user5",
			expect: true,
		},
		{
			desc: "other-is-not-internal",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
						"3": Reviewer{Team: "Core", Owner: false, GithubUsername: "user3"},
						"4": Reviewer{Team: "Core", Owner: false, GithubUsername: "user4"},
					},
					CodeReviewersOmit: map[string]bool{},
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"5": Reviewer{Team: "Core", Owner: true, GithubUsername: "user5"},
						"6": Reviewer{Team: "Core", Owner: true, GithubUsername: "user6"},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"user1",
						"user2",
					},
				},
			},
			author: "user7",
			expect: false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			expect := test.assignments.IsInternal(test.author)
			require.Equal(t, expect, test.expect)
		})
	}
}

// TestGetCodeReviewers checks internal code review assignments.
func TestGetCodeReviewers(t *testing.T) {
	tests := []struct {
		desc        string
		assignments *Assignments
		author      string
		setA        []string
		setB        []string
	}{
		{
			desc: "skip-self-assign",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
						"3": Reviewer{Team: "Core", Owner: false, GithubUsername: "user3"},
						"4": Reviewer{Team: "Core", Owner: false, GithubUsername: "user4"},
					},
					CodeReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"user1",
						"user2",
					},
				},
			},
			author: "user1",
			setA:   []string{"user2"},
			setB:   []string{"user3", "user4"},
		},
		{
			desc: "skip-omitted-user",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
						"3": Reviewer{Team: "Core", Owner: false, GithubUsername: "user3"},
						"4": Reviewer{Team: "Core", Owner: false, GithubUsername: "user4"},
						"5": Reviewer{Team: "Core", Owner: false, GithubUsername: "user5"},
					},
					CodeReviewersOmit: map[string]bool{
						"user3": true,
					},
					// Admins.
					Admins: []string{
						"user1",
						"user2",
					},
				},
			},
			author: "user5",
			setA:   []string{"user1", "user2"},
			setB:   []string{"user4"},
		},
		{
			desc: "internal-gets-defaults",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
						"3": Reviewer{Team: "Core", Owner: false, GithubUsername: "user3"},
						"4": Reviewer{Team: "Core", Owner: false, GithubUsername: "user4"},
						"5": Reviewer{Team: "Internal", GithubUsername: "user5"},
					},
					CodeReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"user1",
						"user2",
					},
				},
			},
			author: "user5",
			setA:   []string{"user1"},
			setB:   []string{"user2"},
		},
		{
			desc: "normal",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
						"3": Reviewer{Team: "Core", Owner: true, GithubUsername: "user3"},
						"4": Reviewer{Team: "Core", Owner: false, GithubUsername: "user4"},
						"5": Reviewer{Team: "Core", Owner: false, GithubUsername: "user5"},
						"6": Reviewer{Team: "Core", Owner: false, GithubUsername: "user6"},
						"7": Reviewer{Team: "Internal", Owner: false, GithubUsername: "user7"},
					},
					CodeReviewersOmit: map[string]bool{
						"user6": true,
					},
					// Docs.
					DocsReviewers:     map[string]Reviewer{},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"user1",
						"user2",
					},
				},
			},
			author: "user4",
			setA:   []string{"user1", "user2", "user3"},
			setB:   []string{"user5"},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			setA, setB := test.assignments.getCodeReviewerSets(test.author)
			require.ElementsMatch(t, setA, test.setA)
			require.ElementsMatch(t, setB, test.setB)
		})
	}
}

// TestGetDocsReviewers checks internal docs review assignments.
func TestGetDocsReviewers(t *testing.T) {
	tests := []struct {
		desc        string
		assignments *Assignments
		author      string
		reviewers   []string
	}{
		{
			desc: "skip-self-assign",
			assignments: &Assignments{
				c: &Config{
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"user3",
						"user4",
					},
				},
			},
			author:    "user1",
			reviewers: []string{"user2"},
		},
		{
			desc: "skip-self-assign-with-omit",
			assignments: &Assignments{
				c: &Config{
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
					},
					DocsReviewersOmit: map[string]bool{
						"user2": true,
					},
					// Admins.
					Admins: []string{
						"user3",
						"user4",
					},
				},
			},
			author:    "user1",
			reviewers: []string{"user3", "user4"},
		},
		{
			desc: "normal",
			assignments: &Assignments{
				c: &Config{
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
						"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"user3",
						"user4",
					},
				},
			},
			author:    "user3",
			reviewers: []string{"user1", "user2"},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			reviewers := test.assignments.getDocsReviewers(test.author)
			require.ElementsMatch(t, reviewers, test.reviewers)
		})
	}
}

// TestCheckExternal checks external reviews.
func TestCheckExternal(t *testing.T) {
	r := &Assignments{
		c: &Config{
			// Code.
			CodeReviewers: map[string]Reviewer{
				"1": Reviewer{Team: "Core", Owner: true},
				"2": Reviewer{Team: "Core", Owner: true},
				"3": Reviewer{Team: "Core", Owner: true},
				"4": Reviewer{Team: "Core", Owner: false},
				"5": Reviewer{Team: "Core", Owner: false},
				"6": Reviewer{Team: "Core", Owner: false},
			},
			CodeReviewersOmit: map[string]bool{},
			// Default.
			Admins: []string{
				"1",
				"2",
			},
		},
	}
	tests := []struct {
		desc    string
		author  string
		reviews map[string]*github.Review
		result  bool
	}{
		{
			desc:    "no-reviews-fail",
			author:  "5",
			reviews: map[string]*github.Review{},
			result:  false,
		},
		{
			desc:   "two-non-admin-reviews-fail",
			author: "5",
			reviews: map[string]*github.Review{
				"3": &github.Review{
					Author: "3",
					State:  approved,
				},
				"4": &github.Review{
					Author: "4",
					State:  approved,
				},
			},
			result: false,
		},
		{
			desc:   "one-admin-reviews-fail",
			author: "5",
			reviews: map[string]*github.Review{
				"1": &github.Review{
					Author: "1",
					State:  approved,
				},
				"4": &github.Review{
					Author: "4",
					State:  approved,
				},
			},
			result: false,
		},
		{
			desc:   "two-admin-reviews-one-denied-success",
			author: "5",
			reviews: map[string]*github.Review{
				"1": &github.Review{
					Author: "1",
					State:  changesRequested,
				},
				"2": &github.Review{
					Author: "2",
					State:  approved,
				},
			},
			result: false,
		},
		{
			desc:   "two-admin-reviews-success",
			author: "5",
			reviews: map[string]*github.Review{
				"1": &github.Review{
					Author: "1",
					State:  approved,
				},
				"2": &github.Review{
					Author: "2",
					State:  approved,
				},
			},
			result: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := r.CheckExternal(test.author, test.reviews)
			if test.result {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestCheckInternal checks internal reviews.
func TestCheckInternal(t *testing.T) {
	r := &Assignments{
		c: &Config{
			// Code.
			CodeReviewers: map[string]Reviewer{
				"1": Reviewer{Team: "Core", Owner: true, GithubUsername: "user1"},
				"2": Reviewer{Team: "Core", Owner: true, GithubUsername: "user2"},
				"3": Reviewer{Team: "Core", Owner: true, GithubUsername: "user3"},
				"9": Reviewer{Team: "Core", Owner: true, GithubUsername: "user9"},
				"4": Reviewer{Team: "Core", Owner: false, GithubUsername: "user4"},
				"5": Reviewer{Team: "Core", Owner: false, GithubUsername: "user5"},
				"6": Reviewer{Team: "Core", Owner: false, GithubUsername: "user6"},
				"8": Reviewer{Team: "Internal", Owner: false, GithubUsername: "user8"},
			},
			// Docs.
			DocsReviewers: map[string]Reviewer{
				"7": Reviewer{Team: "Core", Owner: true, GithubUsername: "user7"},
			},
			DocsReviewersOmit: map[string]bool{},
			CodeReviewersOmit: map[string]bool{},
			// Default.
			Admins: []string{
				"user1",
				"user2",
			},
		},
	}
	tests := []struct {
		desc    string
		author  string
		reviews map[string]*github.Review
		docs    bool
		code    bool
		result  bool
	}{
		{
			desc:    "no-reviews-fail",
			author:  "user4",
			reviews: map[string]*github.Review{},
			result:  false,
		},
		{
			desc:    "docs-only-no-reviews-fail",
			author:  "user4",
			reviews: map[string]*github.Review{},
			docs:    true,
			code:    false,
			result:  false,
		},
		{
			desc:   "docs-only-non-docs-approval-fail",
			author: "user4",
			reviews: map[string]*github.Review{
				"user3": &github.Review{Author: "user3", State: approved},
			},
			docs:   true,
			code:   false,
			result: false,
		},
		{
			desc:   "docs-only-docs-approval-success",
			author: "user4",
			reviews: map[string]*github.Review{
				"user7": &github.Review{Author: "user7", State: approved},
			},
			docs:   true,
			code:   false,
			result: true,
		},
		{
			desc:    "code-only-no-reviews-fail",
			author:  "user4",
			reviews: map[string]*github.Review{},
			docs:    false,
			code:    true,
			result:  false,
		},
		{
			desc:   "code-only-one-approval-fail",
			author: "user4",
			reviews: map[string]*github.Review{
				"user3": &github.Review{Author: "user3", State: approved},
			},
			docs:   false,
			code:   true,
			result: false,
		},
		{
			desc:   "code-only-two-approval-setb-fail",
			author: "user4",
			reviews: map[string]*github.Review{
				"user5": &github.Review{Author: "user5", State: approved},
				"user6": &github.Review{Author: "user6", State: approved},
			},
			docs:   false,
			code:   true,
			result: false,
		},
		{
			desc:   "code-only-one-changes-fail",
			author: "user4",
			reviews: map[string]*github.Review{
				"user3": &github.Review{Author: "user3", State: approved},
				"user4": &github.Review{Author: "user4", State: changesRequested},
			},
			docs:   false,
			code:   true,
			result: false,
		},
		{
			desc:   "code-only-two-approvals-success",
			author: "user6",
			reviews: map[string]*github.Review{
				"user3": &github.Review{Author: "user3", State: approved},
				"user4": &github.Review{Author: "user4", State: approved},
			},
			docs:   false,
			code:   true,
			result: true,
		},
		{
			desc:   "docs-and-code-only-docs-approval-fail",
			author: "user6",
			reviews: map[string]*github.Review{
				"user7": &github.Review{Author: "user7", State: approved},
			},
			docs:   true,
			code:   true,
			result: false,
		},
		{
			desc:   "docs-and-code-only-code-approval-fail",
			author: "user6",
			reviews: map[string]*github.Review{
				"user3": &github.Review{Author: "user3", State: approved},
				"user4": &github.Review{Author: "user4", State: approved},
			},
			docs:   true,
			code:   true,
			result: false,
		},
		{
			desc:   "docs-and-code-docs-and-code-approval-success",
			author: "user6",
			reviews: map[string]*github.Review{
				"user3": &github.Review{Author: "user3", State: approved},
				"user4": &github.Review{Author: "user4", State: approved},
				"user7": &github.Review{Author: "user7", State: approved},
			},
			docs:   true,
			code:   true,
			result: true,
		},
		{
			desc:   "code-only-internal-on-approval-failure",
			author: "user8",
			reviews: map[string]*github.Review{
				"user3": &github.Review{Author: "user3", State: approved},
			},
			docs:   false,
			code:   true,
			result: false,
		},
		{
			desc:   "code-only-internal-code-approval-success",
			author: "user8",
			reviews: map[string]*github.Review{
				"user3": &github.Review{Author: "user3", State: approved},
				"user4": &github.Review{Author: "user4", State: approved},
			},
			docs:   false,
			code:   true,
			result: true,
		},
		{
			desc:   "code-only-internal-two-code-owner-approval-success",
			author: "user4",
			reviews: map[string]*github.Review{
				"user3": &github.Review{Author: "user3", State: approved},
				"user9": &github.Review{Author: "user9", State: approved},
			},
			docs:   false,
			code:   true,
			result: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := r.CheckInternal(test.author, test.reviews, test.docs, test.code)
			if test.result {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestFromString tests if configuration is correctly read in from a string.
func TestFromString(t *testing.T) {
	r, err := FromString(reviewers)
	require.NoError(t, err)

	require.EqualValues(t, r.c.CodeReviewers, map[string]Reviewer{
		"1": Reviewer{
			Team:  "Core",
			Owner: true,
			GithubUsername: "user1",
		},
		"2": Reviewer{
			Team:  "Core",
			Owner: false,
			GithubUsername: "user2",
		},
	})
	require.EqualValues(t, r.c.CodeReviewersOmit, map[string]bool{
		"user3": true,
	})
	require.EqualValues(t, r.c.DocsReviewers, map[string]Reviewer{
		"4": Reviewer{
			Team:  "Core",
			Owner: true,
			GithubUsername: "user4",
		},
		"5": Reviewer{
			Team:  "Core",
			Owner: false,
			GithubUsername: "user5",
		},
	})
	require.EqualValues(t, r.c.DocsReviewersOmit, map[string]bool{
		"user6": true,
	})
	require.EqualValues(t, r.c.Admins, []string{
		"user7",
		"user8",
	})
}

const reviewers = `
{
	"codeReviewers": {
		"1": {
			"team": "Core",
			"owner": true, 
			"username": "user1"
		},
		"2": {
			"team": "Core",
			"owner": false,
			"username": "user2"
		}
	},
	"codeReviewersOmit": {
		"user3": true
    },
	"docsReviewers": {
		"4": {
			"team": "Core",
			"owner": true,
			"username": "user4"
		},
		"5": {
			"team": "Core",
			"owner": false, 
			"username": "user5"
		}
	},	
	"docsReviewersOmit": {
		"user6": true
    },
	"admins": [
		"user7",
		"user8"
	]
}
`
