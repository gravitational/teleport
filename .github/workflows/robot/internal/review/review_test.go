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
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
						"3": Reviewer{Team: "Core", Owner: false},
						"4": Reviewer{Team: "Core", Owner: false},
					},
					CodeReviewersOmit: map[string]bool{},
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"5": Reviewer{Team: "Core", Owner: true},
						"6": Reviewer{Team: "Core", Owner: true},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"1",
						"2",
					},
				},
			},
			author: "1",
			expect: true,
		},
		{
			desc: "docs-is-internal",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
						"3": Reviewer{Team: "Core", Owner: false},
						"4": Reviewer{Team: "Core", Owner: false},
					},
					CodeReviewersOmit: map[string]bool{},
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"5": Reviewer{Team: "Core", Owner: true},
						"6": Reviewer{Team: "Core", Owner: true},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"1",
						"2",
					},
				},
			},
			author: "5",
			expect: true,
		},
		{
			desc: "other-is-not-internal",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
						"3": Reviewer{Team: "Core", Owner: false},
						"4": Reviewer{Team: "Core", Owner: false},
					},
					CodeReviewersOmit: map[string]bool{},
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"5": Reviewer{Team: "Core", Owner: true},
						"6": Reviewer{Team: "Core", Owner: true},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"1",
						"2",
					},
				},
			},
			author: "7",
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
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
						"3": Reviewer{Team: "Core", Owner: false},
						"4": Reviewer{Team: "Core", Owner: false},
					},
					CodeReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"1",
						"2",
					},
				},
			},
			author: "1",
			setA:   []string{"2"},
			setB:   []string{"3", "4"},
		},
		{
			desc: "skip-omitted-user",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
						"3": Reviewer{Team: "Core", Owner: false},
						"4": Reviewer{Team: "Core", Owner: false},
						"5": Reviewer{Team: "Core", Owner: false},
					},
					CodeReviewersOmit: map[string]bool{
						"3": true,
					},
					// Admins.
					Admins: []string{
						"1",
						"2",
					},
				},
			},
			author: "5",
			setA:   []string{"1", "2"},
			setB:   []string{"4"},
		},
		{
			desc: "internal-gets-defaults",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
						"3": Reviewer{Team: "Core", Owner: false},
						"4": Reviewer{Team: "Core", Owner: false},
						"5": Reviewer{Team: "Internal"},
					},
					CodeReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"1",
						"2",
					},
				},
			},
			author: "5",
			setA:   []string{"1"},
			setB:   []string{"2"},
		},
		{
			desc: "normal",
			assignments: &Assignments{
				c: &Config{
					// Code.
					CodeReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
						"3": Reviewer{Team: "Core", Owner: true},
						"4": Reviewer{Team: "Core", Owner: false},
						"5": Reviewer{Team: "Core", Owner: false},
						"6": Reviewer{Team: "Core", Owner: false},
						"7": Reviewer{Team: "Internal", Owner: false},
					},
					CodeReviewersOmit: map[string]bool{
						"6": true,
					},
					// Docs.
					DocsReviewers:     map[string]Reviewer{},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"1",
						"2",
					},
				},
			},
			author: "4",
			setA:   []string{"1", "2", "3"},
			setB:   []string{"5"},
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
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"3",
						"4",
					},
				},
			},
			author:    "1",
			reviewers: []string{"2"},
		},
		{
			desc: "skip-self-assign-with-omit",
			assignments: &Assignments{
				c: &Config{
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
					},
					DocsReviewersOmit: map[string]bool{
						"2": true,
					},
					// Admins.
					Admins: []string{
						"3",
						"4",
					},
				},
			},
			author:    "1",
			reviewers: []string{"3", "4"},
		},
		{
			desc: "normal",
			assignments: &Assignments{
				c: &Config{
					// Docs.
					DocsReviewers: map[string]Reviewer{
						"1": Reviewer{Team: "Core", Owner: true},
						"2": Reviewer{Team: "Core", Owner: true},
					},
					DocsReviewersOmit: map[string]bool{},
					// Admins.
					Admins: []string{
						"3",
						"4",
					},
				},
			},
			author:    "3",
			reviewers: []string{"1", "2"},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			reviewers := test.assignments.getDocsReviewers(test.author)
			require.ElementsMatch(t, reviewers, test.reviewers)
		})
	}
}

// TestCheckAdmin checks external reviews.
func TestCheckAdmin(t *testing.T) {
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
			err := r.CheckAdmin(test.author, test.reviews, 2)
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
				"1": Reviewer{Team: "Core", Owner: true},
				"2": Reviewer{Team: "Core", Owner: true},
				"3": Reviewer{Team: "Core", Owner: true},
				"9": Reviewer{Team: "Core", Owner: true},
				"4": Reviewer{Team: "Core", Owner: false},
				"5": Reviewer{Team: "Core", Owner: false},
				"6": Reviewer{Team: "Core", Owner: false},
				"8": Reviewer{Team: "Internal", Owner: false},
			},
			// Docs.
			DocsReviewers: map[string]Reviewer{
				"7": Reviewer{Team: "Core", Owner: true},
			},
			DocsReviewersOmit: map[string]bool{},
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
		docs    bool
		code    bool
		result  bool
	}{
		{
			desc:    "no-reviews-fail",
			author:  "4",
			reviews: map[string]*github.Review{},
			result:  false,
		},
		{
			desc:    "docs-only-no-reviews-fail",
			author:  "4",
			reviews: map[string]*github.Review{},
			docs:    true,
			code:    false,
			result:  false,
		},
		{
			desc:   "docs-only-non-docs-approval-fail",
			author: "4",
			reviews: map[string]*github.Review{
				"3": &github.Review{Author: "3", State: approved},
			},
			docs:   true,
			code:   false,
			result: false,
		},
		{
			desc:   "docs-only-docs-approval-success",
			author: "4",
			reviews: map[string]*github.Review{
				"7": &github.Review{Author: "7", State: approved},
			},
			docs:   true,
			code:   false,
			result: true,
		},
		{
			desc:    "code-only-no-reviews-fail",
			author:  "4",
			reviews: map[string]*github.Review{},
			docs:    false,
			code:    true,
			result:  false,
		},
		{
			desc:   "code-only-one-approval-fail",
			author: "4",
			reviews: map[string]*github.Review{
				"3": &github.Review{Author: "3", State: approved},
			},
			docs:   false,
			code:   true,
			result: false,
		},
		{
			desc:   "code-only-two-approval-setb-fail",
			author: "4",
			reviews: map[string]*github.Review{
				"5": &github.Review{Author: "5", State: approved},
				"6": &github.Review{Author: "6", State: approved},
			},
			docs:   false,
			code:   true,
			result: false,
		},
		{
			desc:   "code-only-one-changes-fail",
			author: "4",
			reviews: map[string]*github.Review{
				"3": &github.Review{Author: "3", State: approved},
				"4": &github.Review{Author: "4", State: changesRequested},
			},
			docs:   false,
			code:   true,
			result: false,
		},
		{
			desc:   "code-only-two-approvals-success",
			author: "6",
			reviews: map[string]*github.Review{
				"3": &github.Review{Author: "3", State: approved},
				"4": &github.Review{Author: "4", State: approved},
			},
			docs:   false,
			code:   true,
			result: true,
		},
		{
			desc:   "docs-and-code-only-docs-approval-fail",
			author: "6",
			reviews: map[string]*github.Review{
				"7": &github.Review{Author: "7", State: approved},
			},
			docs:   true,
			code:   true,
			result: false,
		},
		{
			desc:   "docs-and-code-only-code-approval-fail",
			author: "6",
			reviews: map[string]*github.Review{
				"3": &github.Review{Author: "3", State: approved},
				"4": &github.Review{Author: "4", State: approved},
			},
			docs:   true,
			code:   true,
			result: false,
		},
		{
			desc:   "docs-and-code-docs-and-code-approval-success",
			author: "6",
			reviews: map[string]*github.Review{
				"3": &github.Review{Author: "3", State: approved},
				"4": &github.Review{Author: "4", State: approved},
				"7": &github.Review{Author: "7", State: approved},
			},
			docs:   true,
			code:   true,
			result: true,
		},
		{
			desc:   "code-only-internal-on-approval-failure",
			author: "8",
			reviews: map[string]*github.Review{
				"3": &github.Review{Author: "3", State: approved},
			},
			docs:   false,
			code:   true,
			result: false,
		},
		{
			desc:   "code-only-internal-code-approval-success",
			author: "8",
			reviews: map[string]*github.Review{
				"3": &github.Review{Author: "3", State: approved},
				"4": &github.Review{Author: "4", State: approved},
			},
			docs:   false,
			code:   true,
			result: true,
		},
		{
			desc:   "code-only-internal-two-code-owner-approval-success",
			author: "4",
			reviews: map[string]*github.Review{
				"3": &github.Review{Author: "3", State: approved},
				"9": &github.Review{Author: "9", State: approved},
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
		},
		"2": Reviewer{
			Team:  "Core",
			Owner: false,
		},
	})
	require.EqualValues(t, r.c.CodeReviewersOmit, map[string]bool{
		"3": true,
	})
	require.EqualValues(t, r.c.DocsReviewers, map[string]Reviewer{
		"4": Reviewer{
			Team:  "Core",
			Owner: true,
		},
		"5": Reviewer{
			Team:  "Core",
			Owner: false,
		},
	})
	require.EqualValues(t, r.c.DocsReviewersOmit, map[string]bool{
		"6": true,
	})
	require.EqualValues(t, r.c.Admins, []string{
		"7",
		"8",
	})
}

const reviewers = `
{
	"codeReviewers": {
		"1": {
			"team": "Core",
			"owner": true
		},
		"2": {
			"team": "Core",
			"owner": false
		}
	},
	"codeReviewersOmit": {
		"3": true
    },
	"docsReviewers": {
		"4": {
			"team": "Core",
			"owner": true
		},
		"5": {
			"team": "Core",
			"owner": false
		}
	},	
	"docsReviewersOmit": {
		"6": true
    },
	"admins": [
		"7",
		"8"
	]
}
`
