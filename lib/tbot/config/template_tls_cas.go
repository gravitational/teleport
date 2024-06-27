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
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const (
	// HostCAPath is the default filename for the host CA certificate
	HostCAPath = "teleport-host-ca.crt"

	// UserCAPath is the default filename for the user CA certificate
	UserCAPath = "teleport-user-ca.crt"

	// DatabaseCAPath is the default filename for the database CA
	// certificate
	DatabaseCAPath = "teleport-database-ca.crt"
)

// templateTLSCAs outputs Teleport's host and user CAs for miscellaneous TLS
// client use.
type templateTLSCAs struct{}

func (t *templateTLSCAs) name() string {
	return TemplateTLSCAsName
}

func (t *templateTLSCAs) describe() []FileDescription {
	return []FileDescription{
		{
			Name: HostCAPath,
		},
		{
			Name: UserCAPath,
		},
		{
			Name: DatabaseCAPath,
		},
	}
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

func (t *templateTLSCAs) render(
	ctx context.Context,
	bot provider,
	_ *identity.Identity,
	destination bot.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"templateTLSCAs/render",
	)
	defer span.End()

	hostCAs, err := bot.GetCertAuthorities(ctx, types.HostCA)
	if err != nil {
		return trace.Wrap(err)
	}

	userCAs, err := bot.GetCertAuthorities(ctx, types.UserCA)
	if err != nil {
		return trace.Wrap(err)
	}

	databaseCAs, err := bot.GetCertAuthorities(ctx, types.DatabaseCA)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note: This implementation mirrors tctl's current behavior. I've noticed
	// that mariadb at least does not seem to like being passed more than one
	// CA so there may be some compat issues to address in the future for the
	// rare case where a CA rotation is in progress.
	if err := destination.Write(ctx, HostCAPath, concatCACerts(hostCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := destination.Write(ctx, UserCAPath, concatCACerts(userCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := destination.Write(ctx, DatabaseCAPath, concatCACerts(databaseCAs)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
