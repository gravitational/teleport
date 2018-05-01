/*
Copyright 2015 Gravitational, Inc.

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

package services

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

// Trust is responsible for managing certificate authorities
// Each authority is managing some domain, e.g. example.com
//
// There are two type of authorities, local and remote.
// Local authorities have both private and public keys, so they can
// sign public keys of users and hosts
//
// Remote authorities have only public keys available, so they can
// be only used to validate
type Trust interface {
	// CreateCertAuthority inserts a new certificate authority
	CreateCertAuthority(ca CertAuthority) error

	// UpsertCertAuthority updates or inserts a new certificate authority
	UpsertCertAuthority(ca CertAuthority) error

	// CompareAndSwapCertAuthority updates the cert authority value
	// if existing value matches existing parameter,
	// returns nil if succeeds, trace.CompareFailed otherwise
	CompareAndSwapCertAuthority(new, existing CertAuthority) error

	// DeleteCertAuthority deletes particular certificate authority
	DeleteCertAuthority(id CertAuthID) error

	// DeleteAllCertAuthorities deletes cert authorities of a certain type
	DeleteAllCertAuthorities(caType CertAuthType) error

	// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
	// controls if signing keys are loaded
	GetCertAuthority(id CertAuthID, loadSigningKeys bool) (CertAuthority, error)

	// GetCertAuthorities returns a list of authorities of a given type
	// loadSigningKeys controls whether signing keys should be loaded or not
	GetCertAuthorities(caType CertAuthType, loadSigningKeys bool) ([]CertAuthority, error)

	// ActivateCertAuthority moves a CertAuthority from the deactivated list to
	// the normal list.
	ActivateCertAuthority(id CertAuthID) error

	// DeactivateCertAuthority moves a CertAuthority from the normal list to
	// the deactivated list.
	DeactivateCertAuthority(id CertAuthID) error
}

const (
	// HostCA identifies the key as a host certificate authority
	HostCA CertAuthType = "host"
	// UserCA identifies the key as a user certificate authority
	UserCA CertAuthType = "user"
)

// CertAuthType specifies certificate authority type, user or host
type CertAuthType string

// Check checks if certificate authority type value is correct
func (c CertAuthType) Check() error {
	if c != HostCA && c != UserCA {
		return trace.BadParameter("'%v' authority type is not supported", c)
	}
	return nil
}

// CertAuthID - id of certificate authority (it's type and domain name)
type CertAuthID struct {
	Type       CertAuthType `json:"type"`
	DomainName string       `json:"domain_name"`
}

func (c *CertAuthID) String() string {
	return fmt.Sprintf("CA(type=%v, domain=%v)", c.Type, c.DomainName)
}

// Check returns error if any of the id parameters are bad, nil otherwise
func (c *CertAuthID) Check() error {
	if err := c.Type.Check(); err != nil {
		return trace.Wrap(err)
	}
	if strings.TrimSpace(c.DomainName) == "" {
		return trace.BadParameter("identity validation error: empty domain name")
	}
	return nil
}
