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

package sqlserver

import (
	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/jcmturner/gokrb5/v8/spnego"

	"github.com/gravitational/teleport/lib/cloud/clients"
	"github.com/gravitational/trace"
)

// getAuth returns Kerberos authenticator used by SQL Server driver.
//
// TODO(r0mant): Unit-test this. In-memory Kerberos server?
func (c *connector) getAuth(sessionCtx *clients.Session) (*krbAuth, error) {
	// Load keytab.
	keytab, err := keytab.Load(sessionCtx.Database.GetAD().KeytabFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Load krb5.conf.
	config, err := config.Load(sessionCtx.Database.GetAD().Krb5File)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create Kerberos client.
	client := client.NewWithKeytab(
		sessionCtx.DatabaseUser,
		sessionCtx.Database.GetAD().Domain,
		keytab,
		config,
		// Active Directory does not commonly support FAST negotiation.
		client.DisablePAFXFAST(true))

	// Login.
	err = client.Login()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Obtain service ticket for the database's Service Principal Name.
	ticket, encryptionKey, err := client.GetServiceTicket(sessionCtx.Database.GetAD().SPN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create init negotiation token.
	initToken, err := spnego.NewNegTokenInitKRB5(client, ticket, encryptionKey)
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
