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

package winpki

import (
	"context"
	"crypto/tls"
	"encoding/base32"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	// ldapDialTimeout is the timeout for dialing the LDAP server
	// when making an initial connection
	ldapDialTimeout = 15 * time.Second

	// ldapRequestTimeout is the timeout for making LDAP requests.
	// It is larger than the dial timeout because LDAP queries in large
	// Active Directory environments may take longer to complete.
	ldapRequestTimeout = 45 * time.Second
)

// LocateServer contains parameters for locating LDAP servers
// from the AD Domain
type LocateServer struct {
	// Automatically locate the LDAP server using DNS SRV records.
	// https://ldap.com/dns-srv-records-for-ldap/
	Enabled bool
	// Use LDAP site to locate servers from a specific logical site.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-adts/b645c125-a7da-4097-84a1-2fa7cea07714#gt_8abdc986-5679-42d9-ad76-b11eb5a0daba
	Site string
}

// LDAPConfig contains parameters for connecting to an LDAP server.
type LDAPConfig struct {
	// Addr is the LDAP server address in the form host:port.
	// Standard port is 636 for LDAPS.
	Addr string
	// LocateServer contains parameters for locating LDAP servers from the AD domain.
	LocateServer LocateServer
	// Domain is an Active Directory domain name, like "example.com".
	Domain string
	// Username is an LDAP username, like "EXAMPLE\Administrator", where
	// "EXAMPLE" is the NetBIOS version of Domain.
	Username string
	// SID is the SID for the user specified by Username.
	SID string
	// Logger is the logger for the service.
	Logger *slog.Logger
}

// LDAPClient is an LDAP client designed for Active Directory environments.
// It uses mutual TLS for authentication, and has the ability to discover
// LDAP servers from DNS if an explicit address is not provided.
//
// LDAPClient does not implement any form of credential refresh or certificate
// rotation. It is no longer useful after its certificate expires.
// For this reason, callers are encouraged to create clients on-demand rather
// than keeping them open for long periods of time.
type LDAPClient struct {
	cfg  *LDAPConfig
	conn *ldap.Conn
}

// DialLDAP creates a new LDAP client using the provided TLS config for client credentials.
func DialLDAP(ctx context.Context, cfg *LDAPConfig, credentials *tls.Config) (*LDAPClient, error) {
	conn, err := cfg.createConnection(ctx, credentials)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to LDAP server")
	}

	return &LDAPClient{
		cfg:  cfg,
		conn: conn,
	}, nil
}

func (l *LDAPClient) Close() error {
	return l.conn.Close()
}

// DomainDN returns the distinguished name for an Active Directory Domain.
func DomainDN(domain string) string {
	var sb strings.Builder
	parts := strings.SplitSeq(domain, ".")
	for p := range parts {
		if sb.Len() > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("DC=")
		sb.WriteString(p)
	}
	return sb.String()
}

const (
	// AttrObjectSid is the Security Identifier of an LDAP object
	AttrObjectSid = "objectSid"
	// AttrObjectClass is the object class of an LDAP object
	AttrObjectClass = "objectClass"

	// AttrSAMAccountType is the SAM Account type for an LDAP object.
	AttrSAMAccountType = "sAMAccountType"
	// AccountTypeUser is the SAM account type for user accounts.
	// See https://learn.microsoft.com/en-us/windows/win32/adschema/a-samaccounttype
	// (SAM_USER_OBJECT)
	AccountTypeUser = "805306368"

	// AttrSAMAccountName is the SAM Account name of an LDAP object.
	AttrSAMAccountName = "sAMAccountName"
)

// searchPageSize is desired page size for LDAP search. In Active Directory the default search size limit is 1000 entries,
// so in most cases the 1000 search page size will result in the optimal amount of requests made to
// LDAP server.
const searchPageSize = 1000

// convertLDAPError attempts to convert LDAP error codes to their
// equivalent trace errors.
func convertLDAPError(err error) error {
	if err == nil {
		return nil
	}

	var ldapErr *ldap.Error
	if errors.As(err, &ldapErr) {
		switch ldapErr.ResultCode {
		case ldap.ErrorNetwork:
			return trace.ConnectionProblem(err, "network error")
		case ldap.LDAPResultOperationsError:
			if strings.Contains(err.Error(), "successful bind must be completed") {
				return trace.NewAggregate(trace.AccessDenied(
					"the LDAP server did not accept Teleport's client certificate, "+
						"has the Teleport CA been imported correctly?"), err)
			}
		case ldap.LDAPResultEntryAlreadyExists:
			return trace.AlreadyExists("LDAP object already exists: %v", err)
		case ldap.LDAPResultConstraintViolation:
			return trace.BadParameter("object constraint violation: %v", err)
		case ldap.LDAPResultInsufficientAccessRights:
			return trace.AccessDenied("insufficient permissions: %v", err)
		}
	}

	return err
}

// GetActiveDirectorySID makes an LDAP query to retrieve the security identifier (SID)
// for the specified Active Directory user.
func (l *LDAPClient) GetActiveDirectorySID(ctx context.Context, username string) (string, error) {
	filter := CombineLDAPFilters([]string{
		fmt.Sprintf("(%s=%s)", AttrSAMAccountType, AccountTypeUser),
		fmt.Sprintf("(%s=%s)", AttrSAMAccountName, ldap.EscapeFilter(username)),
	})

	entries, err := l.ReadWithFilter(DomainDN(l.cfg.Domain), filter, []string{AttrObjectSid})
	switch {
	case err != nil:
		return "", trace.Wrap(err)
	case len(entries) == 0:
		return "", trace.NotFound("could not find Windows account %q", username)
	case len(entries) > 1:
		l.cfg.Logger.WarnContext(ctx, "found multiple entries for user, taking the first", "user", username)
	}

	sid, err := ADSIDStringFromLDAPEntry(entries[0])
	if err != nil {
		return "", trace.Wrap(err)
	}

	return sid, nil
}

// ReadWithFilter searches the specified DN (and its children) using the specified LDAP filter.
// See https://ldap.com/ldap-filters/ for more information on LDAP filter syntax.
func (l *LDAPClient) ReadWithFilter(dn string, filter string, attrs []string) ([]*ldap.Entry, error) {
	req := ldap.NewSearchRequest(
		dn,
		ldap.ScopeWholeSubtree,
		ldap.DerefAlways,
		0,     // no SizeLimit
		0,     // no TimeLimit
		false, // TypesOnly == false, we want attribute values
		filter,
		attrs,
		nil, // no Controls
	)

	res, err := l.conn.SearchWithPaging(req, searchPageSize)
	if err != nil {
		return nil, trace.Wrap(convertLDAPError(err), "fetching LDAP object %q with filter %q", dn, filter)
	}

	return res.Entries, nil
}

// Read fetches an LDAP entry at path and its children, if any. Only
// entries with the given class are returned and only with the specified
// attributes.
//
// You can browse LDAP on the Windows host to find the objectClass for a
// specific entry using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (l *LDAPClient) Read(dn string, class string, attrs []string) ([]*ldap.Entry, error) {
	return l.ReadWithFilter(dn, fmt.Sprintf("(%s=%s)", AttrObjectClass, class), attrs)
}

// Create creates an LDAP entry at the given path, with the given class and
// attributes. Note that AD will create a bunch of attributes for each object
// class automatically and you don't need to specify all of them.
//
// You can browse LDAP on the Windows host to find the objectClass and
// attributes for similar entries using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (l *LDAPClient) Create(dn string, class string, attrs map[string][]string) error {
	req := ldap.NewAddRequest(dn, nil)
	for k, v := range attrs {
		req.Attribute(k, v)
	}
	req.Attribute("objectClass", []string{class})

	if err := l.conn.Add(req); err != nil {
		return trace.Wrap(convertLDAPError(err), "error creating LDAP object %q", dn)
	}
	return nil
}

// CreateContainer creates an LDAP container entry if it doesn't already exist.
func (l *LDAPClient) CreateContainer(ctx context.Context, dn string) error {
	const classContainer = "container"
	err := l.Create(dn, classContainer, nil /* attrs */)
	// Ignore the error if container already exists.
	if trace.IsAlreadyExists(err) {
		return nil
	}

	return trace.Wrap(err)
}

// Update updates an LDAP entry at the given path, replacing the provided
// attributes. For each attribute in replaceAttrs, the value is completely
// replaced, not merged. If you want to modify the value of an existing
// attribute, you should read the existing value first, modify it and provide
// the final combined value in replaceAttrs.
//
// You can browse LDAP on the Windows host to find attributes of existing
// entries using ADSIEdit.msc.
func (l *LDAPClient) Update(ctx context.Context, dn string, replaceAttrs map[string][]string) error {
	req := ldap.NewModifyRequest(dn, nil)
	for k, v := range replaceAttrs {
		req.Replace(k, v)
	}

	if err := l.conn.Modify(req); err != nil {
		return trace.Wrap(convertLDAPError(err), "updating %q", dn)
	}
	return nil
}

// CombineLDAPFilters joins the slice of filters
func CombineLDAPFilters(filters []string) string {
	return "(&" + strings.Join(filters, "") + ")"
}

func crlContainerDN(domain string, caType types.CertAuthType) string {
	return fmt.Sprintf("CN=%s,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,%s", crlKeyName(caType), DomainDN(domain))
}

// CRNCN computes the common name for a Teleport CRL in Windows environments.
// The issuer SKID is optional, but should generally be set for compatibility
// with clusters having more than one issuer (like those using HSMs).
func CRLCN(issuerCN string, issuerSKID []byte) string {
	name := issuerCN
	if len(issuerSKID) > 0 {
		id := base32.HexEncoding.EncodeToString(issuerSKID)
		name = id + "_" + name
	}
	// The limit on the CN attribute should be 64 characters, but in practice
	// we observe that certutil.exe truncates the CN as soon as it exceeds 51 characters.
	return name[:min(len(name), 51)]
}

// CRLDN computes the distinguished name for a Teleport CRL in Windows environments.
// The issuer SKID is optional, but should generally be set for compatibility
// with clusters having more than one issuer (like those using HSMs).
func CRLDN(issuerCN string, issuerSKID []byte, activeDirectoryDomain string, caType types.CertAuthType) string {
	return "CN=" + CRLCN(issuerCN, issuerSKID) + "," + crlContainerDN(activeDirectoryDomain, caType)
}

// CRLDistributionPoint computes the CRL distribution point for certs issued.
func CRLDistributionPoint(activeDirectoryDomain string, caType types.CertAuthType, issuer *tlsca.CertAuthority, includeSKID bool) string {
	var issuerSKID []byte
	if includeSKID {
		issuerSKID = issuer.Cert.SubjectKeyId
	}
	crlDN := CRLDN(issuer.Cert.Subject.CommonName, issuerSKID, activeDirectoryDomain, caType)
	return fmt.Sprintf("ldap:///%s?certificateRevocationList?base?objectClass=cRLDistributionPoint", crlDN)
}

// crlKeyName returns the appropriate LDAP key given the CA type.
//
// Note: UserCA must use "Teleport" to keep backwards compatibility.
func crlKeyName(caType types.CertAuthType) string {
	switch caType {
	case types.DatabaseClientCA, types.DatabaseCA:
		return "TeleportDB"
	default:
		return "Teleport"
	}
}

// createConnection dials an LDAP server using the provided TLS config.
// The server is either obtained directly from the configuration or
// discovered via DNS.
func (c *LDAPConfig) createConnection(ctx context.Context, ldapTLSConfig *tls.Config) (*ldap.Conn, error) {
	servers := []string{c.Addr}
	dialer := net.Dialer{Timeout: ldapDialTimeout}

	if c.LocateServer.Enabled {
		dial := func(dialCtx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(dialCtx, network, address)
		}

		// In development environments, the system's default resolver is unlikely to be
		// able to resolve the Active Directory SRV records needed for server location,
		// so we allow overriding the resolver.
		if resolverAddr := os.Getenv("TELEPORT_LDAP_RESOLVER"); resolverAddr != "" {
			c.Logger.DebugContext(ctx, "Using custom DNS resolver address", "address", resolverAddr)
			// Check if resolver address has a port
			host, port, err := net.SplitHostPort(resolverAddr)
			if err != nil {
				host = resolverAddr
				port = "53"
			}

			customResolverAddr := net.JoinHostPort(host, port)
			dial = func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, customResolverAddr)
			}
		}

		resolver := &net.Resolver{
			PreferGo: true,
			Dial:     dial,
		}

		var err error
		if servers, err = locateLDAPServer(ctx, c.Domain, c.LocateServer.Site, resolver); err != nil {
			return nil, trace.Wrap(err, "locating LDAP server")
		}
	}

	if len(servers) == 0 {
		return nil, trace.NotFound("no LDAP servers found for domain %q", c.Domain)
	}

	var lastErr error
	for _, server := range servers {
		conn, err := ldap.DialURL(
			"ldaps://"+server,
			ldap.DialWithDialer(&dialer),
			ldap.DialWithTLSConfig(ldapTLSConfig),
		)

		if err == nil {
			c.Logger.DebugContext(ctx, "Connected to LDAP server", "server", server)
			conn.SetTimeout(ldapRequestTimeout)
			return conn, nil
		}
		lastErr = err

		if c.LocateServer.Enabled {
			// If the connection fails and we're using LocateServer, log that a server failed.
			c.Logger.DebugContext(ctx, "Error connecting to LDAP server, trying next available server", "server", server, "error", err)
		}
	}

	return nil, trace.NotFound("no LDAP servers responded successfully for domain %q: %v", c.Domain, lastErr)
}
