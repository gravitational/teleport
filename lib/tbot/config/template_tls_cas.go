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
	"github.com/gravitational/teleport/lib/auth"
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
	trusted := auth.AuthoritiesToTrustedCerts(cas)

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
	if err := destination.Write(HostCAPath, concatCACerts(hostCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := destination.Write(UserCAPath, concatCACerts(userCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := destination.Write(DatabaseCAPath, concatCACerts(databaseCAs)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
