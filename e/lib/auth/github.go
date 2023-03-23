package auth

import (
	"encoding/json"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// GithubConnectorE is an enterprise version of the GitHub auth connector
// that allows connecting to self-hosted GitHub Enterprise instances.
type GithubConnectorE struct {
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

	return &GithubConnectorE{
		GithubConnectorV3: ghConnector,
	}, nil
}

// CheckAndSetDefaults verifies the connector is valid and sets some defaults.
func (c *GithubConnectorE) CheckAndSetDefaults() error {
	err := c.GithubConnectorV3.CheckAndSetDefaults()
	if err != nil {
		return err
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
func (c *GithubConnectorE) GetEndpointURL() string {
	return c.Spec.EndpointURL
}

// GetEndpointAPIURL returns the API endpoint URL.
func (c *GithubConnectorE) GetAPIEndpointURL() string {
	return c.Spec.APIEndpointURL
}

// UnmarshalGithubConnectorE unmarshals the GithubConnectorE resource from JSON.
func UnmarshalGithubConnectorE(bytes []byte) (types.GithubConnector, error) {
	var h types.ResourceHeader
	if err := json.Unmarshal(bytes, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var c *types.GithubConnectorV3
		if err := utils.FastUnmarshal(bytes, &c); err != nil {
			return nil, trace.Wrap(err)
		}
		ec := GithubConnectorE{
			GithubConnectorV3: c,
		}
		if err := ec.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		return c, nil
	}
	return nil, trace.BadParameter(
		"Github connector resource version %q is not supported", h.Version)
}

// MarshalGithubConnectorE marshals the GithubConnectorE resource to JSON.
func MarshalGithubConnectorE(connector types.GithubConnector, opts ...services.MarshalOption) ([]byte, error) {
	githubConnector, ok := connector.(*GithubConnectorE)
	if !ok {
		return nil, trace.BadParameter("unrecognized github connector version %T", connector)
	}

	// check connector settings as enterprise connector so using
	// endpoint_url isn't an error
	if err := githubConnector.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		// avoid modifying the original object
		// to prevent unexpected data races
		copy := *githubConnector.GithubConnectorV3
		copy.SetResourceID(0)
		githubConnector.GithubConnectorV3 = &copy
	}
	return utils.FastMarshal(githubConnector.GithubConnectorV3)
}
