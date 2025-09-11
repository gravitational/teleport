package application

import (
	"net"
	"net/url"

	"gopkg.in/yaml.v3"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const ProxyServiceType = "httpproxy-tunnel"

type ProxyServiceConfig struct {
	Name               string                 `yaml:"name,omitempty"`
	Listen             string                 `yaml:"listen"`
	Roles              []string               `yaml:"roles,omitempty"`
	CredentialLifetime bot.CredentialLifetime `yaml:"credential_lifetime,omitempty"`
	Listener           net.Listener           `yaml:"-"`
}

func (c *ProxyServiceConfig) GetName() string {
	return c.Name
}

func (c *ProxyServiceConfig) Type() string {
	return ProxyServiceType
}

func (c *ProxyServiceConfig) MarshalYAML() (any, error) {
	type raw ProxyServiceConfig
	return encoding.WithTypeHeader((*raw)(c), ProxyServiceType)
}

func (c *ProxyServiceConfig) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw ProxyServiceConfig
	if err := node.Decode((*raw)(c)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *ProxyServiceConfig) CheckAndSetDefaults() error {
	switch {
	case c.Listen == "" && c.Listener == nil:
		return trace.BadParameter("listen: should not be empty")
	}

	if _, err := url.Parse(c.Listen); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *ProxyServiceConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return c.CredentialLifetime
}
