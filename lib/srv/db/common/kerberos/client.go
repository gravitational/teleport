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

	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/srv/db/common/kerberos/kinit"
	"github.com/gravitational/teleport/lib/winpki"
)

type clientProvider struct {
	authClient winpki.AuthInterface
	logger     *slog.Logger

	providerFun func(logger *slog.Logger, auth winpki.AuthInterface, adConfig types.AD) (kinit.ClientProvider, error) // for testing
	skipLogin   bool                                                                                                  // for testing
}

// ClientProvider can create Kerberos client appropriate for given database session.
type ClientProvider interface {
	// GetKerberosClient returns Kerberos client for given user and active directory configuration.
	GetKerberosClient(ctx context.Context, ad types.AD, username string) (*client.Client, error)
}

// NewClientProvider returns new instance of ClientProvider.
func NewClientProvider(authClient winpki.AuthInterface, logger *slog.Logger) ClientProvider {
	return newClientProvider(authClient, logger)
}

func newClientProvider(authClient winpki.AuthInterface, logger *slog.Logger) *clientProvider {
	return &clientProvider{
		authClient: authClient,
		logger:     logger,
	}
}

func (c *clientProvider) GetKerberosClient(ctx context.Context, ad types.AD, username string) (*client.Client, error) {
	switch {
	case ad.KeytabFile != "":
		if ad.Krb5File == "" {
			return nil, trace.BadParameter("no Kerberos configuration file provided")
		}
		kt, err := c.keytabClient(ad, username)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return kt, nil
	case ad.KDCHostName != "":
		kt, err := c.kinitClient(ctx, ad, username, c.authClient)
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

	// skip login during tests when there is no actual AD to connect to.
	if c.skipLogin {
		return kbClient, nil
	}

	err = kbClient.Login()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return kbClient, nil
}

// kinitClient returns a kerberos client using a kinit ccache
func (c *clientProvider) kinitClient(ctx context.Context, ad types.AD, username string, auth winpki.AuthInterface) (*client.Client, error) {
	if _, err := tlsutils.ParseCertificatePEM([]byte(ad.LDAPCert)); err != nil {
		return nil, trace.Wrap(err, "invalid certificate was provided via AD configuration")
	}

	if c.providerFun == nil {
		c.providerFun = kinit.NewProviderExternalExecutable
	}
	provider, err := c.providerFun(c.logger, auth, ad)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return provider.CreateClient(ctx, username)
}
