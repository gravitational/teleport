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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/jcmturner/gokrb5/v8/credentials"

	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/jcmturner/gokrb5/v8/spnego"

	"github.com/gravitational/trace"
)

// getAuth returns Kerberos authenticator used by SQL Server driver.
//
// TODO(r0mant): Unit-test this. In-memory Kerberos server?
func (c *connector) getAuth(sessionCtx *common.Session) (*krbAuth, error) {
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

func (c *connector) getAuth2(sessionCtx *common.Session) (*krbAuth, error) {
	certPEM, keyPEM, err := c.generateCredentials(context.Background(), sessionCtx.DatabaseUser, sessionCtx.Database.GetAD().Domain, time.Hour*24*265)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certFile := filepath.Join(tmpDir, "cert.pem")
	certKey := filepath.Join(tmpDir, "key.pem")

	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := os.WriteFile(certKey, keyPEM, 0600); err != nil {
		return nil, trace.Wrap(err)
	}

	var buf bytes.Buffer
	err = krbConfigTemplate.Execute(&buf, &keytabTemplate{
		Realm:       strings.ToUpper(sessionCtx.Database.GetAD().Domain),
		KDC:         strings.ToLower(sessionCtx.Database.GetAD().Domain),
		AdminServer: strings.ToLower(sessionCtx.Database.GetAD().Domain),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	krb5File := filepath.Join(tmpDir, "krb5.conf")
	if err := os.WriteFile(krb5File, buf.Bytes(), 0500); err != nil {
		return nil, trace.Wrap(err)
	}

	kinitFile := filepath.Join(tmpDir, "kinit.cache")

	cmd := exec.Cmd{
		Env: []string{
			fmt.Sprintf("KRB5_CONFIG=%s", krb5File),
			fmt.Sprintf("KRB5_TRACE=/dev/stdout"),
		},
		Path: "kinit",
		Args: []string{
			"-X", anchors(os.Getenv("WINDOWS_CA")),
			"-X", userIdentity(certFile, certKey),
			"-c", kinitFile,
			sessionCtx.DatabaseUser,
		},
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cmd = cmd
	if err := cmd.Run(); err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := credentials.LoadCCache(kinitFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := config.NewFromString(buf.String())
	if err != nil {
		return nil, trace.Wrap(err)

	}
	// Create Kerberos client.
	client, err := client.NewFromCCache(
		creds,
		cc,
		client.DisablePAFXFAST(true),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

//      -X X509_anchors=FILE:/home/ec2-user/user-ca.pem  \
//      -X X509_user_identity=FILE:/home/ec2-user/1655818904/cert.pem,/home/ec2-user/1655818904/key.pem alice \
func anchors(ca string) string {
	return fmt.Sprintf("X509_anchors=FILE:%s", ca)
}
func userIdentity(cert, key string) string {
	return fmt.Sprintf(" X509_user_identity=FILE:%s,%s", cert, key)
}

type keytabTemplate struct {
	Realm       string
	KDC         string
	AdminServer string
}

var krbConfigTemplate = template.Must(template.New("krbConfigTemplate").Parse(`
[libdefaults]
 default_realm = {{.Realm}}
 rdns = false

[realms]
 {{.Realm}} = {
  kdc = {{.KDC}}
  admin_server = {{.AdminServer}}
  pkinit_eku_checking = kpServerAuth
  pkinit_kdc_hostname = {{.KDC}}
 }
`))
