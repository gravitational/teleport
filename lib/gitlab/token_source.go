package gitlab

import "github.com/gravitational/trace"

type envGetter func(key string) string

// IDTokenSource allows a GitLab ID token to be fetched whilst executing
// within the context of a GitLab actions workflow.
type IDTokenSource struct {
	getEnv envGetter
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

func NewIDTokenSource(getEnv envGetter) *IDTokenSource {
	return &IDTokenSource{
		getEnv,
	}
}
