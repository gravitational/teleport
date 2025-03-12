/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package config

import (
	"slices"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

type destinationMixinV1 struct {
	Directory *DestinationDirectory `yaml:"directory"`
	Memory    *DestinationMemory    `yaml:"memory"`
}

func (c *destinationMixinV1) migrate() (bot.Destination, error) {
	switch {
	case c.Memory != nil && c.Directory != nil:
		return nil, trace.BadParameter("both 'memory' and 'directory' cannot be specified")
	case c.Memory != nil:
		return c.Memory, nil
	case c.Directory != nil:
		return c.Directory, nil
	default:
		return nil, trace.BadParameter("at least one of `memory' and 'directory' must be specified")
	}
}

type storageConfigV1 struct {
	Mixin destinationMixinV1 `yaml:",inline"`
}

func (c *storageConfigV1) migrate() (*StorageConfig, error) {
	dest, err := c.Mixin.migrate()
	if err != nil {
		return nil, trace.Wrap(err, "migrating destination mixin")
	}

	return &StorageConfig{
		Destination: dest,
	}, nil
}

type configV1Database struct {
	Service  string `yaml:"service"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
}

type configV1DestinationConfigHostCert struct {
	Principals []string `yaml:"principals"`
}

type configV1DestinationConfig struct {
	SSHClient   map[string]any                     `yaml:"ssh_client"`
	Identity    map[string]any                     `yaml:"identity"`
	TLS         map[string]any                     `yaml:"tls"`
	TLSCAs      map[string]any                     `yaml:"tls_cas"`
	Mongo       map[string]any                     `yaml:"mongo"`
	Cockroach   map[string]any                     `yaml:"cockroach"`
	Kubernetes  map[string]any                     `yaml:"kubernetes"`
	SSHHostCert *configV1DestinationConfigHostCert `yaml:"ssh_host_cert"`
}

func (c *configV1DestinationConfig) UnmarshalYAML(node *yaml.Node) error {
	var simpleTemplate string
	if err := node.Decode(&simpleTemplate); err == nil {
		switch simpleTemplate {
		case TemplateSSHClientName:
			c.SSHClient = map[string]any{}
		case TemplateIdentityName:
			c.Identity = map[string]any{}
		case TemplateTLSName:
			c.TLS = map[string]any{}
		case TemplateTLSCAsName:
			c.TLSCAs = map[string]any{}
		case TemplateMongoName:
			c.Mongo = map[string]any{}
		case TemplateCockroachName:
			c.Cockroach = map[string]any{}
		case TemplateKubernetesName:
			c.Kubernetes = map[string]any{}
		case TemplateSSHHostCertName:
			c.SSHHostCert = &configV1DestinationConfigHostCert{}
		default:
			return trace.BadParameter("unrecognized config template %q", simpleTemplate)
		}
		return nil
	}

	// Fall back to the full struct; alias it to get standard unmarshal
	// behavior and avoid recursion
	type rawTemplate configV1DestinationConfig
	return trace.Wrap(node.Decode((*rawTemplate)(c)))
}

type configV1Destination struct {
	Mixin destinationMixinV1 `yaml:",inline"`

	Roles   []string                    `yaml:"roles"`
	Configs []configV1DestinationConfig `yaml:"configs"`

	Database          *configV1Database `yaml:"database"`
	KubernetesCluster string            `yaml:"kubernetes_cluster"`
	App               string            `yaml:"app"`
	Cluster           string            `yaml:"cluster"`
}

func validateTemplates(configs []configV1DestinationConfig, allowedTypes []string, requiredTypes []string) error {
	var allConfiguredTypes []string

	configUnsupportedErr := func(typeName string) error {
		return trace.BadParameter("configuration options are not supported by migration for %s config template", typeName)
	}

	for _, templateConfig := range configs {
		var configuredTypes []string
		if templateConfig.SSHClient != nil {
			if len(templateConfig.SSHClient) > 0 {
				return configUnsupportedErr(TemplateSSHClientName)
			}
			configuredTypes = append(configuredTypes, TemplateSSHClientName)
		}
		if templateConfig.Identity != nil {
			if len(templateConfig.Identity) > 0 {
				return configUnsupportedErr(TemplateIdentityName)
			}
			configuredTypes = append(configuredTypes, TemplateIdentityName)
		}
		if templateConfig.TLS != nil {
			if len(templateConfig.TLS) > 0 {
				return configUnsupportedErr(TemplateTLSName)
			}
			configuredTypes = append(configuredTypes, TemplateTLSName)
		}
		if templateConfig.TLSCAs != nil {
			if len(templateConfig.TLSCAs) > 0 {
				return configUnsupportedErr(TemplateTLSCAsName)
			}
			configuredTypes = append(configuredTypes, TemplateTLSCAsName)
		}
		if templateConfig.Mongo != nil {
			if len(templateConfig.Mongo) > 0 {
				return configUnsupportedErr(TemplateMongoName)
			}
			configuredTypes = append(configuredTypes, TemplateMongoName)
		}
		if templateConfig.Cockroach != nil {
			if len(templateConfig.Cockroach) > 0 {
				return configUnsupportedErr(TemplateCockroachName)
			}
			configuredTypes = append(configuredTypes, TemplateCockroachName)
		}
		if templateConfig.Kubernetes != nil {
			if len(templateConfig.Kubernetes) > 0 {
				return configUnsupportedErr(TemplateKubernetesName)
			}
			configuredTypes = append(configuredTypes, TemplateKubernetesName)
		}
		if templateConfig.SSHHostCert != nil {
			if len(templateConfig.SSHHostCert.Principals) == 0 {
				return trace.BadParameter("no principals specified for %s config template", TemplateSSHHostCertName)
			}
			configuredTypes = append(configuredTypes, TemplateSSHHostCertName)
		}

		if len(configuredTypes) == 0 {
			return trace.BadParameter("config template must not be empty")
		}
		if len(configuredTypes) > 1 {
			return trace.BadParameter("config template must have exactly one configuration")
		}

		allConfiguredTypes = append(allConfiguredTypes, configuredTypes...)
	}

	// Ensure all types are allowed by the new output type
	for _, typeName := range allConfiguredTypes {
		if !slices.Contains(allowedTypes, typeName) {
			return trace.BadParameter("config template %q unsupported by new output type", typeName)
		}
	}

	// Ensure the required types are specified for the new output type
	for _, typeName := range requiredTypes {
		if !slices.Contains(allConfiguredTypes, typeName) {
			return trace.BadParameter("old config templates missing required template %s", typeName)
		}
	}

	// Check for any weird duplicates we can't handle correctly
	typeCounts := map[string]int{}
	for _, typeName := range allConfiguredTypes {
		typeCounts[typeName]++
	}
	for typeName, count := range typeCounts {
		if count > 1 {
			return trace.BadParameter("multiple config template entries found for %q", typeName)
		}
	}

	return nil
}

func (c *configV1Destination) migrate() (ServiceConfig, error) {
	dest, err := c.Mixin.migrate()
	if err != nil {
		return nil, trace.Wrap(err, "migrating destination")
	}

	appConfigured := c.App != ""
	databaseConfigured := c.Database != nil
	kubernetesConfigured := c.KubernetesCluster != ""
	hostCertConfigured := false
	for _, templateConfig := range c.Configs {
		if templateConfig.SSHHostCert != nil {
			hostCertConfigured = true
		}
	}
	outputTypesCount := 0
	for _, val := range []bool{appConfigured, databaseConfigured, kubernetesConfigured, hostCertConfigured} {
		if val {
			outputTypesCount++
		}
	}
	if outputTypesCount > 1 {
		return nil, trace.BadParameter("multiple potential output types detected, cannot determine correct type")
	}

	switch {
	case appConfigured:
		if err := validateTemplates(
			c.Configs,
			[]string{TemplateTLSCAsName, TemplateTLSName, TemplateIdentityName},
			[]string{},
		); err != nil {
			return nil, trace.Wrap(err, "validating template configs")
		}
		specificTLSExtensions := false
		for _, templateConfig := range c.Configs {
			if templateConfig.TLS != nil {
				specificTLSExtensions = true
				break
			}
		}
		return &ApplicationOutput{
			Destination:           dest,
			Roles:                 c.Roles,
			AppName:               c.App,
			SpecificTLSExtensions: specificTLSExtensions,
		}, nil
	case databaseConfigured:
		if err := validateTemplates(
			c.Configs,
			[]string{TemplateTLSCAsName, TemplateIdentityName, TemplateMongoName, TemplateCockroachName, TemplateTLSName},
			[]string{},
		); err != nil {
			return nil, trace.Wrap(err, "validating template configs")
		}
		format := UnspecifiedDatabaseFormat
		for _, templateConfig := range c.Configs {
			if templateConfig.Mongo != nil {
				if format != UnspecifiedDatabaseFormat {
					return nil, trace.BadParameter("multiple candidate formats for database output")
				}
				format = MongoDatabaseFormat
			}
			if templateConfig.Cockroach != nil {
				if format != UnspecifiedDatabaseFormat {
					return nil, trace.BadParameter("multiple candidate formats for database output")
				}
				format = CockroachDatabaseFormat
			}
			if templateConfig.TLS != nil {
				if format != UnspecifiedDatabaseFormat {
					return nil, trace.BadParameter("multiple candidate formats for database output")
				}
				format = TLSDatabaseFormat
			}
		}
		return &DatabaseOutput{
			Destination: dest,
			Roles:       c.Roles,
			Format:      format,
			Database:    c.Database.Database,
			Service:     c.Database.Service,
			Username:    c.Database.Username,
		}, nil
	case kubernetesConfigured:
		if err := validateTemplates(
			c.Configs,
			[]string{TemplateTLSCAsName, TemplateIdentityName, TemplateKubernetesName},
			[]string{},
		); err != nil {
			return nil, trace.Wrap(err, "validating template configs")
		}
		return &KubernetesOutput{
			Destination:       dest,
			Roles:             c.Roles,
			KubernetesCluster: c.KubernetesCluster,
		}, nil
	case hostCertConfigured:
		if err := validateTemplates(
			c.Configs,
			[]string{TemplateSSHHostCertName},
			[]string{TemplateSSHHostCertName},
		); err != nil {
			return nil, trace.Wrap(err, "validating template configs")
		}

		// Extract principals from template config
		principals := []string{}
		for _, c := range c.Configs {
			if c.SSHHostCert != nil {
				principals = c.SSHHostCert.Principals
				break
			}
		}
		return &SSHHostOutput{
			Destination: dest,
			Roles:       c.Roles,
			Principals:  principals,
		}, nil
	default:
		if err := validateTemplates(
			c.Configs,
			[]string{TemplateTLSCAsName, TemplateIdentityName, TemplateSSHClientName},
			[]string{},
		); err != nil {
			return nil, trace.Wrap(err, "validating template configs")
		}
		return &IdentityOutput{
			Destination: dest,
			Roles:       c.Roles,
			Cluster:     c.Cluster,
		}, nil
	}
}

type configV1 struct {
	Onboarding      OnboardingConfig `yaml:"onboarding"`
	Debug           bool             `yaml:"debug"`
	AuthServer      string           `yaml:"auth_server"`
	CertificateTTL  time.Duration    `yaml:"certificate_ttl"`
	RenewalInterval time.Duration    `yaml:"renewal_interval"`
	Oneshot         bool             `yaml:"oneshot"`
	FIPS            bool             `yaml:"fips"`
	DiagAddr        string           `yaml:"diag_addr"`

	Destinations  []configV1Destination `yaml:"destinations"`
	StorageConfig *storageConfigV1      `yaml:"storage"`

	// This field doesn't exist in V1, but, it exists here so we can detect
	// a scenario where for some reason we're trying to migrate a V2 config
	// that's missing the version header.
	Outputs []any `yaml:"outputs"`
}

func (c *configV1) migrate() (*BotConfig, error) {
	if len(c.Outputs) > 0 {
		return nil, trace.BadParameter("config has been detected as potentially v1, but includes the v2 outputs field")
	}

	var storage *StorageConfig
	var err error
	if c.StorageConfig != nil {
		storage, err = c.StorageConfig.migrate()
		if err != nil {
			return nil, trace.Wrap(err, "migrating storage config")
		}
	}

	var outputs []ServiceConfig
	for _, d := range c.Destinations {
		o, err := d.migrate()
		if err != nil {
			return nil, trace.Wrap(err, "migrating output")
		}
		outputs = append(outputs, o)
	}

	return &BotConfig{
		Version: V2,

		Onboarding: c.Onboarding,
		Debug:      c.Debug,
		AuthServer: c.AuthServer,
		CredentialLifetime: CredentialLifetime{
			TTL:             c.CertificateTTL,
			RenewalInterval: c.RenewalInterval,
		},
		Oneshot:  c.Oneshot,
		FIPS:     c.FIPS,
		DiagAddr: c.DiagAddr,

		Storage:  storage,
		Services: outputs,
	}, nil
}
