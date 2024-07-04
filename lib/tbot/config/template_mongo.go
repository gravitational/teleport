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

// defaultMongoPrefix is the default prefix in generated MongoDB certs.
const defaultMongoPrefix = "mongo"

// templateMongo is a config template that generates TLS certs formatted for
// use with MongoDB.
type templateMongo struct{}

func (t *templateMongo) name() string {
	return TemplateMongoName
}

func (t *templateMongo) describe() []FileDescription {
	return []FileDescription{
		{
			Name: defaultMongoPrefix + ".crt",
		},
		{
			Name: defaultMongoPrefix + ".cas",
		},
	}
}

func (t *templateMongo) render(
	ctx context.Context,
	bot provider,
	identity *identity.Identity,
	destination bot.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"templateMongo/render",
	)
	defer span.End()

	dbCAs, err := bot.GetCertAuthorities(ctx, types.DatabaseCA)
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := newClientKey(identity, dbCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	cfg := identityfile.WriteConfig{
		OutputPath: defaultMongoPrefix,
		Writer: &BotConfigWriter{
			ctx:  ctx,
			dest: destination,
		},
		Key:    key,
		Format: identityfile.FormatMongo,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Wrote MongoDB identity files", "files", files)

	return nil
}
