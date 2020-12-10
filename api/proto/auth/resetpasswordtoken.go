package auth

import (
	"github.com/gravitational/teleport/api"
	"github.com/gravitational/trace"
)

const (
	// ResetPasswordTokenTypeInvite indicates invite UI flow
	ResetPasswordTokenTypeInvite = "invite"
	// ResetPasswordTokenTypePassword indicates set new password UI flow
	ResetPasswordTokenTypePassword = "password"
)

// CheckAndSetDefaults checks and sets the defaults
func (r *CreateResetPasswordTokenRequest) CheckAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("user name can't be empty")
	}
	if r.TTL < 0 {
		return trace.BadParameter("TTL can't be negative")
	}

	if r.Type == "" {
		r.Type = ResetPasswordTokenTypePassword
	}

	// We use the same mechanism to handle invites and password resets
	// as both allow setting up a new password based on auth preferences.
	// The only difference is default TTL values and URLs to web UI.
	switch r.Type {
	case ResetPasswordTokenTypeInvite:
		if r.TTL == 0 {
			r.TTL = api.SignupTokenTTL
		}

		if r.TTL > api.MaxSignupTokenTTL {
			return trace.BadParameter(
				"failed to create user invite token: maximum token TTL is %v hours",
				api.MaxSignupTokenTTL)
		}
	case ResetPasswordTokenTypePassword:
		if r.TTL == 0 {
			r.TTL = api.ChangePasswordTokenTTL
		}
		if r.TTL > api.MaxChangePasswordTokenTTL {
			return trace.BadParameter(
				"failed to create reset password token: maximum token TTL is %v hours",
				api.MaxChangePasswordTokenTTL)
		}
	default:
		return trace.BadParameter("unknown reset password token request type(%v)", r.Type)
	}

	return nil
}
