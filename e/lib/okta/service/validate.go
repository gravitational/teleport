package oktaservice

import (
	"net/url"

	"github.com/gravitational/trace"
)

// TODO(kopiczko) will be merged with https://github.com/gravitational/teleport.e/pull/5756/

func validateAndSanitizeUrl(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", trace.BadParameter("invalid URL: %v", err)
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if u.Scheme != "https" {
		return "", trace.BadParameter("required https scheme, but got %q", u.Scheme)
	}
	return u.String(), nil
}
