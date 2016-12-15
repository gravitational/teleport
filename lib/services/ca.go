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
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
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

	// UpsertCertAuthority updates or inserts a new certificate authority
	UpsertCertAuthority(ca CertAuthority, ttl time.Duration) error

	// DeleteCertAuthority deletes particular certificate authority
	DeleteCertAuthority(id CertAuthID) error

	// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
	// controls if signing keys are loaded
	GetCertAuthority(id CertAuthID, loadSigningKeys bool) (*CertAuthority, error)

	// GetCertAuthorities returns a list of authorities of a given type
	// loadSigningKeys controls whether signing keys should be loaded or not
	GetCertAuthorities(caType CertAuthType, loadSigningKeys bool) ([]*CertAuthority, error)
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

// CertAuthority is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthority struct {
	// Type is either user or host certificate authority
	Type CertAuthType `json:"type"`
	// DomainName identifies domain name this authority serves,
	// for host authorities that means base hostname of all servers,
	// for user authorities that means organization name
	DomainName string `json:"domain_name"`
	// Checkers is a list of SSH public keys that can be used to check
	// certificate signatures
	CheckingKeys [][]byte `json:"checking_keys"`
	// SigningKeys is a list of private keys used for signing
	SigningKeys [][]byte `json:"signing_keys"`
	// AllowedLogins is a list of allowed logins for users within
	// this certificate authority
	AllowedLogins []string `json:"allowed_logins"`
	// Roles is a list of roles assumed by users signed by this CA
	Roles []string `json:"roles"`
}

// FirstSigningKey returns first signing key or returns error if it's not here
func (ca *CertAuthority) FirstSigningKey() ([]byte, error) {
	if len(ca.SigningKeys) == 0 {
		return nil, trace.NotFound("%v has no signing keys", ca.ID())
	}
	return ca.SigningKeys[0], nil
}

// ID returns id (consisting of domain name and type) that
// identifies the authority this key belongs to
func (ca *CertAuthority) ID() *CertAuthID {
	return &CertAuthID{DomainName: ca.DomainName, Type: ca.Type}
}

// Checkers returns public keys that can be used to check cert authorities
func (ca *CertAuthority) Checkers() ([]ssh.PublicKey, error) {
	out := make([]ssh.PublicKey, 0, len(ca.CheckingKeys))
	for _, keyBytes := range ca.CheckingKeys {
		key, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
		if err != nil {
			return nil, trace.Errorf("invalid authority public key (len=%d): %v", len(keyBytes), err)
		}
		out = append(out, key)
	}
	return out, nil
}

// Signers returns a list of signers that could be used to sign keys
func (ca *CertAuthority) Signers() ([]ssh.Signer, error) {
	out := make([]ssh.Signer, 0, len(ca.SigningKeys))
	for _, keyBytes := range ca.SigningKeys {
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, signer)
	}
	return out, nil
}

// Check checks if all passed parameters are valid
func (ca *CertAuthority) Check() error {
	err := ca.ID().Check()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = ca.Checkers()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = ca.Signers()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
