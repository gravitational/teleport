/*
Copyright 2021 Gravitational, Inc.

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

package windows

import (
	"crypto/x509"
	"fmt"
	"strings"
	"sync"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"
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
	// InsecureSkipVerify decides whether we skip verifying with the LDAP server's CA when making the LDAPS connection.
	InsecureSkipVerify bool
	// ServerName is the name of the LDAP server for TLS.
	ServerName string
	// CA is an optional CA cert to be used for verification if InsecureSkipVerify is set to false.
	CA *x509.Certificate
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
	var sb strings.Builder
	parts := strings.Split(cfg.Domain, ".")
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
	// ComputerClass is the object class for computers in Active Directory
	ComputerClass = "computer"
	// ContainerClass is the object class for containers in Active Directory
	ContainerClass = "container"
	// GMSAClass is the object class for group managed service accounts in Active Directory.
	GMSAClass = "msDS-GroupManagedServiceAccount"

	// See: https://docs.microsoft.com/en-US/windows/security/identity-protection/access-control/security-identifiers

	// WritableDomainControllerGroupID is the windows security identifier for dcs with write permissions
	WritableDomainControllerGroupID = "516"
	// ReadOnlyDomainControllerGroupID is the windows security identifier for read only dcs
	ReadOnlyDomainControllerGroupID = "521"

	// AttrName is the name of an LDAP object
	AttrName = "name"
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
)

// Note: if you want to browse LDAP on the Windows machine, run ADSIEdit.msc.

// LDAPClient is a windows LDAP client
type LDAPClient struct {
	// Cfg is the LDAPConfig
	Cfg LDAPConfig

	mu     sync.Mutex
	client ldap.Client
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
	res, err := c.client.Search(req)
	if ldap.IsErrorWithCode(err, ldap.ErrorNetwork) {
		return nil, trace.ConnectionProblem(err, "fetching LDAP object %q", dn)
	} else if err != nil {
		return nil, trace.Wrap(err, "fetching LDAP object %q: %v", dn, err)
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
	return c.ReadWithFilter(dn, fmt.Sprintf("(objectClass=%s)", class), attrs)
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
		if ldapErr, ok := err.(*ldap.Error); ok {
			switch ldapErr.ResultCode {
			case ldap.LDAPResultEntryAlreadyExists:
				return trace.AlreadyExists("LDAP object %q already exists: %v", dn, err)
			case ldap.LDAPResultConstraintViolation:
				return trace.BadParameter("object constraint violation on %q: %v", dn, err)
			case ldap.LDAPResultInsufficientAccessRights:
				return trace.AccessDenied("insufficient permissions to create %q: %v", dn, err)
			case ldap.ErrorNetwork:
				return trace.ConnectionProblem(err, "network error creating %q", dn)
			}
		}
		return trace.Wrap(err, "error creating LDAP object %q: %v", dn, err)
	}
	return nil
}

// CreateContainer creates an LDAP container entry if
// it doesn't already exist.
func (c *LDAPClient) CreateContainer(dn string) error {
	err := c.Create(dn, ContainerClass, nil)
	// Ignore the error if container already exists.
	if trace.IsAlreadyExists(err) {
		return nil
	} else if ldap.IsErrorWithCode(err, ldap.ErrorNetwork) {
		return trace.ConnectionProblem(err, "creating %v", dn)
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

	if err := c.client.Modify(req); ldap.IsErrorWithCode(err, ldap.ErrorNetwork) {
		return trace.ConnectionProblem(err, "updating %q", dn)
	} else if err != nil {
		return trace.Wrap(err, "updating %q: %v", dn, err)
	}
	return nil
}

// CombineLDAPFilters joins the slice of filters
func CombineLDAPFilters(filters []string) string {
	return "(&" + strings.Join(filters, "") + ")"
}

func crlContainerDN(config LDAPConfig) string {
	return "CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration," + config.DomainDN()
}

func crlDN(clusterName string, config LDAPConfig) string {
	return "CN=" + clusterName + "," + crlContainerDN(config)
}
