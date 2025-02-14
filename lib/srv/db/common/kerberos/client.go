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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/windows"
	"github.com/gravitational/teleport/lib/srv/db/common/kerberos/kinit"
)

type clientProvider struct {
	AuthClient windows.AuthInterface
	DataDir    string

	kinitCommandGenerator kinit.CommandGenerator
}

// ClientProvider can create Kerberos client appropriate for given database session.
type ClientProvider interface {
	// GetKerberosClient returns Kerberos client for given user and active directory configuration.
	GetKerberosClient(ctx context.Context, ad types.AD, username string) (*client.Client, error)
}

func NewClientProvider(authClient windows.AuthInterface, dataDir string) ClientProvider {
	return newClientProvider(authClient, dataDir)
}

func newClientProvider(authClient windows.AuthInterface, dataDir string) *clientProvider {
	return &clientProvider{
		AuthClient: authClient,
		DataDir:    dataDir,
	}
}

var errBadCertificate = errors.New("invalid certificate was provided via AD configuration")
var errBadKerberosConfig = errors.New("configuration must have either keytab_file or kdc_host_name and ldap_cert")

func (c *clientProvider) GetKerberosClient(ctx context.Context, ad types.AD, username string) (*client.Client, error) {
	switch {
	case ad.KeytabFile != "":
		kt, err := c.keytabClient(ad, username)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return kt, nil
	case ad.KDCHostName != "" && ad.LDAPCert != "":
		kt, err := c.kinitClient(ctx, ad, username, c.AuthClient, c.DataDir)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return kt, nil

	}
	return nil, trace.Wrap(errBadKerberosConfig)
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
func (c *clientProvider) kinitClient(ctx context.Context, ad types.AD, username string, auth windows.AuthInterface, dataDir string) (*client.Client, error) {
	ldapPem, _ := pem.Decode([]byte(ad.LDAPCert))

	if ldapPem == nil {
		return nil, trace.Wrap(errBadCertificate)
	}

	cert, err := x509.ParseCertificate(ldapPem.Bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certGetter := &kinit.DBCertGetter{
		Auth:            auth,
		KDCHostName:     strings.ToUpper(ad.KDCHostName),
		RealmName:       ad.Domain,
		AdminServerName: ad.KDCHostName,
		UserName:        username,
		LDAPCA:          cert,
	}

	realmName := strings.ToUpper(ad.Domain)
	k := kinit.New(kinit.NewCommandLineInitializer(
		kinit.CommandConfig{
			AuthClient:  auth,
			User:        username,
			Realm:       realmName,
			KDCHost:     ad.KDCHostName,
			AdminServer: ad.Domain,
			DataDir:     dataDir,
			LDAPCA:      cert,
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
