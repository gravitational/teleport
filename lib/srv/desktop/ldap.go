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
	"fmt"
	"strings"
	"sync"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"
)

// Note: if you want to browse LDAP on the Windows machine, run ADSIEdit.msc.
type ldapClient struct {
	cfg LDAPConfig

	mu     sync.Mutex
	client ldap.Client
}

func (c *ldapClient) setClient(client ldap.Client) {
	c.mu.Lock()
	if c.client != nil {
		c.client.Close()
	}
	c.client = client
	c.mu.Unlock()
}

func (c *ldapClient) close() {
	c.mu.Lock()
	if c.client != nil {
		c.client.Close()
	}
	c.mu.Unlock()
}

// readWithFilter searches the specified DN (and its children) using the specified LDAP filter.
// See https://ldap.com/ldap-filters/ for more information on LDAP filter syntax.
func (c *ldapClient) readWithFilter(dn string, filter string, attrs []string) ([]*ldap.Entry, error) {
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
	if err != nil {
		return nil, trace.Wrap(err, "fetching LDAP object %q: %v", dn, err)
	}
	return res.Entries, nil

}

// read fetches an LDAP entry at path and its children, if any. Only
// entries with the given class are returned and only with the specified
// attributes.
//
// You can browse LDAP on the Windows host to find the objectClass for a
// specific entry using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (c *ldapClient) read(dn string, class string, attrs []string) ([]*ldap.Entry, error) {
	return c.readWithFilter(dn, fmt.Sprintf("(objectClass=%s)", class), attrs)
}

// create creates an LDAP entry at the given path, with the given class and
// attributes. Note that AD will create a bunch of attributes for each object
// class automatically and you don't need to specify all of them.
//
// You can browse LDAP on the Windows host to find the objectClass and
// attributes for similar entries using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (c *ldapClient) create(dn string, class string, attrs map[string][]string) error {
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
			}
		}
		return trace.Wrap(err, "error creating LDAP object %q: %v", dn, err)
	}
	return nil
}

// createContainer creates an LDAP container entry if
// it doesn't already exist.
func (c *ldapClient) createContainer(dn string) error {
	err := c.create(dn, containerClass, nil)
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
func (c *ldapClient) update(dn string, replaceAttrs map[string][]string) error {
	req := ldap.NewModifyRequest(dn, nil)
	for k, v := range replaceAttrs {
		req.Replace(k, v)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.client.Modify(req); err != nil {
		return trace.Wrap(err, "updating %q: %v", dn, err)
	}
	return nil
}

func combineLDAPFilters(filters []string) string {
	return "(&" + strings.Join(filters, "") + ")"
}
