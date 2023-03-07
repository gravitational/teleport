package gitlab

import "github.com/gravitational/trace"

// IDTokenSource allows a GitLab ID token to be fetched whilst executing
// within the context of a GitLab actions workflow.
type IDTokenSource struct {
	getEnv func(key string) string
}

func (its *IDTokenSource) GetIDToken() (string, error) {
	tok := its.getEnv("TBOT_GITLAB_JWT")
	if tok == "" {
		return "", trace.BadParameter(
			"TBOT_GITLAB_JWT environment variable missing",
		)
	}

	return tok, nil
}
