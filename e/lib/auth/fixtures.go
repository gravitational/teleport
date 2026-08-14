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
