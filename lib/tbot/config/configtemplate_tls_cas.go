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
	// DefaultHostCAPath is the default filename for the host CA certificate
	DefaultHostCAPath = "teleport-host-ca.crt"

	// defaultUserCAPath is the default filename for the user CA certificate
	defaultUserCAPath = "teleport-user-ca.crt"

	// defaultDatabaseCAPath is the default filename for the database CA
	// certificate
	defaultDatabaseCAPath = "teleport-database-ca.crt"
)

// TemplateTLSCAs outputs Teleport's host and user CAs for miscellaneous TLS
// client use.
type TemplateTLSCAs struct {
	// HostCAPath is the path to which Teleport's host CAs will be written.
	HostCAPath string `yaml:"host_ca_path,omitempty"`

	// UserCAPath is the path to which Teleport's user CAs will be written.
	UserCAPath string `yaml:"user_ca_path,omitempty"`

	// DatabaseCAPath is the path to which Teleport's database CA will be
	// written.
	DatabaseCAPath string `yaml:"database_ca_path,omitempty"`
}

func (t *TemplateTLSCAs) CheckAndSetDefaults() error {
	// As much as it seems silly to make these configurable, some apps require
	// certs to have a certain name / file extension and it's trivial to make
	// that configurable.

	if t.HostCAPath == "" {
		t.HostCAPath = DefaultHostCAPath
	}

	if t.UserCAPath == "" {
		t.UserCAPath = defaultUserCAPath
	}

	if t.DatabaseCAPath == "" {
		t.DatabaseCAPath = defaultDatabaseCAPath
	}

	return nil
}

func (t *TemplateTLSCAs) Name() string {
	return TemplateTLSCAsName
}

func (t *TemplateTLSCAs) Describe(destination bot.Destination) []FileDescription {
	return []FileDescription{
		{
			Name: t.UserCAPath,
		},
		{
			Name: t.HostCAPath,
		},
		{
			Name: t.DatabaseCAPath,
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

func (t *TemplateTLSCAs) Render(
	ctx context.Context,
	bot provider,
	_ *identity.Identity,
	destination *DestinationConfig,
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

	dest, err := destination.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	// Note: This implementation mirrors tctl's current behavior. I've noticed
	// that mariadb at least does not seem to like being passed more than one
	// CA so there may be some compat issues to address in the future for the
	// rare case where a CA rotation is in progress.

	if err := dest.Write(t.HostCAPath, concatCACerts(hostCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(t.UserCAPath, concatCACerts(userCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(t.DatabaseCAPath, concatCACerts(databaseCAs)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
