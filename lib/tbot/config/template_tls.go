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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const defaultTLSPrefix = "tls"

// templateTLS is a config template that wraps identityfile's TLS writer.
// It's not generally needed but can be used to write out TLS certificates with
// alternative prefix and file extensions if needed for application
// compatibility reasons.
type templateTLS struct {
	// caCertType is the type of CA cert to be written
	caCertType types.CertAuthType
}

func (t *templateTLS) name() string {
	return TemplateTLSName
}

func (t *templateTLS) describe() []FileDescription {
	return []FileDescription{
		{
			Name: defaultTLSPrefix + ".key",
		},
		{
			Name: defaultTLSPrefix + ".crt",
		},
		{
			Name: defaultTLSPrefix + ".cas",
		},
	}
}

func (t *templateTLS) render(
	ctx context.Context,
	bot provider,
	identity *identity.Identity,
	destination bot.Destination,
) error {
	cas, err := bot.GetCertAuthorities(ctx, t.caCertType)
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := newClientKey(identity, cas)
	if err != nil {
		return trace.Wrap(err)
	}

	cfg := identityfile.WriteConfig{
		OutputPath: defaultTLSPrefix,
		Writer: &BotConfigWriter{
			dest: destination,
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
