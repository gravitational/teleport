package vnet

import "github.com/gravitational/teleport/lib/tbot/bot"

const ServiceType = "unstable/vnet"

type Config struct {
	Name             string `yaml:"name,omitempty"`
	DelegationTicket string `yaml:"delegation_ticket,omitempty"`
}

func (cfg *Config) CheckAndSetDefaults() error { return nil }
func (cfg Config) GetName() string             { return cfg.Name }
func (cfg Config) SetName(name string)         { cfg.Name = name }
func (cfg Config) Type() string                { return ServiceType }

func (cfg Config) GetCredentialLifetime() bot.CredentialLifetime {
	return bot.DefaultCredentialLifetime
}
