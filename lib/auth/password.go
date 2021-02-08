package auth

import (
	"context"
	"crypto/subtle"

	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/tstranex/u2f"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// This is bcrypt hash for password "barbaz".
var fakePasswordHash = []byte(`$2a$10$Yy.e6BmS2SrGbBDsyDLVkOANZmvjjMR890nUGSXFJHBXWzxe7T44m`)

// ChangePasswordWithTokenRequest defines a request to change user password
type ChangePasswordWithTokenRequest struct {
	// SecondFactorToken is 2nd factor token value
	SecondFactorToken string `json:"second_factor_token"`
	// TokenID is this token ID
	TokenID string `json:"token"`
	// Password is user password
	Password []byte `json:"password"`
	// U2FRegisterResponse is U2F register response
	U2FRegisterResponse u2f.RegisterResponse `json:"u2f_register_response"`
}

// ChangePasswordWithToken changes password with token
func (s *Server) ChangePasswordWithToken(ctx context.Context, req ChangePasswordWithTokenRequest) (services.WebSession, error) {
	user, err := s.changePasswordWithToken(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := s.createUserWebSession(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

// ResetPassword securely generates a new random password and assigns it to user.
// This method is used to invalidate existing user password during password
// reset process.
func (s *Server) ResetPassword(username string) (string, error) {
	user, err := s.GetUser(username, false)
	if err != nil {
		return "", trace.Wrap(err)
	}

	password, err := utils.CryptoRandomHex(defaults.ResetPasswordLength)
	if err != nil {
		return "", trace.Wrap(err)
	}

	err = s.UpsertPassword(user.GetName(), []byte(password))
	if err != nil {
		return "", trace.Wrap(err)
	}

	return password, nil
}

// ChangePassword updates users password based on the old password.
func (s *Server) ChangePassword(req services.ChangePasswordReq) error {
	// validate new password
	if err := services.VerifyPassword(req.NewPassword); err != nil {
		return trace.Wrap(err)

	}

	authPreference, err := s.GetAuthPreference()
	if err != nil {
		return trace.Wrap(err)
	}

	userID := req.User
	fn := func() error {
		secondFactor := authPreference.GetSecondFactor()
		switch secondFactor {
		case teleport.OFF:
			return s.CheckPasswordWOToken(userID, req.OldPassword)
		case teleport.OTP:
			return s.CheckPassword(userID, req.OldPassword, req.SecondFactorToken)
		case teleport.U2F:
			if req.U2FSignResponse == nil {
				return trace.BadParameter("missing U2F sign response")
			}

			return s.CheckU2FSignResponse(userID, req.U2FSignResponse)
		}

		return trace.BadParameter("unsupported second factor method: %q", secondFactor)
	}

	if err := s.WithUserLock(userID, fn); err != nil {
		return trace.Wrap(err)
	}

	if err := s.UpsertPassword(userID, req.NewPassword); err != nil {
		return trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(s.closeCtx, &events.UserPasswordChange{
		Metadata: events.Metadata{
			Type: events.UserPasswordChangeEvent,
			Code: events.UserPasswordChangeCode,
		},
		UserMetadata: events.UserMetadata{
			User: userID,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit password change event.")
	}
	return nil
}

// CheckPasswordWOToken checks just password without checking OTP tokens
// used in case of SSH authentication, when token has been validated.
func (s *Server) CheckPasswordWOToken(user string, password []byte) error {
	const errMsg = "invalid username or password"

	err := services.VerifyPassword(password)
	if err != nil {
		return trace.BadParameter(errMsg)
	}

	hash, err := s.GetPasswordHash(user)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	userFound := true
	if trace.IsNotFound(err) {
		userFound = false
		log.Debugf("Username %q not found, using fake hash to mitigate timing attacks.", user)
		hash = fakePasswordHash
	}

	if err = bcrypt.CompareHashAndPassword(hash, password); err != nil {
		log.Debugf("Password for %q does not match", user)
		return trace.BadParameter(errMsg)
	}

	// Careful! The bcrypt check above may succeed for an unknown user when the
	// provided password is "barbaz", which is what fakePasswordHash hashes to.
	if !userFound {
		return trace.BadParameter(errMsg)
	}

	return nil
}

// CheckPassword checks the password and OTP token. Called by tsh or lib/web/*.
func (s *Server) CheckPassword(user string, password []byte, otpToken string) error {
	err := s.CheckPasswordWOToken(user, password)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.CheckOTP(user, otpToken)
	return trace.Wrap(err)
}

// CheckOTP determines the type of OTP token used (for legacy HOTP support), fetches the
// appropriate type from the backend, and checks if the token is valid.
func (s *Server) CheckOTP(user string, otpToken string) error {
	var err error

	otpType, err := s.getOTPType(user)
	if err != nil {
		return trace.Wrap(err)
	}

	switch otpType {
	case teleport.HOTP:
		otp, err := s.GetHOTP(user)
		if err != nil {
			return trace.Wrap(err)
		}

		// look ahead n tokens to see if we can find a matching token
		if !otp.Scan(otpToken, defaults.HOTPFirstTokensRange) {
			return trace.BadParameter("bad one time token")
		}

		// we need to upsert the hotp state again because the
		// counter was incremented
		if err := s.UpsertHOTP(user, otp); err != nil {
			return trace.Wrap(err)
		}
	case teleport.TOTP:
		otpSecret, err := s.GetTOTP(user)
		if err != nil {
			return trace.Wrap(err)
		}

		// get the previously used token to mitigate token replay attacks
		usedToken, err := s.GetUsedTOTPToken(user)
		if err != nil {
			return trace.Wrap(err)
		}

		// we use a constant time compare function to mitigate timing attacks
		if subtle.ConstantTimeCompare([]byte(otpToken), []byte(usedToken)) == 1 {
			return trace.BadParameter("previously used totp token")
		}

		// we use totp.ValidateCustom over totp.Validate so we can use
		// a fake clock in tests to get reliable results
		valid, err := totp.ValidateCustom(otpToken, otpSecret, s.clock.Now(), totp.ValidateOpts{
			Period:    teleport.TOTPValidityPeriod,
			Skew:      teleport.TOTPSkew,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			log.Errorf("unable to validate token: %v", err)
			return trace.BadParameter("unable to validate token")
		}
		if !valid {
			return trace.BadParameter("invalid totp token")
		}

		// if we have a valid token, update the previously used token
		err = s.UpsertUsedTOTPToken(user, otpToken)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// CreateSignupU2FRegisterRequest creates U2F requests
func (s *Server) CreateSignupU2FRegisterRequest(tokenID string) (u2fRegisterRequest *u2f.RegisterRequest, e error) {
	cap, err := s.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	universalSecondFactor, err := cap.GetU2F()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = s.GetResetPasswordToken(context.TODO(), tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c, err := u2f.NewChallenge(universalSecondFactor.AppID, universalSecondFactor.Facets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.UpsertU2FRegisterChallenge(tokenID, c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request := c.RegisterRequest()
	return request, nil
}

// getOTPType returns the type of OTP token used, HOTP or TOTP.
// Deprecated: Remove this method once HOTP support has been removed from Gravity.
func (s *Server) getOTPType(user string) (string, error) {
	_, err := s.GetHOTP(user)
	if err != nil {
		if trace.IsNotFound(err) {
			return teleport.TOTP, nil
		}
		return "", trace.Wrap(err)
	}
	return teleport.HOTP, nil
}

func (s *Server) changePasswordWithToken(ctx context.Context, req ChangePasswordWithTokenRequest) (services.User, error) {
	// Get cluster configuration and check if local auth is allowed.
	clusterConfig, err := s.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !clusterConfig.GetLocalAuth() {
		return nil, trace.AccessDenied(noLocalAuth)
	}

	err = services.VerifyPassword(req.Password)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if token exists.
	token, err := s.GetResetPasswordToken(ctx, req.TokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if token.Expiry().Before(s.clock.Now().UTC()) {
		return nil, trace.BadParameter("expired token")
	}

	err = s.changeUserSecondFactor(req, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := token.GetUser()
	// Delete this token first to minimize the chances
	// of partially updated user with still valid token.
	err = s.deleteResetPasswordTokens(ctx, username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set a new password.
	err = s.UpsertPassword(username, req.Password)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := s.GetUser(username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return user, nil
}

func (s *Server) changeUserSecondFactor(req ChangePasswordWithTokenRequest, ResetPasswordToken services.ResetPasswordToken) error {
	username := ResetPasswordToken.GetUser()
	cap, err := s.GetAuthPreference()
	if err != nil {
		return trace.Wrap(err)
	}

	switch cap.GetSecondFactor() {
	case teleport.OFF:
		return nil
	case teleport.OTP, teleport.TOTP, teleport.HOTP:
		secrets, err := s.Identity.GetResetPasswordTokenSecrets(context.TODO(), req.TokenID)
		if err != nil {
			return trace.Wrap(err)
		}

		// TODO: create a separate method to validate TOTP without inserting it first
		err = s.UpsertTOTP(username, secrets.GetOTPKey())
		if err != nil {
			return trace.Wrap(err)
		}

		err = s.CheckOTP(username, req.SecondFactorToken)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	case teleport.U2F:
		_, err = cap.GetU2F()
		if err != nil {
			return trace.Wrap(err)
		}

		challenge, err := s.GetU2FRegisterChallenge(req.TokenID)
		if err != nil {
			return trace.Wrap(err)
		}

		u2fRes := req.U2FRegisterResponse
		reg, err := u2f.Register(u2fRes, *challenge, &u2f.Config{SkipAttestationVerify: true})
		if err != nil {
			// U2F is a 3rd party library and sends back a string based error. Wrap this error with a
			// trace.BadParameter error to allow the Web UI to unmarshal it correctly.
			return trace.BadParameter(err.Error())
		}

		err = s.UpsertU2FRegistration(username, reg)
		if err != nil {
			return trace.Wrap(err)
		}

		err = s.UpsertU2FRegistrationCounter(username, 0)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	return trace.BadParameter("unknown second factor type %q", cap.GetSecondFactor())
}
