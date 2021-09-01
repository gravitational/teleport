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
	envSync := &Environment{action: "synchronize"}
	envPull := &Environment{action: "opened"}
	envReview := &Environment{action: "submitted"}

	tests := []struct {
		checkErr require.ErrorAssertionFunc
		env      *Environment
		input    []byte
		desc     string
	}{
		{
			env:      envSync,
			checkErr: require.NoError,
			input:    []byte(synchronize),
			desc:     "sync, no error",
		},
		{
			env:      envPull,
			checkErr: require.NoError,
			input:    []byte(pullRequest),
			desc:     "pull request, no error",
		},
		{
			env:      envReview,
			checkErr: require.NoError,
			input:    []byte(submitted),
			desc:     "review, no error",
		},

		{
			env:      envSync,
			checkErr: require.Error,
			input:    []byte(submitted),
			desc:     "sync, error",
		},
		{
			env:      envPull,
			checkErr: require.Error,
			input:    []byte(submitted),
			desc:     "pull request, error",
		},
		{
			env:      envReview,
			checkErr: require.Error,
			input:    []byte(pullRequest),
			desc:     "review, error",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.env.setPullRequest(test.input)
			test.checkErr(t, err)
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
