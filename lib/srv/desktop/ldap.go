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
	"fmt"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"
)

// Note: if you want to browse LDAP on the Windows machine, run ADSIEdit.msc.
type ldapClient struct {
	cfg    LDAPConfig
	client ldap.Client
}

func newLDAPClient(cfg LDAPConfig) (*ldapClient, error) {
	con, err := ldap.Dial("tcp", cfg.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(awly): should we get a CA cert for the LDAP cert validation? Active
	// Directory Certificate Services (their managed CA thingy) seems to be
	// issuing those.
	if err := con.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		con.Close()
		return nil, trace.Wrap(err)
	}
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

func (c *ldapClient) read(path ldapPath, class string, attrs []string) ([]*ldap.Entry, error) {
	dn := c.cfg.dn(path)
	req := ldap.NewSearchRequest(dn, ldap.ScopeBaseObject,
		ldap.DerefAlways,
		0,
		0,
		false,
		fmt.Sprintf("(objectClass=%s)", class),
		attrs,
		nil,
	)
	res, err := c.client.Search(req)
	if err != nil {
		return nil, trace.Wrap(err, "fetching LDAP object %q: %v", dn, err)
	}
	return res.Entries, nil
}

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

func (c *ldapClient) createContainer(path ldapPath) error {
	err := c.create(path, "container", nil)
	// Ignore the error if container already exists.
	if trace.IsAlreadyExists(err) {
		return nil
	}
	return trace.Wrap(err)
}

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
