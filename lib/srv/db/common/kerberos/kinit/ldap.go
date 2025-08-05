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

package kinit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/winpki"
)

type ldapConnectionConfig struct {
	address string

	tlsServerName string
	tlsCACert     *x509.Certificate

	domain            string
	serviceAccount    string
	serviceAccountSID string
}

type ldapConnector struct {
	logger     *slog.Logger
	authClient winpki.AuthInterface

	ldapConfig ldapConnectionConfig
}

type LDAPConnector interface {
	GetActiveDirectorySID(ctx context.Context, username string) (sid string, err error)
}

func newLDAPConnector(logger *slog.Logger, authClient winpki.AuthInterface, adConfig types.AD) (*ldapConnector, error) {
	if authClient == nil {
		return nil, trace.BadParameter("auth client is missing")
	}
	if adConfig.LDAPServiceAccountName == "" {
		return nil, trace.BadParameter("missing LDAP service account name")
	}
	if adConfig.LDAPServiceAccountSID == "" {
		return nil, trace.BadParameter("missing LDAP service account SID")
	}
	if adConfig.Domain == "" {
		return nil, trace.BadParameter("missing AD domain")
	}
	if adConfig.KDCHostName == "" {
		return nil, trace.BadParameter("missing KDC host name / LDAP address")
	}

	ldapCert, err := tlsca.ParseCertificatePEM([]byte(adConfig.LDAPCert))
	if err != nil {
		return nil, trace.Wrap(err, "cannot find valid LDAP certificate block in AD configuration")
	}

	cfg := ldapConnectionConfig{
		address:           adConfig.KDCHostName,
		tlsServerName:     adConfig.KDCHostName,
		domain:            adConfig.Domain,
		serviceAccount:    adConfig.LDAPServiceAccountName,
		serviceAccountSID: adConfig.LDAPServiceAccountSID,
		tlsCACert:         ldapCert,
	}

	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("domain", cfg.domain, "service_account", cfg.serviceAccount)

	return &ldapConnector{
		logger:     logger,
		authClient: authClient,
		ldapConfig: cfg,
	}, nil
}

// GetActiveDirectorySID queries LDAP to get SID of a given username.
func (s *ldapConnector) GetActiveDirectorySID(ctx context.Context, username string) (sid string, err error) {
	clusterName, err := s.authClient.GetClusterName(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	tc, err := s.tlsConfigForLDAP(ctx, clusterName.GetClusterName())
	if err != nil {
		return "", trace.Wrap(err)
	}

	s.logger.DebugContext(ctx, "Querying LDAP for objectSid of Windows user", "username", username)
	client, err := winpki.DialLDAP(ctx, &winpki.LDAPConfig{
		Addr:   s.ldapConfig.address,
		Domain: s.ldapConfig.domain,
		Logger: s.logger,
	}, tc)
	if err != nil {
		return "", trace.Wrap(err)
	}

	defer client.Close()

	return client.GetActiveDirectorySID(ctx, username)
}

func (s *ldapConnector) tlsConfigForLDAP(ctx context.Context, clusterName string) (*tls.Config, error) {
	// trim NETBIOS name from username
	user := s.ldapConfig.serviceAccount
	if i := strings.LastIndex(s.ldapConfig.serviceAccount, `\`); i != -1 {
		user = user[i+1:]
	}

	s.logger.DebugContext(ctx, "Requesting certificate for LDAP access", "user", user, "sid", s.ldapConfig.serviceAccountSID, "domain", s.ldapConfig.domain)

	if s.ldapConfig.serviceAccountSID == "" {
		s.logger.WarnContext(ctx, "LDAP configuration is missing service account SID; querying LDAP may fail.")
	}

	req := &winpki.GenerateCredentialsRequest{
		Username:           user,
		CAType:             types.DatabaseClientCA,
		TTL:                time.Hour,
		ClusterName:        clusterName,
		Domain:             s.ldapConfig.domain,
		ActiveDirectorySID: s.ldapConfig.serviceAccountSID,
	}

	certPEM, keyPEM, caCerts, err := winpki.DatabaseCredentials(ctx, s.authClient, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.logger.DebugContext(ctx, "Received credentials for LDAP access", "ignored_ca_cert_count", len(caCerts))

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tc := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   s.ldapConfig.tlsServerName,
	}
	if s.ldapConfig.tlsCACert != nil {
		pool := x509.NewCertPool()
		pool.AddCert(s.ldapConfig.tlsCACert)
		tc.RootCAs = pool
	}

	return tc, nil
}
