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

package sqlserver

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
	"github.com/jcmturner/gokrb5/v8/spnego"

	"github.com/gravitational/teleport/lib/auth/windows"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/kinit"
)

var (
	errBadCertificate = errors.New("invalid certificate was provided via AD configuration")
)

// keytabClient returns a kerberos client using a keytab file
func (c *connector) keytabClient(session *common.Session) (*client.Client, error) {
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
func (c *connector) kinitClient(ctx context.Context, session *common.Session, auth windows.AuthInterface, dataDir string) (*client.Client, error) {
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

// getAuth returns Kerberos authenticator used by SQL Server driver.
//
// TODO(r0mant): Unit-test this. In-memory Kerberos server?
func (c *connector) getAuth(sessionCtx *common.Session, kbClient *client.Client) (*krbAuth, error) {
	// Obtain service ticket for the database's Service Principal Name.
	ticket, encryptionKey, err := kbClient.GetServiceTicket(sessionCtx.Database.GetAD().SPN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create init negotiation token.
	initToken, err := spnego.NewNegTokenInitKRB5(kbClient, ticket, encryptionKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal init negotiation token.
	initTokenBytes, err := initToken.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &krbAuth{
		initToken: initTokenBytes,
	}, nil
}

// krbAuth implements SQL Server driver's "auth" interface used during login
// to provide Kerberos authentication.
type krbAuth struct {
	initToken []byte
}

func (a *krbAuth) InitialBytes() ([]byte, error) {
	return a.initToken, nil
}

func (a *krbAuth) NextBytes(bytes []byte) ([]byte, error) {
	return nil, nil
}

func (a *krbAuth) Free() {}
