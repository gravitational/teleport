package plugins

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/auth/oauth"
	"github.com/gravitational/teleport/integrations/access/slack"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// Authorizer wraps oauth.Authorizer
type Authorizer struct {
	oauth.Authorizer
	ClientID string
}

// AuthorizerSet contains exchangers for different types of plugins
type AuthorizerSet struct {
	Authorizers map[types.PluginType]*Authorizer
}

// NewAuthorizerSetFromConfig creates an AuthorizerSet,
// and automatically adds Authorizers from the service config to it
func NewAuthorizerSetFromConfig(cfg servicecfg.PluginOAuthProviders) *AuthorizerSet {
	a := NewAuthorizerSet()
	if cfg.SlackCredentials != nil {
		a.Add(types.PluginTypeSlack, &Authorizer{
			Authorizer: slack.NewAuthorizer(cfg.SlackCredentials.ClientID, cfg.SlackCredentials.ClientSecret),
			ClientID:   cfg.SlackCredentials.ClientID,
		})
	}
	return a
}

// NewAuthorizerSet creates an empty AuthorizerSet
func NewAuthorizerSet() *AuthorizerSet {
	return &AuthorizerSet{
		Authorizers: make(map[types.PluginType]*Authorizer),
	}
}

// Add registers an authorizer for the given type of plugin
func (s *AuthorizerSet) Add(typ types.PluginType, authorizer *Authorizer) {
	s.Authorizers[typ] = authorizer
}

// Get returns an authorizer depending on the plugin type
func (s *AuthorizerSet) Get(typ types.PluginType) (*Authorizer, error) {
	if a, ok := s.Authorizers[typ]; ok {
		return a, nil
	}

	return nil, trace.BadParameter("no authorizer configured for plugin type %q", typ)
}
