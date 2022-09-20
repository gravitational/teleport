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

package env

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEnvironment makes sure the environment is correctly parsed.
func TestEnvironment(t *testing.T) {
	tests := []struct {
		desc         string
		path         string
		organization string
		repository   string
		number       int
		author       string
		unsafeBranch string
		err          bool
	}{
		{
			desc:         "opened-event",
			path:         "testdata/opened.json",
			organization: "Codertocat",
			repository:   "Hello-World",
			number:       2,
			author:       "Codertocat",
			unsafeBranch: "changes",
		},
		{
			desc:         "submitted-event",
			path:         "testdata/submitted.json",
			organization: "Codertocat",
			repository:   "Hello-World",
			number:       2,
			author:       "Codertocat",
			unsafeBranch: "changes",
		},
		{
			desc:         "synchronize-event",
			path:         "testdata/submitted.json",
			organization: "Codertocat",
			repository:   "Hello-World",
			number:       2,
			author:       "Codertocat",
			unsafeBranch: "changes",
		},
		{
			desc:         "schedule-event",
			path:         "testdata/schedule.json",
			organization: "foo",
			repository:   "bar",
			number:       0,
			author:       "",
			unsafeBranch: "",
		},
		{
			desc:         "no-event",
			path:         "",
			organization: "foo",
			repository:   "bar",
			err:          true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := os.Setenv(githubRepository, "foo/bar")
			require.NoError(t, err)
			err = os.Setenv(githubEventPath, test.path)
			require.NoError(t, err)

			environment, err := New()
			if test.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, environment.Organization, test.organization)
				require.Equal(t, environment.Repository, test.repository)
				require.Equal(t, environment.Number, test.number)
				require.Equal(t, environment.Author, test.author)
				require.Equal(t, environment.UnsafeBranch, test.unsafeBranch)
			}
		})
	}

}
