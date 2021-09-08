package environment

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
	"github.com/stretchr/testify/require"
)

func TestNewEnvironment(t *testing.T) {
	tests := []struct {
		cfg        Config
		checkErr   require.ErrorAssertionFunc
		expected   *Environment
		desc       string
		createFile bool
	}{
		{
			cfg: Config{
				Client:               github.NewClient(nil),
				DefaultReviewers:     "[\"admin\"]",
				EventPath:            "",
				unmarshalRevs:        UnmarshalReviewersTest,
				unmarshalDefaultRevs: UnmarshalDefaultReviewersTest,
				Token:                "testtoken",
			},
			checkErr:   require.Error,
			desc:       "invalid Environment config with Reviewers parameter",
			expected:   nil,
			createFile: true,
		},
		{
			cfg: Config{
				Client:               github.NewClient(nil),
				DefaultReviewers:     "[\"admin\"]",
				Reviewers:            `{"foo": ["bar", "baz"]}`,
				unmarshalRevs:        UnmarshalReviewersTest,
				unmarshalDefaultRevs: UnmarshalDefaultReviewersTest,
				Token:                "testtoken",
			},
			checkErr: require.NoError,
			desc:     "valid Environment config",
			expected: &Environment{
				reviewers: map[string][]string{"foo": {"bar", "baz"}},
				Client:    github.NewClient(nil),
			},
			createFile: true,
		},
		{
			cfg: Config{
				Reviewers:            `{"foo": "baz"}`,
				Client:               github.NewClient(nil),
				DefaultReviewers:     "[\"admin\"]",
				unmarshalRevs:        UnmarshalReviewersTest,
				unmarshalDefaultRevs: UnmarshalDefaultReviewersTest,
				Token:                "testtoken",
			},
			checkErr:   require.Error,
			desc:       "invalid reviewers object format",
			expected:   nil,
			createFile: true,
		},
		{
			cfg: Config{
				unmarshalRevs:        UnmarshalReviewersTest,
				unmarshalDefaultRevs: UnmarshalDefaultReviewersTest,
				Token:                "testtoken",
			},
			checkErr:   require.Error,
			desc:       "invalid config with no client",
			expected:   nil,
			createFile: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if test.createFile {
				// Valid config
				f, err := ioutil.TempFile("", "payload")
				require.NoError(t, err)
				filePath := f.Name()
				defer os.Remove(f.Name())
				_, err = f.Write([]byte(pullRequest))
				require.NoError(t, err)
				test.cfg.EventPath = filePath
			}
			_, err := New(test.cfg)
			test.checkErr(t, err)
		})
	}
}

func TestSetPullRequest(t *testing.T) {
	tests := []struct {
		checkErr require.ErrorAssertionFunc
		env      *Environment
		input    []byte
		desc     string
		value    *PullRequestMetadata
	}{
		{
			env:      &Environment{action: "synchronize"},
			checkErr: require.NoError,
			input:    []byte(synchronize),
			value: &PullRequestMetadata{Author: "quinqu",
				RepoName:   "gh-actions-poc",
				RepoOwner:  "gravitational",
				Number:     28,
				HeadSHA:    "ecabd9d97b218368ea47d17cd23815590b76e196",
				BaseSHA:    "cbb23161d4c33d70189430d07957d2d66d42fc30",
				BranchName: "jane/ci",
			},
			desc: "sync, no error",
		},
		{
			env:      &Environment{action: "opened"},
			checkErr: require.NoError,
			input:    []byte(pullRequest),
			value: &PullRequestMetadata{Author: "Codertocat",
				RepoName:   "Hello-World",
				RepoOwner:  "Codertocat",
				Number:     2,
				HeadSHA:    "ec26c3e57ca3a959ca5aad62de7213c562f8c821",
				BaseSHA:    "f95f852bd8fca8fcc58a9a2d6c842781e32a215e",
				BranchName: "changes",
			},
			desc: "pull request, no error",
		},
		{
			env:      &Environment{action: "submitted"},
			checkErr: require.NoError,
			input:    []byte(submitted),
			value: &PullRequestMetadata{Author: "Codertocat",
				RepoName:   "Hello-World",
				RepoOwner:  "Codertocat",
				Number:     2,
				HeadSHA:    "ec26c3e57ca3a959ca5aad62de7213c562f8c821",
				BaseSHA:    "f95f852bd8fca8fcc58a9a2d6c842781e32a215e",
				BranchName: "changes",
				Reviewer:   "Codertocat",
			},
			desc: "review, no error",
		},

		{
			env:      &Environment{action: "synchronize"},
			checkErr: require.Error,
			input:    []byte(submitted),
			value:    nil,
			desc:     "sync, error",
		},
		{
			env:      &Environment{action: "opened"},
			checkErr: require.Error,
			input:    []byte(submitted),
			value:    nil,
			desc:     "pull request, error",
		},
		{
			env:      &Environment{action: "submitted"},
			checkErr: require.Error,
			input:    []byte(pullRequest),
			value:    nil,
			desc:     "review, error",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.env.setPullRequest(test.input)
			test.checkErr(t, err)
			require.Equal(t, test.value, test.env.PullRequest)
		})
	}

}

func UnmarshalReviewersTest(str string, client *github.Client) (map[string][]string, error) {
	if str == "" {
		return nil, trace.BadParameter("reviewers not found.")
	}
	m := make(map[string][]string)
	err := json.Unmarshal([]byte(str), &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func UnmarshalDefaultReviewersTest(str string, client *github.Client) ([]string, error) {
	str = strings.Trim(str, "[")
	str = strings.Trim(str, "]")
	reviewers := strings.Split(str, ",")
	defaultReviewers := []string{}
	defaultReviewers = append(defaultReviewers, reviewers...)
	return defaultReviewers, nil
}
