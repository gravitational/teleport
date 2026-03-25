package vnet

import "github.com/gravitational/teleport/lib/tbot/bot"

const ServiceType = "unstable/vnet"

type Config struct {
	Name                string `yaml:"name,omitempty"`
	DelegationSessionID string `yaml:"delegation_session_id,omitempty"`
	BeamID              string `yaml:"beam_id,omitempty"`
	// Restricted, if true, enables restricted egress mode: VNet will only
	// resolve and proxy domains it explicitly handles (beam allowlist, apps,
	// databases). All other DNS queries return NXDOMAIN and connections to
	// unrecognized hosts are rejected at TCP level.
	Restricted bool `yaml:"restricted,omitempty"`
}

func (cfg *Config) CheckAndSetDefaults() error { return nil }
func (cfg Config) GetName() string             { return cfg.Name }
func (cfg Config) SetName(name string)         { cfg.Name = name }
func (cfg Config) Type() string                { return ServiceType }

func (cfg Config) GetCredentialLifetime() bot.CredentialLifetime {
	return bot.DefaultCredentialLifetime
}
