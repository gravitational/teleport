/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
)

func writeIdentityFile(
	ctx context.Context, log *slog.Logger, key *client.Key, dest bot.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"writeIdentityFile",
	)
	defer span.End()

	cfg := identityfile.WriteConfig{
		OutputPath: config.IdentityFilePath,
		Writer:     config.NewBotConfigWriter(ctx, dest),
		Key:        key,
		Format:     identityfile.FormatFile,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Wrote identity file", "files", files)
	return nil
}

// writeIdentityFileTLS writes the identity file in TLS format according to the
// core identityfile.Write method. This isn't usually needed but can be
// useful when writing out TLS certificates with alternative prefix and file
// extensions for application compatibility reasons.
func writeIdentityFileTLS(
	ctx context.Context, log *slog.Logger, key *client.Key, dest bot.Destination,
) error {
	cfg := identityfile.WriteConfig{
		OutputPath: config.DefaultTLSPrefix,
		Writer:     config.NewBotConfigWriter(ctx, dest, ""),
		Key:        key,
		Format:     identityfile.FormatTLS,

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

// concatCACerts borrow's identityfile's CA cert concat method.
func concatCACerts(cas []types.CertAuthority) []byte {
	trusted := authclient.AuthoritiesToTrustedCerts(cas)

	var caCerts []byte
	for _, ca := range trusted {
		for _, cert := range ca.TLSCertificates {
			caCerts = append(caCerts, cert...)
		}
	}

	return caCerts
}

// writeTLSCAs writes the three "main" TLS CAs to disk.
// TODO(noah): This is largely a copy of templateTLSCAs. We should reconsider
// which CAs are actually worth writing for each type of service because
// it seems inefficient to write the "Database" CA for a Kubernetes output.
func writeTLSCAs(ctx context.Context, dest bot.Destination, hostCAs, userCAs, databaseCAs []types.CertAuthority) error {
	ctx, span := tracer.Start(
		ctx,
		"writeTLSCAs",
	)
	defer span.End()

	// Note: This implementation mirrors tctl's current behavior. I've noticed
	// that mariadb at least does not seem to like being passed more than one
	// CA so there may be some compat issues to address in the future for the
	// rare case where a CA rotation is in progress.
	if err := dest.Write(ctx, config.HostCAPath, concatCACerts(hostCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(ctx, config.UserCAPath, concatCACerts(userCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(ctx, config.DatabaseCAPath, concatCACerts(databaseCAs)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
