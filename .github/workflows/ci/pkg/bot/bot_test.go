package bot

import (
	"testing"

	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"

	"github.com/google/go-github/v37/github"
	"github.com/stretchr/testify/require"
)

func TestNewBot(t *testing.T) {
	clt := github.NewClient(nil)
	tests := []struct {
		cfg      Config
		checkErr require.ErrorAssertionFunc
		expected *Bot
	}{
		{
			cfg:      Config{Environment: &environment.PullRequestEnvironment{}, GithubClient: clt},
			checkErr: require.NoError,
		},
		{
			cfg:      Config{},
			checkErr: require.Error,
		},
	}
	for _, test := range tests {
		_, err := New(test.cfg)
		test.checkErr(t, err)
	}
}

func TestValidatePullRequestFields(t *testing.T) {
	testString := "testString"
	tests := []struct {
		pull     *github.PullRequest
		checkErr require.ErrorAssertionFunc
		desc     string
	}{
		{
			pull: &github.PullRequest{
				Base: &github.PullRequestBranch{User: &github.User{Login: &testString}, Repo: &github.Repository{Name: &testString}},
				Head: &github.PullRequestBranch{},
			},
			checkErr: require.Error,
			desc:     "missing Head.Ref",
		},
		{
			pull: &github.PullRequest{
				Base: &github.PullRequestBranch{User: &github.User{Login: &testString}, Repo: &github.Repository{Name: &testString}},
				Head: &github.PullRequestBranch{Ref: &testString},
			},
			checkErr: require.NoError,
			desc:     "valid pull request",
		},
		{
			pull: &github.PullRequest{
				Base: &github.PullRequestBranch{User: &github.User{}, Repo: &github.Repository{Name: &testString}},
				Head: &github.PullRequestBranch{Ref: &testString},
			},
			checkErr: require.Error,
			desc:     "missing Base.User.Login",
		},
		{
			pull: &github.PullRequest{
				Base: &github.PullRequestBranch{User: &github.User{}, Repo: &github.Repository{Name: &testString}},
				Head: &github.PullRequestBranch{Ref: &testString},
			},
			checkErr: require.Error,
			desc:     "missing Base.Repo.Name",
		},
		{
			pull: &github.PullRequest{

				Head: &github.PullRequestBranch{Ref: &testString},
			},
			checkErr: require.Error,
			desc:     "missing Base",
		},
		{
			pull: &github.PullRequest{
				Base: &github.PullRequestBranch{Repo: &github.Repository{Name: &testString}},
				Head: &github.PullRequestBranch{Ref: &testString},
			},
			checkErr: require.Error,
			desc:     "missing Base.User",
		},
		{
			pull: &github.PullRequest{
				Base: &github.PullRequestBranch{User: &github.User{}},
				Head: &github.PullRequestBranch{Ref: &testString},
			},
			checkErr: require.Error,
			desc:     "missing Base.Repo",
		},
		{
			pull: &github.PullRequest{
				Base: &github.PullRequestBranch{User: &github.User{}, Repo: &github.Repository{Name: &testString}},
			},
			checkErr: require.Error,
			desc:     "missing Head",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := validatePullRequestFields(test.pull)
			test.checkErr(t, err)
		})
	}

}
