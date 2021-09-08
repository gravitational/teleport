package environment

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/gravitational/teleport/.github/workflows/ci"
	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
)

// Config is used to configure Environment
type Config struct {
	Client        *github.Client
	Reviewers     string
	EventPath     string
	Token         string
	unmarshalRevs unmarshalReviewersFn
}

// Environment contains information about the environment
type Environment struct {
	Client           *github.Client
	PullRequest      *PullRequestMetadata
	token            string
	reviewers        map[string][]string
	defaultReviewers []string
	action           string
}

type unmarshalReviewersFn func(str string, client *github.Client) (map[string][]string, error)

// CheckAndSetDefaults verifies configuration and sets defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client.")
	}
	if c.Reviewers == "" {
		return trace.BadParameter("missing parameter Reviewers.")
	}
	if c.EventPath == "" {
		return trace.BadParameter("missing parameter EventPath.")
	}
	if c.Token == "" {
		return trace.BadParameter("missing parameter token.")
	}
	if c.unmarshalRevs == nil {
		c.unmarshalRevs = unmarshalReviewers
	}
	return nil
}

// New creates a new instance of Environment.
func New(c Config) (*Environment, error) {
	var env Environment
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	env.Client = c.Client
	revs, err := c.unmarshalRevs(c.Reviewers, c.Client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	env.reviewers = revs
	env.defaultReviewers = revs[""]
	err = env.SetPullRequest(c.EventPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &env, nil
}

// unmarshalReviewers converts the passed in string representing json object into a map
func unmarshalReviewers(str string, client *github.Client) (map[string][]string, error) {
	var hasDefaultReviewers bool
	if str == "" {
		return nil, trace.BadParameter("reviewers not found.")
	}
	m := make(map[string][]string)

	err := json.Unmarshal([]byte(str), &m)
	if err != nil {
		return nil, err
	}
	for k, v := range m {
		for _, reviewer := range v {
			err := userExists(reviewer, client)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		if k == "" {
			hasDefaultReviewers = true
			continue
		}
		err := userExists(k, client)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if !hasDefaultReviewers {
		return nil, trace.BadParameter("default reviewers are not set. set default reviewers with an empty string as a key.")
	}
	return m, nil

}

// userExists checks if a user exists
func userExists(user string, client *github.Client) error {
	_, resp, err := client.Search.Users(context.TODO(), user, &github.SearchOptions{})
	if err != nil || resp.Status != "200 OK" {
		return trace.Wrap(err)
	}
	return nil
}

// GetReviewersForAuthor gets the required reviewers for the current user.
func (e *Environment) GetReviewersForAuthor(user string) []string {
	value, ok := e.reviewers[user]
	// author is external or does not have set reviewers
	if !ok {
		return e.defaultReviewers
	}
	return value
}

// IsInternal determines if an author is an internal contributor.
func (e *Environment) IsInternal(author string) bool {
	_, ok := e.reviewers[author]
	return ok
}

// GetToken gets token
func (e *Environment) GetToken() string {
	return e.token
}

// SetPullRequest sets the pull request in which the environment
// is processing.
func (e *Environment) SetPullRequest(path string) error {
	var actionType action
	file, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	body, err := ioutil.ReadAll(file)
	if err != nil {
		return trace.Wrap(err)
	}
	err = json.Unmarshal(body, &actionType)
	if err != nil {
		return trace.Wrap(err)
	}
	e.action = actionType.Action
	return e.setPullRequest(body)
}

func (e *Environment) setPullRequest(body []byte) error {
	switch e.action {
	case ci.SYNCHRONIZE:
		// Push events to pull requests
		var push PushEvent
		err := json.Unmarshal(body, &push)
		if err != nil {
			return trace.Wrap(err)
		}
		if push.Number != 0 && push.Repository.Name != "" && push.Repository.Owner.Name != "" && push.PullRequest.User.Login != "" && push.CommitSHA != "" && push.PullRequest.Head.BranchName != "" && push.BeforeSHA != "" {
			e.PullRequest = &PullRequestMetadata{
				Author:     push.PullRequest.User.Login,
				RepoName:   push.Repository.Name,
				RepoOwner:  push.Repository.Owner.Name,
				Number:     push.Number,
				HeadSHA:    push.CommitSHA,
				BaseSHA:    push.BeforeSHA,
				BranchName: push.PullRequest.Head.BranchName,
			}
			return nil
		}
		return trace.BadParameter("insufficient data obtained")
	case ci.ASSIGNED, ci.OPENED, ci.REOPENED, ci.READY:
		// PullRequestEvents
		var pull PullRequestEvent
		err := json.Unmarshal(body, &pull)
		if err != nil {
			return trace.Wrap(err)
		}
		if pull.Number != 0 && pull.Repository.Name != "" && pull.Repository.Owner.Name != "" && pull.PullRequest.User.Login != "" && pull.PullRequest.Head.SHA != "" && pull.PullRequest.Base.SHA != "" && pull.PullRequest.Head.BranchName != "" {
			e.PullRequest = &PullRequestMetadata{
				Author:     pull.PullRequest.User.Login,
				RepoName:   pull.Repository.Name,
				RepoOwner:  pull.Repository.Owner.Name,
				Number:     pull.Number,
				HeadSHA:    pull.PullRequest.Head.SHA,
				BaseSHA:    pull.PullRequest.Base.SHA,
				BranchName: pull.PullRequest.Head.BranchName,
			}
			return nil
		}
		return trace.BadParameter("insufficient data obtained")

	default:
		// Review Events
		var rev ReviewMetadata
		err := json.Unmarshal(body, &rev)
		if nil != err {
			return trace.Wrap(err)
		}
		if rev.PullRequest.Number != 0 && rev.Review.User.Login != "" && rev.Repository.Name != "" && rev.Repository.Owner.Name != "" && rev.PullRequest.Head.SHA != "" && rev.PullRequest.Base.SHA != "" && rev.PullRequest.Head.BranchName != "" {
			e.PullRequest = &PullRequestMetadata{
				Author:     rev.PullRequest.Author.Login,
				Reviewer:   rev.Review.User.Login,
				RepoName:   rev.Repository.Name,
				RepoOwner:  rev.Repository.Owner.Name,
				Number:     rev.PullRequest.Number,
				HeadSHA:    rev.PullRequest.Head.SHA,
				BaseSHA:    rev.PullRequest.Base.SHA,
				BranchName: rev.PullRequest.Head.BranchName,
			}
			return nil
		}
		return trace.BadParameter("insufficient data obtained")

	}
}
