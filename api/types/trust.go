/*
Copyright 2020 Gravitational, Inc.

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

package types

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

// CertAuthType specifies certificate authority type
type CertAuthType string

const (
	// HostCA identifies the key as a host certificate authority
	HostCA CertAuthType = "host"
	// UserCA identifies the key as a user certificate authority
	UserCA CertAuthType = "user"
	// DatabaseCA is a certificate authority used in database access.
	DatabaseCA CertAuthType = "db"
	// OpenSSHCA is a certificate authority used when connecting to agentless nodes.
	OpenSSHCA CertAuthType = "openssh"
	// JWTSigner identifies type of certificate authority as JWT signer. In this
	// case JWT is not a certificate authority because it does not issue
	// certificates but rather is an authority that signs tokens, however it behaves
	// much like a CA in terms of rotation and storage.
	JWTSigner CertAuthType = "jwt"
	// SAMLIDPCA identifies the certificate authority that will be used by the
	// SAML identity provider.
	SAMLIDPCA CertAuthType = "saml_idp"
	// CertAuthTypeAll is a special type that represents all CertAuthTypes.
	// DEPRECATED, DELETE IN 13.0.0. For more information see:
	// https://github.com/gravitational/teleport/issues/17493
	CertAuthTypeAll CertAuthType = "all"
	// OIDCIdPCA (OpenID Connect Identity Provider Certificate Authority) identifies
	// the certificate authority that will be used by the OIDC Identity Provider.
	// Similar to JWTSigner, it doesn't issue Certificates but signs JSON Web Tokens.
	OIDCIdPCA CertAuthType = "oidc_idp"
)

// CertAuthTypes lists all certificate authority types.
var CertAuthTypes = []CertAuthType{HostCA, UserCA, DatabaseCA, OpenSSHCA, JWTSigner, SAMLIDPCA, OIDCIdPCA}

// Check checks if certificate authority type value is correct
func (c CertAuthType) Check() error {
	for _, caType := range CertAuthTypes {
		if c == caType {
			return nil
		}
	}

	return trace.BadParameter("%q authority type is not supported", c)
}

// CertAuthID - id of certificate authority (it's type and domain name)
type CertAuthID struct {
	Type       CertAuthType `json:"type"`
	DomainName string       `json:"domain_name"`
}

func (c CertAuthID) String() string {
	return fmt.Sprintf("CA(type=%q, domain=%q)", c.Type, c.DomainName)
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
