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
