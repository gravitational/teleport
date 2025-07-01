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
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
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

// LDAPConfig contains parameters for connecting to an LDAP server.
type LDAPConfig struct {
	// Addr is the LDAP server address in the form host:port.
	// Standard port is 636 for LDAPS.
	Addr string
	// Domain is an Active Directory domain name, like "example.com".
	Domain string
	// Username is an LDAP username, like "EXAMPLE\Administrator", where
	// "EXAMPLE" is the NetBIOS version of Domain.
	Username string
	// SID is the SID for the user specified by Username.
	SID string
	// InsecureSkipVerify decides whether we skip verifying with the LDAP server's CA when making the LDAPS connection.
	InsecureSkipVerify bool
	// ServerName is the name of the LDAP server for TLS.
	ServerName string
	// CA is an optional CA cert to be used for verification if InsecureSkipVerify is set to false.
	CA *x509.Certificate
	// Automatically locate the LDAP server using DNS SRV records.
	// https://ldap.com/dns-srv-records-for-ldap/
	LocateServer bool
	// Use LDAP site to locate servers from a specific logical site.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-adts/b645c125-a7da-4097-84a1-2fa7cea07714#gt_8abdc986-5679-42d9-ad76-b11eb5a0daba
	Site string
	// Logger is the logger for the service.
	Logger *slog.Logger
}

// CheckAndSetDefaults verifies this LDAPConfig
func (cfg *LDAPConfig) CheckAndSetDefaults() error {
	cfg.Logger = cmp.Or(cfg.Logger, slog.With(teleport.ComponentKey, teleport.ComponentWindowsDesktop))

	if cfg.Addr == "" && !cfg.LocateServer {
		return trace.BadParameter("Addr is required if locate_server is false in LDAPConfig")
	}
	if !cfg.LocateServer && cfg.Site != "" {
		cfg.Logger.WarnContext(context.Background(), "Site is set, but locate_server is false. Site will be ignored.")
	}
	if cfg.Domain == "" {
		return trace.BadParameter("missing Domain in LDAPConfig")
	}
	if cfg.Username == "" {
		return trace.BadParameter("missing Username in LDAPConfig")
	}

	return nil
}

// DomainDN returns the distinguished name for an Active Directory Domain.
func DomainDN(domain string) string {
	var sb strings.Builder
	parts := strings.Split(domain, ".")
	for _, p := range parts {
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
)

// classContainer is the object class for containers in Active Directory
const classContainer = "container"

// searchPageSize is desired page size for LDAP search. In Active Directory the default search size limit is 1000 entries,
// so in most cases the 1000 search page size will result in the optimal amount of requests made to
// LDAP server.
const searchPageSize = 1000

// LDAPClient is a windows LDAP client.
//
// It does not automatically detect when the underlying connection
// is closed. Callers should check for trace.ConnectionProblem errors
// and provide a new client with [SetClient].
type LDAPClient struct {
	cfg LDAPConfig
}

// NewLDAPClient returns new LDAPClient. Parameter client may be nil.
func NewLDAPClient(cfg LDAPConfig) *LDAPClient {
	return &LDAPClient{
		cfg: cfg,
	}
}

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

// ReadWithFilter searches the specified DN (and its children) using the specified LDAP filter.
// See https://ldap.com/ldap-filters/ for more information on LDAP filter syntax.
func (c *LDAPClient) ReadWithFilter(ctx context.Context, dn string, filter string, attrs []string, ldapTlsConfig *tls.Config) ([]*ldap.Entry, error) {
	client, err := c.cfg.createConnection(ctx, ldapTlsConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating LDAP client")
	}
	defer client.Close()

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

	res, err := client.SearchWithPaging(req, searchPageSize)
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
func (c *LDAPClient) Read(ctx context.Context, dn string, class string, attrs []string, ldapTlsConfig *tls.Config) ([]*ldap.Entry, error) {
	return c.ReadWithFilter(ctx, dn, fmt.Sprintf("(%s=%s)", AttrObjectClass, class), attrs, ldapTlsConfig)
}

// Create creates an LDAP entry at the given path, with the given class and
// attributes. Note that AD will create a bunch of attributes for each object
// class automatically and you don't need to specify all of them.
//
// You can browse LDAP on the Windows host to find the objectClass and
// attributes for similar entries using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (c *LDAPClient) Create(ctx context.Context, dn string, class string, attrs map[string][]string, ldapTlsConfig *tls.Config) error {
	client, err := c.cfg.createConnection(ctx, ldapTlsConfig)
	if err != nil {
		return trace.Wrap(err, "creating LDAP client")
	}
	defer client.Close()

	req := ldap.NewAddRequest(dn, nil)
	for k, v := range attrs {
		req.Attribute(k, v)
	}
	req.Attribute("objectClass", []string{class})

	if err := client.Add(req); err != nil {
		return trace.Wrap(convertLDAPError(err), "error creating LDAP object %q", dn)
	}
	return nil
}

// CreateContainer creates an LDAP container entry if
// it doesn't already exist.
func (c *LDAPClient) CreateContainer(ctx context.Context, dn string, ldapTlsConfig *tls.Config) error {
	err := c.Create(ctx, dn, classContainer, nil, ldapTlsConfig)
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
func (c *LDAPClient) Update(ctx context.Context, dn string, replaceAttrs map[string][]string, ldapTlsConfig *tls.Config) error {
	client, err := c.cfg.createConnection(ctx, ldapTlsConfig)
	if err != nil {
		return trace.Wrap(err, "creating LDAP client")
	}
	defer client.Close()

	req := ldap.NewModifyRequest(dn, nil)
	for k, v := range replaceAttrs {
		req.Replace(k, v)
	}

	if err := client.Modify(req); err != nil {
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

func crlDN(clusterName string, activeDirectoryDomain string, caType types.CertAuthType) string {
	return "CN=" + clusterName + "," + crlContainerDN(activeDirectoryDomain, caType)
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
func (c *LDAPConfig) createConnection(ctx context.Context, ldapTlsConfig *tls.Config) (*ldap.Conn, error) {
	dnsDialer := net.Dialer{
		Timeout: ldapDialTimeout,
	}

	servers := []string{c.Addr}
	if c.LocateServer {
		var resolver *net.Resolver
		resolverAddr := os.Getenv("TELEPORT_DESKTOP_ACCESS_RESOLVER_IP")
		dial := func(dialCtx context.Context, network, address string) (net.Conn, error) {
			return dnsDialer.DialContext(dialCtx, network, address)
		}
		if resolverAddr != "" {
			c.Logger.DebugContext(ctx, "Using custom DNS resolver address", "address", resolverAddr)
			// Check if resolver address has a port
			host, port, err := net.SplitHostPort(resolverAddr)
			if err != nil {
				host = resolverAddr
				port = "53"
			}
			customResolverAddr := net.JoinHostPort(host, port)

			dial = func(ctx context.Context, network, address string) (net.Conn, error) {
				return dnsDialer.DialContext(ctx, network, customResolverAddr)
			}
		}
		resolver = &net.Resolver{
			PreferGo: true,
			Dial:     dial,
		}

		var err error
		if servers, err = locateLDAPServer(ctx, c.Domain, c.Site, resolver); err != nil {
			return nil, trace.Wrap(err, "locating LDAP server")
		}
	}

	if len(servers) == 0 {
		return nil, trace.NotFound("no LDAP servers found for domain %q", c.Domain)
	}

	for _, server := range servers {
		conn, err := ldap.DialURL(
			"ldaps://"+server,
			ldap.DialWithDialer(&dnsDialer),
			ldap.DialWithTLSConfig(ldapTlsConfig),
		)

		if err != nil {
			// If the connection fails, try the next server
			c.Logger.DebugContext(ctx, "Error connecting to LDAP server, trying next available server", "server", server, "error", err)
			continue
		}

		conn.SetTimeout(ldapRequestTimeout)
		return conn, nil
	}

	return nil, trace.NotFound("no LDAP servers responded successfully for domain %q", c.Domain)
}
