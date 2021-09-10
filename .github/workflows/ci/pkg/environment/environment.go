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
	var pr PullRequestMetadata

	switch e.action {
	case ci.SYNCHRONIZE:
		var push PushEvent
		err := json.Unmarshal(body, &push)
		if err != nil {
			return trace.Wrap(err)
		}
		pr, err = push.toPullRequestMetadata()
		if err != nil {
			return err
		}
	case ci.ASSIGNED, ci.OPENED, ci.REOPENED, ci.READY:
		var pull PullRequestEvent
		err := json.Unmarshal(body, &pull)
		if err != nil {
			return trace.Wrap(err)
		}
		pr, err = pull.toPullRequestMetadata()
		if err != nil {
			return err
		}
	default:
		var rev ReviewEvent
		err := json.Unmarshal(body, &rev)
		if nil != err {
			return trace.Wrap(err)
		}
		pr, err = rev.toPullRequestMetadata()
		if err != nil {
			return err
		}
	}
	e.PullRequest = &pr
	return nil
}

func (r ReviewEvent) toPullRequestMetadata() (PullRequestMetadata, error) {
	pr, err := validateData(r.PullRequest.Number,
		r.PullRequest.Author.Login,
		r.Repository.Owner.Name,
		r.Repository.Name,
		r.PullRequest.Head.SHA,
		r.PullRequest.Base.SHA,
		r.PullRequest.Head.BranchName,
	)
	if err != nil {
		return PullRequestMetadata{}, err
	}
	if r.Review.User.Login == "" {
		return PullRequestMetadata{}, trace.BadParameter("missing reviewer username.")
	}
	pr.Reviewer = r.Review.User.Login
	return pr, nil
}

func (p PullRequestEvent) toPullRequestMetadata() (PullRequestMetadata, error) {
	return validateData(p.Number,
		p.PullRequest.User.Login,
		p.Repository.Owner.Name,
		p.Repository.Name,
		p.PullRequest.Head.SHA,
		p.PullRequest.Base.SHA,
		p.PullRequest.Head.BranchName,
	)
}

func (s PushEvent) toPullRequestMetadata() (PullRequestMetadata, error) {
	return validateData(s.Number,
		s.PullRequest.User.Login,
		s.Repository.Owner.Name,
		s.Repository.Name,
		s.CommitSHA,
		s.BeforeSHA,
		s.PullRequest.Head.BranchName,
	)
}

func validateData(num int, login, owner, repoName, headSHA, baseSHA, branchName string) (PullRequestMetadata, error) {
	switch {
	case num == 0:
		return PullRequestMetadata{}, trace.BadParameter("missing pull request number.")
	case login == "":
		return PullRequestMetadata{}, trace.BadParameter("missing user login.")
	case owner == "":
		return PullRequestMetadata{}, trace.BadParameter("missing repository owner.")
	case repoName == "":
		return PullRequestMetadata{}, trace.BadParameter("missing repository name.")
	case headSHA == "":
		return PullRequestMetadata{}, trace.BadParameter("missing head commit sha.")
	case baseSHA == "":
		return PullRequestMetadata{}, trace.BadParameter("missing base commit sha.")
	case branchName == "":
		return PullRequestMetadata{}, trace.BadParameter("missing branch name.")
	}
	return PullRequestMetadata{Number: num,
		Author:     login,
		RepoOwner:  owner,
		RepoName:   repoName,
		HeadSHA:    headSHA,
		BaseSHA:    baseSHA,
		BranchName: branchName}, nil
}
