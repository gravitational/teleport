package plugins

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/auth/oauth"
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
