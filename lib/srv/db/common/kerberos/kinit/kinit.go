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

// Package kinit provides utilities for interacting with a KDC (Key Distribution Center) for Kerberos5
package kinit

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/winpki"
)

type ClientProvider interface {
	// CreateClient returns a client logged in as particular username, ready to use.
	CreateClient(ctx context.Context, username string) (*client.Client, error)
}

// NewProviderExternalExecutable returns a new CredentialProvider which performs PKINIT using an external, preinstalled kinit binary.
func NewProviderExternalExecutable(logger *slog.Logger, auth winpki.AuthInterface, adConfig types.AD) (ClientProvider, error) {
	return newKinitProvider(logger, auth, adConfig)
}

func newKinitProvider(logger *slog.Logger, auth winpki.AuthInterface, adConfig types.AD) (*kinitProvider, error) {
	if logger == nil {
		logger = slog.Default()
	}

	connector, err := newLDAPConnector(logger, auth, adConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	krb5Config, err := newKrb5Config(adConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	provider := &kinitProvider{
		ldapCertificatePEM: adConfig.LDAPCert,
		runner: &execCommandRunner{
			logger: logger,
		},
		krb5Config: krb5Config,
		certGetter: &dbCertGetter{
			logger:        logger,
			auth:          auth,
			domain:        adConfig.Domain,
			ldapConnector: connector,
		},
		logger: logger,
	}
	return provider, nil
}

func newKrb5Config(config types.AD) (string, error) {
	data := map[string]string{
		"RealmName":       strings.ToUpper(config.Domain),
		"KDCHostName":     config.KDCHostName,
		"AdminServerName": config.Domain,
	}

	const krb5ConfigTemplate = `[libdefaults]
 default_realm = {{ .RealmName }}
 rdns = false


[realms]
 {{ .RealmName }} = {
  kdc = {{ .AdminServerName }}
  admin_server = {{ .AdminServerName }}
  pkinit_eku_checking = kpServerAuth
  pkinit_kdc_hostname = {{ .KDCHostName }}
 }`
	tpl, err := template.New("krb_conf").Parse(krb5ConfigTemplate)
	if err != nil {
		return "", trace.Wrap(err)
	}
	b := bytes.NewBuffer([]byte{})
	err = tpl.Execute(b, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return b.String(), nil
}

type execCommandRunner struct {
	logger *slog.Logger
}

func (e *execCommandRunner) runCommand(ctx context.Context, env map[string]string, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	e.logger.DebugContext(ctx, "running command", "cmd", cmd, "args", args, "env", cmd.Env)
	output, err := cmd.CombinedOutput()
	return string(output), trace.Wrap(err)
}

// kinitProvider performs PKINIT using an external, preinstalled kinit binary.
type kinitProvider struct {
	krb5Config string

	runner     commandRunner
	certGetter certGetter

	ldapCertificatePEM string
	logger             *slog.Logger
}

type commandRunner interface {
	runCommand(ctx context.Context, env map[string]string, command string, args ...string) (string, error)
}

type certGetter interface {
	getCertificate(ctx context.Context, username string) (*getCertificateResult, error)
}

type dbCertGetter struct {
	logger        *slog.Logger
	auth          winpki.AuthInterface
	domain        string
	ldapConnector LDAPConnector
}

type getCertificateResult struct {
	certPEM []byte
	keyPEM  []byte
	caCert  []byte

	sidLookupError error
}

// GetCertificateBytes returns a new cert/key pem and the DB CA bytes
func (d *dbCertGetter) getCertificate(ctx context.Context, username string) (*getCertificateResult, error) {
	clusterName, err := d.auth.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sid, sidLookupError := d.ldapConnector.GetActiveDirectorySID(ctx, username)
	if sidLookupError != nil {
		d.logger.WarnContext(ctx, "Failed to get SID from ActiveDirectory; PKINIT flow is likely to fail.", "error", sidLookupError)
	}

	req := &winpki.GenerateCredentialsRequest{
		CAType:             types.DatabaseClientCA,
		TTL:                time.Minute * 10,
		Domain:             d.domain,
		ClusterName:        clusterName.GetClusterName(),
		OmitCDP:            true,
		Username:           username,
		ActiveDirectorySID: sid,
	}

	certPEM, keyPEM, caCerts, err := winpki.DatabaseCredentials(ctx, d.auth, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &getCertificateResult{
		certPEM:        certPEM,
		keyPEM:         keyPEM,
		caCert:         bytes.Join(caCerts, []byte("\n")),
		sidLookupError: sidLookupError,
	}, nil
}

const (
	// kinitBinary is the binary Name for the kinit executable
	kinitBinary = "kinit"
)

func (k *kinitProvider) CreateClient(ctx context.Context, username string) (*client.Client, error) {
	tmp, err := os.MkdirTemp("", "kinit")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		err = os.RemoveAll(tmp)
		if err != nil {
			k.logger.ErrorContext(ctx, "failed removing temporary kinit directory", "error", err)
		}
	}()

	certPath := filepath.Join(tmp, "cert.pem")
	keyPath := filepath.Join(tmp, "key.pem")
	userCAPath := filepath.Join(tmp, "userca.pem")
	cachePath := filepath.Join(tmp, "login.ccache")

	certResult, err := k.certGetter.getCertificate(ctx, username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = os.WriteFile(certPath, certResult.certPEM, 0644)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = os.WriteFile(keyPath, certResult.keyPEM, 0644)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = os.WriteFile(userCAPath, k.buildAnchorsFileContents(certResult.caCert), 0644)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	krbConfPath := filepath.Join(tmp, "krb5.conf")
	err = os.WriteFile(krbConfPath, []byte(k.krb5Config), 0600)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conf, err := config.Load(krbConfPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	env := map[string]string{
		"KRB5_CONFIG": krbConfPath,
		"KRB5_TRACE":  "/dev/stdout",
	}

	output, err := k.runner.runCommand(ctx,
		env,
		kinitBinary,
		"-X", fmt.Sprintf("X509_anchors=FILE:%s", userCAPath),
		"-X", fmt.Sprintf("X509_user_identity=FILE:%s,%s", certPath, keyPath),
		"-c", cachePath,
		"--",
		username,
	)
	if err != nil {
		k.logger.ErrorContext(ctx, "Failed to authenticate with KDC", "command_output", output, "error", err)
		if certResult.sidLookupError != nil {
			k.logger.WarnContext(ctx, "The failed request was made with empty SID due to LDAP lookup failure. AD servers are likely to reject such requests. The lookup error may be due to non-existent user or invalid configuration.", "sid_lookup_error", certResult.sidLookupError)
		}
		return nil, trace.Wrap(err)
	}

	ccache, err := credentials.LoadCCache(cachePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.NewFromCCache(ccache, conf, client.DisablePAFXFAST(true))
}

// buildAnchorsFileContents generates the contents of the anchors file (pkinit).
// The file must contain the Teleport DB CA and the KDB/LDAP CA, otherwise the
// connections will fail.
func (k *kinitProvider) buildAnchorsFileContents(caBytes []byte) []byte {
	return append(caBytes, []byte(k.ldapCertificatePEM)...)
}
