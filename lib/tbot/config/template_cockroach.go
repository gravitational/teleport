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

const defaultCockroachDirName = "cockroach"

// templateCockroach generates certificates for CockroachDB. These are standard
// TLS certs but have specific naming requirements. We write them to a
// subdirectory to ensure naming is clear.
type templateCockroach struct{}

func (t *templateCockroach) name() string {
	return TemplateCockroachName
}

func (t *templateCockroach) describe() []FileDescription {
	return []FileDescription{
		{
			Name:  defaultCockroachDirName,
			IsDir: true,
		},
	}
}

func (t *templateCockroach) render(
	ctx context.Context,
	bot provider,
	identity *identity.Identity,
	destination bot.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"templateCockroach/render",
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
		OutputPath: defaultCockroachDirName,
		Writer: &BotConfigWriter{
			ctx:     ctx,
			dest:    destination,
			subpath: defaultCockroachDirName,
		},
		Key:    key,
		Format: identityfile.FormatCockroach,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Wrote CockroachDB files", "files", files)

	return nil
}
