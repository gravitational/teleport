package environment

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gravitational/teleport/tool/ci"
	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
)

// Config is used to configure Environment
type Config struct {
	// Context is the context for Environment
	Context context.Context
	// Client is the authenticated Github client.
	Client *github.Client
	// Reviewers is a json object encoded as a string with
	// authors mapped to their respective required reviewers.
	Reviewers string
	// EventPath is the path of the file with the complete
	// webhook event payload on the runner.
	EventPath string
	// Token is the token to authenticate on behalf of the GitHub.
	Token string
	// unmarshalReviewers is the function to unmarshal
	// the `Reviewers` string into map[string][]string.
	unmarshalReviewers unmarshalReviewersFn
}

// Environment contains information about the environment
type Environment struct {
	// Client is the authenticated Github client
	Client *github.Client
	// PullRequest is the pull request in the
	// current context.
	PullRequest *PullRequestMetadata
	// token is the Github token.
	token string
	// reviewers is a map of reviewers where the key
	// is the user name of the author and the value is a list
	// of required reviewers.
	reviewers map[string][]string
	// defaultReviewers is a list of reviewers used for authors whose
	// usernames are not a key in `reviewers`
	defaultReviewers []string
	// action is the action that triggered the workflow.
	action string
}

// PullRequestMetadata is the current pull request metadata
type PullRequestMetadata struct {
	// Author is the pull request author.
	Author string
	// RepoName is the repository name that the
	// current pull request is trying to merge into.
	RepoName string
	// RepoOwner is the owner of the repository the
	// author is trying to merge into.
	RepoOwner string
	// Number is the pull request number.
	Number int
	// HeadSHA is the commit sha of the author's branch.
	HeadSHA string
	// BaseSHA is the commit sha of the base branch.
	BaseSHA string
	// Reviewer is the reviewer's Github username.
	// Only used for pull request review events.
	Reviewer string
	// BranchName is the name of the branch the author
	// is trying to merge in.
	BranchName string
}

type unmarshalReviewersFn func(ctx context.Context, str string, client *github.Client) (map[string][]string, error)

// CheckAndSetDefaults verifies configuration and sets defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if c.Reviewers == "" {
		return trace.BadParameter("missing parameter Reviewers")
	}
	if c.EventPath == "" {
		return trace.BadParameter("missing parameter EventPath")
	}
	if c.Token == "" {
		return trace.BadParameter("missing parameter Token")
	}
	if c.unmarshalReviewers == nil {
		c.unmarshalReviewers = unmarshalReviewers
	}
	return nil
}

// New creates a new instance of Environment.
func New(c Config) (*Environment, error) {
	err := c.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	revs, err := c.unmarshalReviewers(c.Context, c.Reviewers, c.Client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pr, err := GetPullRequest(c.EventPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Environment{
		Client:           c.Client,
		reviewers:        revs,
		defaultReviewers: revs[""],
		PullRequest:      pr,
	}, nil
}

// unmarshalReviewers converts the passed in string representing json object into a map
func unmarshalReviewers(ctx context.Context, str string, client *github.Client) (map[string][]string, error) {
	var hasDefaultReviewers bool
	if str == "" {
		return nil, trace.NotFound("reviewers not found")
	}
	m := make(map[string][]string)

	err := json.Unmarshal([]byte(str), &m)
	if err != nil {
		return nil, err
	}
	for author, requiredReviewers := range m {
		for _, reviewer := range requiredReviewers {
			_, err := userExists(ctx, reviewer, client)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		if author == "" {
			hasDefaultReviewers = true
			continue
		}
		_, err := userExists(ctx, author, client)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if !hasDefaultReviewers {
		return nil, trace.BadParameter("default reviewers are not set. set default reviewers with an empty string as a key")
	}
	return m, nil

}

// userExists checks if a user exists
func userExists(ctx context.Context, userLogin string, client *github.Client) (*github.User, error) {
	user, resp, err := client.Users.Get(ctx, userLogin)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, trace.Wrap(err)
	}
	return user, nil
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

// GetPullRequest gets the pull request metadata in the current context.
func GetPullRequest(path string) (*PullRequestMetadata, error) {
	var actionType action
	file, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()
	body, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = json.Unmarshal(body, &actionType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return getPullRequest(body, actionType.Action)
}

func getPullRequest(body []byte, action string) (*PullRequestMetadata, error) {
	var pr PullRequestMetadata

	switch action {
	case ci.Synchronize:
		var push PushEvent
		err := json.Unmarshal(body, &push)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pr, err = push.toPullRequestMetadata()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case ci.Assigned, ci.Opened, ci.Reopened, ci.Ready:
		var pull PullRequestEvent
		err := json.Unmarshal(body, &pull)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pr, err = pull.toPullRequestMetadata()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		var rev ReviewEvent
		err := json.Unmarshal(body, &rev)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pr, err = rev.toPullRequestMetadata()
		if err != nil {
			return nil, err
		}
	}
	return &pr, nil
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
		return PullRequestMetadata{}, trace.BadParameter("missing pull request number")
	case login == "":
		return PullRequestMetadata{}, trace.BadParameter("missing user login")
	case owner == "":
		return PullRequestMetadata{}, trace.BadParameter("missing repository owner")
	case repoName == "":
		return PullRequestMetadata{}, trace.BadParameter("missing repository name")
	case headSHA == "":
		return PullRequestMetadata{}, trace.BadParameter("missing head commit sha")
	case baseSHA == "":
		return PullRequestMetadata{}, trace.BadParameter("missing base commit sha")
	case branchName == "":
		return PullRequestMetadata{}, trace.BadParameter("missing branch name")
	}
	return PullRequestMetadata{Number: num,
		Author:     login,
		RepoOwner:  owner,
		RepoName:   repoName,
		HeadSHA:    headSHA,
		BaseSHA:    baseSHA,
		BranchName: branchName}, nil
}
