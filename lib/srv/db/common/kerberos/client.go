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

	"github.com/gravitational/teleport/lib/auth/windows"
	"github.com/gravitational/teleport/lib/srv/db/common"
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
	GetKerberosClient(ctx context.Context, sessionCtx *common.Session) (*client.Client, error)
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
var errBadKerberosConfig = errors.New("configuration must have either keytab or kdc_host_name and ldap_cert")

func (c *clientProvider) GetKerberosClient(ctx context.Context, sessionCtx *common.Session) (*client.Client, error) {
	switch {
	case sessionCtx.Database.GetAD().KeytabFile != "":
		kt, err := c.keytabClient(sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return kt, nil
	case sessionCtx.Database.GetAD().KDCHostName != "" && sessionCtx.Database.GetAD().LDAPCert != "":
		kt, err := c.kinitClient(ctx, sessionCtx, c.AuthClient, c.DataDir)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return kt, nil

	}
	return nil, trace.Wrap(errBadKerberosConfig)
}

// keytabClient returns a kerberos client using a keytab file
func (c *clientProvider) keytabClient(session *common.Session) (*client.Client, error) {
	// Load keytab.
	kt, err := keytab.Load(session.Database.GetAD().KeytabFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Load krb5.conf.
	conf, err := config.Load(session.Database.GetAD().Krb5File)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create Kerberos client.
	kbClient := client.NewWithKeytab(
		session.DatabaseUser,
		session.Database.GetAD().Domain,
		kt,
		conf,
		// Active Directory does not commonly support FAST negotiation.
		client.DisablePAFXFAST(true))

	// Login.
	err = kbClient.Login()
	return kbClient, err
}

// kinitClient returns a kerberos client using a kinit ccache
func (c *clientProvider) kinitClient(ctx context.Context, session *common.Session, auth windows.AuthInterface, dataDir string) (*client.Client, error) {
	ldapPem, _ := pem.Decode([]byte(session.Database.GetAD().LDAPCert))

	if ldapPem == nil {
		return nil, trace.Wrap(errBadCertificate)
	}

	cert, err := x509.ParseCertificate(ldapPem.Bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certGetter := &kinit.DBCertGetter{
		Auth:            auth,
		KDCHostName:     strings.ToUpper(session.Database.GetAD().KDCHostName),
		RealmName:       session.Database.GetAD().Domain,
		AdminServerName: session.Database.GetAD().KDCHostName,
		UserName:        session.DatabaseUser,
		LDAPCA:          cert,
	}

	realmName := strings.ToUpper(session.Database.GetAD().Domain)
	k := kinit.New(kinit.NewCommandLineInitializer(
		kinit.CommandConfig{
			AuthClient:  auth,
			User:        session.DatabaseUser,
			Realm:       realmName,
			KDCHost:     session.Database.GetAD().KDCHostName,
			AdminServer: session.Database.GetAD().Domain,
			DataDir:     dataDir,
			LDAPCA:      cert,
			LDAPCAPEM:   session.Database.GetAD().LDAPCert,
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
