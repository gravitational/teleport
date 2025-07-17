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
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/windows"
)

type ldapConfig struct {
	Address string

	TLSServerName string
	TLSCACert     *x509.Certificate

	Domain            string
	ServiceAccount    string
	ServiceAccountSID string
}

type ldapConnectorConfig struct {
	logger     *slog.Logger
	authClient windows.AuthInterface

	ldapConfig  ldapConfig
	clusterName string
}

type ldapConnector struct {
	ldapConnectorConfig

	dialLDAPServerFunc func(ctx context.Context) (ldap.Client, error) // only used in tests.
}

func newLDAPConnector(cfg ldapConnectorConfig) *ldapConnector {
	if cfg.logger == nil {
		cfg.logger = slog.Default()
	}

	cfg.logger = cfg.logger.With("domain", cfg.ldapConfig.Domain, "service_account", cfg.ldapConfig.ServiceAccount)

	conn := &ldapConnector{
		ldapConnectorConfig: cfg,
	}
	return conn
}

const (
	// ldapDialTimeout is the timeout for dialing the LDAP server
	// when making an initial connection
	ldapDialTimeout = 15 * time.Second
	// ldapRequestTimeout is the timeout for making LDAP requests.
	// It is larger than the dial timeout because LDAP queries in large
	// Active Directory environments may take longer to complete.
	ldapRequestTimeout = 45 * time.Second

	// attrSAMAccountName is the SAM Account name of an LDAP object.
	attrSAMAccountName = "sAMAccountName"
	// attrSAMAccountType is the SAM Account type for an LDAP object.
	attrSAMAccountType = "sAMAccountType"
	// AccountTypeUser is the SAM account type for user accounts.
	// See https://learn.microsoft.com/en-us/windows/win32/adschema/a-samaccounttype
	// (SAM_USER_OBJECT)
	AccountTypeUser = "805306368"
)

func (s *ldapConnector) dialLDAPServer(ctx context.Context) (ldap.Client, error) {
	if s.dialLDAPServerFunc != nil {
		return s.dialLDAPServerFunc(ctx)
	}

	tc, err := s.tlsConfigForLDAP(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ldapURL := "ldaps://" + s.ldapConfig.Address
	s.logger.DebugContext(ctx, "Dialing LDAP server", "url", ldapURL)

	conn, err := ldap.DialURL(
		ldapURL,
		ldap.DialWithDialer(&net.Dialer{Timeout: ldapDialTimeout}),
		ldap.DialWithTLSConfig(tc),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn.SetTimeout(ldapRequestTimeout)

	return conn, nil
}

// GetActiveDirectorySID queries LDAP to get SID of a given username.
func (s *ldapConnector) GetActiveDirectorySID(ctx context.Context, username string) (sid string, err error) {
	var activeDirectorySID string
	// Find the user's SID
	filter := windows.CombineLDAPFilters([]string{
		fmt.Sprintf("(%s=%s)", attrSAMAccountType, AccountTypeUser),
		fmt.Sprintf("(%s=%s)", attrSAMAccountName, username),
	})

	domainDN := windows.DomainDN(s.ldapConfig.Domain)

	s.logger.DebugContext(ctx, "Querying LDAP for objectSid of Windows user", "username", username, "filter", filter, "domain", domainDN)

	ldapConn, err := s.dialLDAPServer(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	lc := windows.NewLDAPClient(ldapConn)

	entries, err := lc.ReadWithFilter(domainDN, filter, []string{windows.AttrObjectSid})
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(entries) == 0 {
		return "", trace.NotFound("could not find Windows account %q", username)
	} else if len(entries) > 1 {
		s.logger.WarnContext(ctx, "found multiple entries for user, taking the first", "username", username)
	}
	activeDirectorySID, err = windows.ADSIDStringFromLDAPEntry(entries[0])
	if err != nil {
		return "", trace.Wrap(err)
	}
	s.logger.DebugContext(ctx, "Found objectSid Windows user", "username", username, "sid", activeDirectorySID)
	return activeDirectorySID, nil
}

func (s *ldapConnector) tlsConfigForLDAP(ctx context.Context) (*tls.Config, error) {
	// trim NETBIOS name from username
	user := s.ldapConfig.ServiceAccount
	if i := strings.LastIndex(s.ldapConfig.ServiceAccount, `\`); i != -1 {
		user = user[i+1:]
	}

	s.logger.DebugContext(ctx, "Requesting certificate for LDAP access", "user", user, "sid", s.ldapConfig.ServiceAccountSID, "domain", s.ldapConfig.Domain)

	if s.ldapConfig.ServiceAccountSID == "" {
		s.logger.WarnContext(ctx, "LDAP configuration is missing service account SID; querying LDAP may fail.")
	}

	req := &windows.GenerateCredentialsRequest{
		Username:           user,
		CAType:             types.DatabaseClientCA,
		TTL:                time.Hour,
		ClusterName:        s.clusterName,
		AuthClient:         s.authClient,
		Domain:             s.ldapConfig.Domain,
		ActiveDirectorySID: s.ldapConfig.ServiceAccountSID,
	}

	certPEM, keyPEM, caCerts, err := windows.DatabaseCredentials(ctx, req)
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
		ServerName:   s.ldapConfig.TLSServerName,
	}
	if s.ldapConfig.TLSCACert != nil {
		pool := x509.NewCertPool()
		pool.AddCert(s.ldapConfig.TLSCACert)
		tc.RootCAs = pool
	}

	return tc, nil
}
