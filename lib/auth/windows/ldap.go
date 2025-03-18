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

package windows

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	ber "github.com/go-asn1-ber/asn1-ber"
	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// LDAPConfig contains parameters for connecting to an LDAP server.
type LDAPConfig struct {
	// Addr is the LDAP server address in the form host:port.
	// Standard port is 636 for LDAPS.
	Addr string //nolint:unused // False-positive
	// Domain is an Active Directory domain name, like "example.com".
	Domain string //nolint:unused // False-positive
	// Username is an LDAP username, like "EXAMPLE\Administrator", where
	// "EXAMPLE" is the NetBIOS version of Domain.
	Username string //nolint:unused // False-positive
	// SID is the SID for the user specified by Username.
	SID string //nolint:unused // False-positive
	// InsecureSkipVerify decides whether we skip verifying with the LDAP server's CA when making the LDAPS connection.
	InsecureSkipVerify bool //nolint:unused // False-positive
	// ServerName is the name of the LDAP server for TLS.
	ServerName string //nolint:unused // False-positive
	// CA is an optional CA cert to be used for verification if InsecureSkipVerify is set to false.
	CA *x509.Certificate //nolint:unused // False-positive
}

// Check verifies this LDAPConfig
func (cfg LDAPConfig) Check() error {
	if cfg.Addr == "" {
		return trace.BadParameter("missing Addr in LDAPConfig")
	}
	if cfg.Domain == "" {
		return trace.BadParameter("missing Domain in LDAPConfig")
	}
	if cfg.Username == "" {
		return trace.BadParameter("missing Username in LDAPConfig")
	}
	return nil
}

// DomainDN returns the distinguished name for the domain
func (cfg LDAPConfig) DomainDN() string {
	return ToDN(cfg.Domain)
}

func ToDN(domain string) string {
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

// See: https://docs.microsoft.com/en-US/windows/security/identity-protection/access-control/security-identifiers
const (
	// WritableDomainControllerGroupID is the windows security identifier for dcs with write permissions
	WritableDomainControllerGroupID = "516"
	// ReadOnlyDomainControllerGroupID is the windows security identifier for read only dcs
	ReadOnlyDomainControllerGroupID = "521"
)

const (
	// ClassComputer is the object class for computers in Active Directory
	ClassComputer = "computer"
	// ClassContainer is the object class for containers in Active Directory
	ClassContainer = "container"
	// ClassGMSA is the object class for group managed service accounts in Active Directory.
	ClassGMSA = "msDS-GroupManagedServiceAccount"

	// AccountTypeUser is the SAM account type for user accounts.
	// See https://learn.microsoft.com/en-us/windows/win32/adschema/a-samaccounttype
	// (SAM_USER_OBJECT)
	AccountTypeUser = "805306368"

	// AttrName is the name of an LDAP object
	AttrName = "name"
	// AttrSAMAccountName is the SAM Account name of an LDAP object
	AttrSAMAccountName = "sAMAccountName"
	// AttrSAMAccountType is the SAM Account type for an LDAP object
	AttrSAMAccountType = "sAMAccountType"
	// AttrCommonName is the common name of an LDAP object, or "CN"
	AttrCommonName = "cn"
	// AttrDistinguishedName is the distinguished name of an LDAP object, or "DN"
	AttrDistinguishedName = "distinguishedName"
	// AttrDNSHostName is the DNS Host name of an LDAP object
	AttrDNSHostName = "dNSHostName" // unusual capitalization is correct
	// AttrObjectGUID is the globally unique identifier for an LDAP object
	AttrObjectGUID = "objectGUID"
	// AttrOS is the operating system of a computer object
	AttrOS = "operatingSystem"
	// AttrOSVersion is the operating system version of a computer object
	AttrOSVersion = "operatingSystemVersion"
	// AttrPrimaryGroupID is the primary group id of an LDAP object
	AttrPrimaryGroupID = "primaryGroupID"
	// AttrObjectSid is the Security Identifier of an LDAP object
	AttrObjectSid = "objectSid"
	// AttrObjectCategory is the object category of an LDAP object
	AttrObjectCategory = "objectCategory"
	// AttrObjectClass is the object class of an LDAP object
	AttrObjectClass = "objectClass"
)

// searchPageSize is desired page size for LDAP search. In Active Directory the default search size limit is 1000 entries,
// so in most cases the 1000 search page size will result in the optimal amount of requests made to
// LDAP server.
const searchPageSize = 1000

// Note: if you want to browse LDAP on the Windows machine, run ADSIEdit.msc.

// LDAPClient is a windows LDAP client.
//
// It does not automatically detect when the underlying connection
// is closed. Callers should check for trace.ConnectionProblem errors
// and provide a new client with [SetClient].
type LDAPClient struct {
	// Cfg is the LDAPConfig
	Cfg               LDAPConfig
	Logger            *slog.Logger
	mu                sync.Mutex
	client            ldap.Client
	connectionCreator func(addr string) (*ldap.Conn, error)
}

// SetConnectionCreator sets the function used for creating connections during referrals traversal
func (c *LDAPClient) SetConnectionCreator(creator func(addr string) (*ldap.Conn, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectionCreator = creator
}

// SetClient sets the underlying ldap.Client
func (c *LDAPClient) SetClient(client ldap.Client) {
	c.mu.Lock()
	if c.client != nil {
		c.client.Close()
	}
	c.client = client
	c.mu.Unlock()
}

// Close closes the underlying ldap.Client
func (c *LDAPClient) Close() {
	c.mu.Lock()
	if c.client != nil {
		c.client.Close()
	}
	c.mu.Unlock()
}

// extractReferrals gathers referrals from ldapErr
// If LDAP server can't provide the information required but has the knowledge of proper it will return error like:
// LDAP Result Code 10 "Referral": 0000202B: RefErr: DSID-0310084A
// You then have to parse content of the ber-encoded error to extract the address for the referral
func extractReferrals(ldapErr *ldap.Error) []string {
	if ldapErr == nil {
		return nil
	}

	if ldapErr.ResultCode != ldap.LDAPResultReferral {
		return nil
	}
	searchResultIndex := slices.IndexFunc(ldapErr.Packet.Children, func(packet *ber.Packet) bool {
		return packet.Description == "Search Result Done"
	})
	if searchResultIndex < 0 {
		return nil
	}
	searchResult := ldapErr.Packet.Children[searchResultIndex]

	referralsIndex := slices.IndexFunc(searchResult.Children, func(packet *ber.Packet) bool {
		return packet.Description == "Referral"
	})
	if referralsIndex < 0 {
		return nil
	}
	referrals := searchResult.Children[referralsIndex].Children

	out := make([]string, 0, len(referrals))
	for _, referral := range referrals {
		referralValue := referral.Value.(string)
		//value is in form of ldaps://my.domain.example.com/DC=my,DC=domain,DC=example,DC=com
		// we have to remove everything after the last /
		last := strings.LastIndex(referralValue, "/")
		if last >= 0 {
			referralValue = referralValue[:last]
		}
		out = append(out, referralValue)
	}

	return out
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
			// this one is especially important, because Teleport will
			// try to re-establish the connection when a ConnectionProblem
			// is detected
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
func (c *LDAPClient) ReadWithFilter(dn string, filter string, attrs []string) ([]*ldap.Entry, error) {
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
	c.mu.Lock()
	defer c.mu.Unlock()

	res, err := c.client.SearchWithPaging(req, searchPageSize)

	if err == nil {
		return res.Entries, nil
	}

	ctx := context.Background()

	var ldapErr *ldap.Error
	if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultReferral {
		referrals := extractReferrals(ldapErr)
		for i := 0; i < len(referrals); i++ {
			c.Logger.DebugContext(ctx, "Trying connection to referral", "referral", referrals[i])
			if conn, err := c.connectionCreator(referrals[i]); err == nil {
				res, err := conn.SearchWithPaging(req, searchPageSize)
				if err == nil {
					return res.Entries, nil
				} else if len(referrals) < 10 && errors.As(err, &ldapErr) {
					newReferrals := extractReferrals(ldapErr)
					referrals = append(referrals, newReferrals...)
				} else {
					c.Logger.DebugContext(ctx, "LDAP search failed", "referral", referrals[i], "error", err)
				}
			} else {
				c.Logger.DebugContext(ctx, "Can't connect to referral", "referral", referrals[i], "error", err)
			}
		}
		return nil, trace.BadParameter("no referral provided by LDAP server can execute the query, tried: %s", strings.Join(referrals, ","))
	} else if err != nil {
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
func (c *LDAPClient) Read(dn string, class string, attrs []string) ([]*ldap.Entry, error) {
	return c.ReadWithFilter(dn, fmt.Sprintf("(%s=%s)", AttrObjectClass, class), attrs)
}

// Create creates an LDAP entry at the given path, with the given class and
// attributes. Note that AD will create a bunch of attributes for each object
// class automatically and you don't need to specify all of them.
//
// You can browse LDAP on the Windows host to find the objectClass and
// attributes for similar entries using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (c *LDAPClient) Create(dn string, class string, attrs map[string][]string) error {
	req := ldap.NewAddRequest(dn, nil)
	for k, v := range attrs {
		req.Attribute(k, v)
	}
	req.Attribute("objectClass", []string{class})

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.client.Add(req); err != nil {
		return trace.Wrap(convertLDAPError(err), "error creating LDAP object %q", dn)
	}
	return nil
}

// CreateContainer creates an LDAP container entry if
// it doesn't already exist.
func (c *LDAPClient) CreateContainer(dn string) error {
	err := c.Create(dn, ClassContainer, nil)
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
func (c *LDAPClient) Update(dn string, replaceAttrs map[string][]string) error {
	req := ldap.NewModifyRequest(dn, nil)
	for k, v := range replaceAttrs {
		req.Replace(k, v)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.client.Modify(req); err != nil {
		return trace.Wrap(convertLDAPError(err), "updating %q", dn)
	}
	return nil
}

// CombineLDAPFilters joins the slice of filters
func CombineLDAPFilters(filters []string) string {
	return "(&" + strings.Join(filters, "") + ")"
}

func crlContainerDN(config LDAPConfig, caType types.CertAuthType) string {
	return fmt.Sprintf("CN=%s,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,%s", crlKeyName(caType), config.DomainDN())
}

func crlDN(clusterName string, config LDAPConfig, caType types.CertAuthType) string {
	return "CN=" + clusterName + "," + crlContainerDN(config, caType)
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
