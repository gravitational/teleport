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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/defaults"
)

// CertAuthType specifies certificate authority type. New variants should be
// added to CertAuthTypes and, for one major version, to NewlyAdded().
type CertAuthType string

const (
	// HostCA identifies the key as a host certificate authority
	HostCA CertAuthType = "host"
	// UserCA identifies the key as a user certificate authority
	UserCA CertAuthType = "user"
	// DatabaseCA is a certificate authority used as a server CA in database
	// access.
	DatabaseCA CertAuthType = "db"
	// DatabaseClientCA is a certificate authority used as a client CA in
	// database access.
	DatabaseClientCA CertAuthType = "db_client"
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
	// OIDCIdPCA (OpenID Connect Identity Provider Certificate Authority) identifies
	// the certificate authority that will be used by the OIDC Identity Provider.
	// Similar to JWTSigner, it doesn't issue Certificates but signs JSON Web Tokens.
	OIDCIdPCA CertAuthType = "oidc_idp"
	// SPIFFECA identifies the certificate authority that will be used by the
	// SPIFFE Workload Identity provider functionality.
	SPIFFECA CertAuthType = "spiffe"
	// OktaCA identifies the certificate authority that will be used by the
	// integration with Okta.
	OktaCA CertAuthType = "okta"
	// AWSRACA identifies the certificate authority that will be used by the
	// AWS IAM Roles Anywhere integration functionality.
	AWSRACA CertAuthType = "awsra"
	// BoundKeypairCA identifies the CA used to sign bound keypair client state
	// documents.
	BoundKeypairCA CertAuthType = "bound_keypair"
)

// CertAuthTypes lists all certificate authority types.
var CertAuthTypes = []CertAuthType{HostCA,
	UserCA,
	DatabaseCA,
	DatabaseClientCA,
	OpenSSHCA,
	JWTSigner,
	SAMLIDPCA,
	OIDCIdPCA,
	SPIFFECA,
	OktaCA,
	AWSRACA,
	BoundKeypairCA,
}

// NewlyAdded should return true for CA types that were added in the current
// major version, so that we can avoid erroring out when a potentially older
// remote server doesn't know about them.
func (c CertAuthType) NewlyAdded() bool {
	return c.addedInMajorVer() >= api.VersionMajor
}

// addedInVer return the major version in which given CA was added.
func (c CertAuthType) addedInMajorVer() int64 {
	switch c {
	case DatabaseCA:
		return 9
	case OpenSSHCA, SAMLIDPCA, OIDCIdPCA:
		return 12
	case DatabaseClientCA:
		return 15
	case SPIFFECA:
		return 15
	case OktaCA:
		return 16
	case AWSRACA, BoundKeypairCA:
		return 18
	default:
		// We don't care about other CAs added before v4.0.0
		return 4
	}
}

// IsUnsupportedAuthorityErr returns whether an error is due to an unsupported
// CertAuthType.
func IsUnsupportedAuthorityErr(err error) bool {
	return err != nil && trace.IsBadParameter(err) &&
		strings.Contains(err.Error(), authTypeNotSupported)
}

const authTypeNotSupported string = "authority type is not supported"

// Check checks if certificate authority type value is correct
func (c CertAuthType) Check() error {
	for _, caType := range CertAuthTypes {
		if c == caType {
			return nil
		}
	}

	return trace.BadParameter("%q %s", c, authTypeNotSupported)
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

type RotateRequest struct {
	// Type is a certificate authority type, if omitted, both user and host CA
	// will be rotated.
	Type CertAuthType `json:"type"`
	// GracePeriod is used to generate cert rotation schedule that defines
	// times at which different rotation phases will be applied by the auth server
	// in auto mode. It is not used in manual rotation mode.
	// If omitted, default value is set, if 0 is supplied, it is interpreted as
	// forcing rotation of all certificate authorities with no grace period,
	// all existing users and hosts will have to re-login and re-added
	// into the cluster.
	GracePeriod *time.Duration `json:"grace_period,omitempty"`
	// TargetPhase sets desired rotation phase to move to, if not set
	// will be set automatically, it is a required argument
	// for manual rotation.
	TargetPhase string `json:"target_phase,omitempty"`
	// Mode sets manual or auto rotation mode.
	Mode string `json:"mode"`
	// Schedule is an optional rotation schedule,
	// autogenerated based on GracePeriod parameter if not set.
	Schedule *RotationSchedule `json:"schedule"`
}

// CheckAndSetDefaults checks and sets default values.
func (r *RotateRequest) CheckAndSetDefaults(clock clockwork.Clock) error {
	if r.TargetPhase == "" {
		// if phase is not set, imply that the first meaningful phase
		// is set as a target phase
		r.TargetPhase = RotationPhaseInit
	}
	// if mode is not set, default to manual (as it's safer)
	if r.Mode == "" {
		r.Mode = RotationModeManual
	}

	if err := r.Type.Check(); err != nil {
		return trace.Wrap(err)
	}
	if r.GracePeriod == nil {
		period := defaults.MaxCertDuration
		r.GracePeriod = &period
	}
	if r.Schedule == nil {
		var err error
		r.Schedule, err = GenerateSchedule(clock.Now(), *r.GracePeriod)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		if err := r.Schedule.CheckAndSetDefaults(clock.Now()); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
