// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package kerberos

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/auth/windows"
	"github.com/gravitational/teleport/lib/srv/db/common/kerberos/kinit"
)

type clientProvider struct {
	AuthClient            windows.AuthInterface
	Logger                *slog.Logger
	kinitCommandGenerator kinit.CommandGenerator
}

// ClientProvider can create Kerberos client appropriate for given database session.
type ClientProvider interface {
	// GetKerberosClient returns Kerberos client for given user and active directory configuration.
	GetKerberosClient(ctx context.Context, ad types.AD, username string) (*client.Client, error)
}

// NewClientProvider returns new instance of ClientProvider.
func NewClientProvider(authClient windows.AuthInterface, logger *slog.Logger) ClientProvider {
	return newClientProvider(authClient, logger)
}

func newClientProvider(authClient windows.AuthInterface, logger *slog.Logger) *clientProvider {
	return &clientProvider{
		AuthClient: authClient,
		Logger:     logger,
	}
}

func (c *clientProvider) GetKerberosClient(ctx context.Context, ad types.AD, username string) (*client.Client, error) {
	switch {
	case ad.KeytabFile != "":
		kt, err := c.keytabClient(ad, username)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return kt, nil
	case ad.KDCHostName != "" && ad.LDAPCert != "":
		kt, err := c.kinitClient(ctx, ad, username, c.AuthClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return kt, nil

	}
	return nil, trace.BadParameter("configuration must have either keytab_file or kdc_host_name and ldap_cert")
}

// keytabClient returns a kerberos client using a keytab file
func (c *clientProvider) keytabClient(ad types.AD, username string) (*client.Client, error) {
	// Load keytab.
	kt, err := keytab.Load(ad.KeytabFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Load krb5.conf.
	conf, err := config.Load(ad.Krb5File)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create Kerberos client.
	kbClient := client.NewWithKeytab(
		username,
		ad.Domain,
		kt,
		conf,
		// Active Directory does not commonly support FAST negotiation.
		client.DisablePAFXFAST(true))

	// Login.
	err = kbClient.Login()
	return kbClient, trace.Wrap(err)
}

// kinitClient returns a kerberos client using a kinit ccache
func (c *clientProvider) kinitClient(ctx context.Context, ad types.AD, username string, auth windows.AuthInterface) (*client.Client, error) {
	if _, err := tlsutils.ParseCertificatePEM([]byte(ad.LDAPCert)); err != nil {
		return nil, trace.Wrap(err, "invalid certificate was provided via AD configuration")
	}
	certGetter := &kinit.DBCertGetter{
		Auth:     auth,
		Logger:   c.Logger,
		ADConfig: ad,
		UserName: username,
	}

	realmName := strings.ToUpper(ad.Domain)
	k := kinit.New(kinit.NewCommandLineInitializer(
		kinit.CommandConfig{
			AuthClient:  auth,
			User:        username,
			Realm:       realmName,
			KDCHost:     ad.KDCHostName,
			AdminServer: ad.Domain,
			LDAPCAPEM:   ad.LDAPCert,
			Command:     c.kinitCommandGenerator,
			CertGetter:  certGetter,
		}))
	// create the kinit credentials cache using the previously prepared cert/key pair
	cc, conf, err := k.UseOrCreateCredentialsCache(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Create Kerberos client from ccache. No need to login, `kinit` will have already done that.
	return client.NewFromCCache(cc, conf, client.DisablePAFXFAST(true))
}
