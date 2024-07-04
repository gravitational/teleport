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
	ctx, span := tracer.Start(
		ctx,
		"templateTLS/render",
	)
	defer span.End()

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
			ctx:  ctx,
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

	log.DebugContext(ctx, "Wrote TLS identity files", "files", files)

	return nil
}
