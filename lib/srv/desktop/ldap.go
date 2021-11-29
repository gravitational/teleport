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

package desktop

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"
)

// Note: if you want to browse LDAP on the Windows machine, run ADSIEdit.msc.
type ldapClient struct {
	cfg    LDAPConfig
	client ldap.Client
}

// newLDAPClient connects to an LDAP server, authenticates and returns the
// client connection. Caller must close the client after using it.
func newLDAPClient(cfg LDAPConfig) (*ldapClient, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	if !cfg.InsecureSkipVerify {
		// Get the SystemCertPool, continue with an empty pool on error
		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}

		if cfg.CA != nil {
			// Append our cert to the pool.
			rootCAs.AddCert(cfg.CA)
		}

		// Supply our cert pool to TLS config for verification.
		tlsConfig.RootCAs = rootCAs
	}

	con, err := ldap.DialURL("ldaps://"+cfg.Addr, ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(zmb3): Active Directory, theoretically, supports cert-based
	// authentication. Figure out the right certificate format and generate it
	// with Teleport CA for authn here.
	if err := con.Bind(cfg.Username, cfg.Password); err != nil {
		con.Close()
		return nil, trace.Wrap(err)
	}
	return &ldapClient{
		cfg:    cfg,
		client: con,
	}, nil
}

func (c *ldapClient) close() {
	c.client.Close()
}

// read fetches an LDAP entry at path and its children, if any. Only
// entries with the given class are returned and only with the specified
// attributes.
//
// You can browse LDAP on the Windows host to find the objectClass for a
// specific entry using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (c *ldapClient) read(path ldapPath, class string, attrs []string) ([]*ldap.Entry, error) {
	dn := c.cfg.dn(path)
	req := ldap.NewSearchRequest(
		dn,
		ldap.ScopeWholeSubtree,
		ldap.DerefAlways,
		0,     // no SizeLimit
		0,     // no TimeLimit
		false, // TypesOnly == false, we want attribute values
		fmt.Sprintf("(objectClass=%s)", class),
		attrs,
		nil, // no Controls
	)
	res, err := c.client.Search(req)
	if err != nil {
		return nil, trace.Wrap(err, "fetching LDAP object %q: %v", dn, err)
	}
	return res.Entries, nil
}

// create creates an LDAP entry at the given path, with the given class and
// attributes. Note that AD will create a bunch of attributes for each object
// class automatically and you don't need to specify all of them.
//
// You can browse LDAP on the Windows host to find the objectClass and
// attributes for similar entries using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (c *ldapClient) create(path ldapPath, class string, attrs map[string][]string) error {
	dn := c.cfg.dn(path)
	req := ldap.NewAddRequest(dn, nil)
	for k, v := range attrs {
		req.Attribute(k, v)
	}
	req.Attribute("objectClass", []string{class})

	if err := c.client.Add(req); err != nil {
		if ldapErr, ok := err.(*ldap.Error); ok {
			switch ldapErr.ResultCode {
			case ldap.LDAPResultEntryAlreadyExists:
				return trace.AlreadyExists("LDAP object %q already exists: %v", dn, err)
			case ldap.LDAPResultConstraintViolation:
				return trace.BadParameter("object constraint violation on %q: %v", dn, err)
			case ldap.LDAPResultInsufficientAccessRights:
				return trace.AccessDenied("insufficient permissions to create %q: %v", dn, err)
			}
		}
		return trace.Wrap(err, "error creating LDAP object %q: %v", dn, err)
	}
	return nil
}

// createContainer creates an LDAP container entry at the given path.
func (c *ldapClient) createContainer(path ldapPath) error {
	err := c.create(path, "container", nil)
	// Ignore the error if container already exists.
	if trace.IsAlreadyExists(err) {
		return nil
	}
	return trace.Wrap(err)
}

// update updates an LDAP entry at the given path, replacing the provided
// attributes. For each attribute in replaceAttrs, the value is completely
// replaced, not merged. If you want to modify the value of an existing
// attribute, you should read the existing value first, modify it and provide
// the final combined value in replaceAttrs.
//
// You can browse LDAP on the Windows host to find attributes of existing
// entries using ADSIEdit.msc.
func (c *ldapClient) update(path ldapPath, replaceAttrs map[string][]string) error {
	dn := c.cfg.dn(path)
	req := ldap.NewModifyRequest(dn, nil)
	for k, v := range replaceAttrs {
		req.Replace(k, v)
	}
	if err := c.client.Modify(req); err != nil {
		return trace.Wrap(err, "updating %q: %v", dn, err)
	}
	return nil
}
