package types

import (
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// GithubConnector is an enterprise version of the GitHub auth connector
// that allows connecting to self-hosted GitHub Enterprise instances.
type GithubConnector struct {
	*types.GithubConnectorV3
}

// NewGithubConnectorE creates a new enterprise GitHub auth connector.
func NewGithubConnectorE(name string, spec types.GithubConnectorSpecV3) (types.GithubConnector, error) {
	ghc, err := types.NewGithubConnector(name, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ghConnector, ok := ghc.(*types.GithubConnectorV3)
	if !ok {
		return nil, trace.BadParameter("unrecognized github connector %T", ghc)
	}

	return &GithubConnector{
		GithubConnectorV3: ghConnector,
	}, nil
}

// CheckAndSetDefaults verifies the connector is valid and sets some defaults.
func (c *GithubConnector) CheckAndSetDefaults() error {
	if err := c.GithubConnectorV3.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if c.Spec.APIEndpointURL != "" && c.Spec.EndpointURL == "" {
		return trace.BadParameter("endpoint_url must be set when api_endpoint_url is set")
	}

	endpointURL, err := checkAndSetDefaultURL(c.Spec.EndpointURL, c.GithubConnectorV3.GetEndpointURL())
	if err != nil {
		return trace.BadParameter("error validating endpoint_url: %v", err)
	}
	c.Spec.EndpointURL = endpointURL.String()

	apiEndpointURL, err := checkAndSetDefaultURL(c.Spec.APIEndpointURL, c.GithubConnectorV3.GetAPIEndpointURL())
	if err != nil {
		return trace.BadParameter("error validating api_endpoint_url: %v", err)
	}
	// if the endpoint URL was set, and the API endpoint URL wasn't set,
	// prepend 'api.' to the endpoint URL to get the API endpoint URL
	if c.Spec.EndpointURL != "" && c.Spec.APIEndpointURL == "" {
		apiEndpointURL.Host = "api." + endpointURL.Host
	}
	c.Spec.APIEndpointURL = apiEndpointURL.String()

	return nil
}

func checkAndSetDefaultURL(u string, defaultURL string) (*url.URL, error) {
	if u == "" {
		return url.Parse(defaultURL)
	}

	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, trace.BadParameter("error parsing %q: %v", u, err)
	}
	if parsedURL.Scheme == "" {
		return nil, trace.BadParameter("%q is missing a scheme", u)
	}
	if parsedURL.Host == "" {
		return nil, trace.BadParameter("%q is missing a host", u)
	}

	return parsedURL, nil
}

// GetEndpointURL returns the endpoint URL.
func (c *GithubConnector) GetEndpointURL() string {
	return c.Spec.EndpointURL
}

// GetAPIEndpointURL returns the API endpoint URL.
func (c *GithubConnector) GetAPIEndpointURL() string {
	return c.Spec.APIEndpointURL
}

// UnmarshalGithubConnector unmarshals the enterprise GithubConnector resource from JSON.
func UnmarshalGithubConnector(bytes []byte, opts ...services.MarshalOption) (types.GithubConnector, error) {
	connector, err := services.UnmarshalOSSGithubConnector(bytes, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	v3, ok := connector.(*types.GithubConnectorV3)
	if !ok {
		return nil, trace.BadParameter("unrecognized github connector version %T", connector)
	}

	ec := GithubConnector{GithubConnectorV3: v3}
	if err := ec.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return connector, nil
}

// MarshalGithubConnector marshals the enterprise GithubConnector resource to JSON.
func MarshalGithubConnector(connector types.GithubConnector, opts ...services.MarshalOption) ([]byte, error) {
	if githubConnector, ok := connector.(*GithubConnector); ok {
		// check connector settings as enterprise connector so using
		// endpoint_url isn't an error
		if err := githubConnector.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		connector = githubConnector.GithubConnectorV3
	}

	out, err := services.MarshalOSSGithubConnector(connector, opts...)
	return out, trace.Wrap(err)
}
