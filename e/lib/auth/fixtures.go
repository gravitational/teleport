package auth

import (
	liblicense "github.com/gravitational/license"
)

// These fixures are provided in this package so they can be imported by other
// packages that have tests that need to provide a license to a testing auth
// server.

// ValidLicense implements the License interface, for a license that is not disabled.
type ValidLicense struct{}

func (vl ValidLicense) IsDisabled() bool                { return false }
func (vl ValidLicense) GetKeyPair() *liblicense.License { return nil }

// DisabledLicense implements the License interface, for a license that is disabled
// (is expired for more than the grace period).
type DisabledLicense struct{}

func (dl DisabledLicense) IsDisabled() bool                { return true }
func (dl DisabledLicense) GetKeyPair() *liblicense.License { return nil }
