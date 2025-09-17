package application

import (
	"net"
	"net/url"

	"gopkg.in/yaml.v3"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const ProxyServiceType = "application-proxy"

type ProxyServiceConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	// Optional.
	Name string `yaml:"name,omitempty"`
	// Listen is the address on which application proxy should listen. Example:
	// - "tcp://127.0.0.1:3306"
	// - "tcp://0.0.0.0:3306
	Listen string `yaml:"listen"`
	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed. For the application
	// proxy, this is primarily an internal detail.
	CredentialLifetime bot.CredentialLifetime `yaml:"credential_lifetime,omitempty"`
	// Listener overrides "listen" and directly provides an opened listener to
	// use. Primarily used for testing.
	Listener net.Listener `yaml:"-"`
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
