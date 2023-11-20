package webauthncli

import (
	"errors"
	"github.com/flynn/u2f/u2ftoken"
)

// ErrAlreadyRegistered may be used by RunOnU2FDevices callbacks to signify that
// a certain authenticator is already registered, and thus should be removed
// from the loop.
var ErrAlreadyRegistered = errors.New("already registered")

// Token represents the actions possible using an U2F/CTAP1 token.
type Token interface {
	CheckAuthenticate(req u2ftoken.AuthenticateRequest) error
	Authenticate(req u2ftoken.AuthenticateRequest) (*u2ftoken.AuthenticateResponse, error)
	Register(req u2ftoken.RegisterRequest) ([]byte, error)
}
