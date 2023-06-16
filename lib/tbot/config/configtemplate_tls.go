/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"context"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const defaultTLSPrefix = "tls"

// CertAuthType is a types.CertAuthType wrapper with unmarshalling support.
type CertAuthType types.CertAuthType

const defaultCAType = types.HostCA

func (c *CertAuthType) UnmarshalYAML(node *yaml.Node) error {
	var certType string
	err := node.Decode(&certType)
	if err != nil {
		return trace.Wrap(err)
	}

	switch certType {
	case "":
		*c = CertAuthType(defaultCAType)
	case string(types.HostCA), string(types.UserCA), string(types.DatabaseCA):
		*c = CertAuthType(certType)
	default:
		return trace.BadParameter("invalid CA certificate type: %q", certType)
	}

	return nil
}

func (c *CertAuthType) CheckAndSetDefaults() error {
	switch types.CertAuthType(*c) {
	case "":
		*c = CertAuthType(defaultCAType)
	case types.HostCA, types.UserCA, types.DatabaseCA:
		// valid, nothing to do
	default:
		return trace.BadParameter("unsupported CA certificate type: %q", string(*c))
	}

	return nil
}

// TemplateTLS is a config template that wraps identityfile's TLS writer.
// It's not generally needed but can be used to write out TLS certificates with
// alternative prefix and file extensions if needed for application
// compatibility reasons.
type TemplateTLS struct {
	// Prefix is the filename prefix for the output files.
	Prefix string `yaml:"prefix,omitempty"`

	// CACertType is the type of CA cert to be written
	CACertType CertAuthType `yaml:"ca_cert_type,omitempty"`
}

func (t *TemplateTLS) CheckAndSetDefaults() error {
	if t.Prefix == "" {
		t.Prefix = defaultTLSPrefix
	}

	if err := t.CACertType.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (t *TemplateTLS) Name() string {
	return TemplateTLSName
}

func (t *TemplateTLS) Describe(destination bot.Destination) []FileDescription {
	return []FileDescription{
		{
			Name: t.Prefix + ".key",
		},
		{
			Name: t.Prefix + ".crt",
		},
		{
			Name: t.Prefix + ".cas",
		},
	}
}

func (t *TemplateTLS) Render(
	ctx context.Context,
	bot provider,
	identity *identity.Identity,
	destination *DestinationConfig,
) error {
	dest, err := destination.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	cas, err := bot.GetCertAuthorities(ctx, types.CertAuthType(t.CACertType))
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := newClientKey(identity, cas)
	if err != nil {
		return trace.Wrap(err)
	}

	cfg := identityfile.WriteConfig{
		OutputPath: t.Prefix,
		Writer: &BotConfigWriter{
			dest: dest,
		},
		Key:    key,
		Format: identityfile.FormatTLS,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Debugf("Wrote TLS identity files: %+v", files)

	return nil
}
