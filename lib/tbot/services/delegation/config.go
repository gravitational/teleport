package delegation

import (
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
	"github.com/gravitational/trace"
)

const ServiceType = "delegation-api"

type Config struct {
	Listen             string                 `yaml:"listen"`
	Path               string                 `yaml:"path"`
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Listen == "" {
		return trace.BadParameter("listen: should not be empty")
	}
	return nil
}

func (c *Config) GetCredentialLifetime() bot.CredentialLifetime {
	return c.CredentialLifetime
}

func (c *Config) GetName() string {
	return ServiceType
}

func (c *Config) Type() string {
	return ServiceType
}

func (o *Config) MarshalYAML() (any, error) {
	type raw Config
	return encoding.WithTypeHeader((*raw)(o), ServiceType)
}
