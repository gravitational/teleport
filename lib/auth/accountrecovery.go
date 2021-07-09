/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/trace"
	"github.com/sethvargo/go-diceware/diceware"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const (
	numOfRecoveryCodes     = 3
	numWordsInRecoveryCode = 8
	accountLockedMsg       = "user has exceeded maximum failed account recovery attempts"

	// AccountRecoveryEmailMarker is a marker in error messages to send emails to users.
	// Also serves to send email one time.
	AccountRecoveryEmailMarker = "an email will be sent notifying user"
)

// fakeRecoveryCodeHash is bcrypt hash for "fake-barbaz x 8"
var fakeRecoveryCodeHash = []byte(`$2a$10$c2.h4pF9AA25lbrWo6U0D.ZmnYpFDaNzN3weNNYNC3jAkYEX9kpzu`)

// VerifyRecoveryCode verifies a given account recovery code.
// If an existing user fails to provide a correct code some number of times, user's account is temporarily locked
// from further recovery attempts and from logging in.
//
// Returns a user token, subkind set to recovery.
func (s *Server) VerifyRecoveryCode(ctx context.Context, req *proto.VerifyRecoveryCodeRequest) (types.ResetPasswordToken, error) {
	if err := s.isAccountRecoveryAllowed(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.verifyCodeWithRecoveryLock(ctx, req.GetUsername(), req.GetRecoveryCode()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove any other existing reset tokens for this user before creating a token.
	if err := s.deleteResetPasswordTokens(ctx, req.Username); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := s.createRecoveryToken(ctx, req.GetUsername(), ResetPasswordTokenRecoveryStart, req.GetRecoverType())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s.GetResetPasswordToken(ctx, token.GetName())
}

// verifyCodeWithRecoveryLock counts number of failed attempts at providing a valid recovery code.
// After max failed attempt, user is temporarily locked from further attempts at recovering and locked from
// logging in. This functions similar to WithUserLock.
func (s *Server) verifyCodeWithRecoveryLock(ctx context.Context, username string, recoveryCode []byte) error {
	user, err := s.Identity.GetUser(username, false)
	if err != nil {
		if trace.IsNotFound(err) {
			// If user is not found, still authenticate. It should
			// always return an error. This prevents username oracles and
			// timing attacks.
			return s.verifyRecoveryCode(ctx, username, recoveryCode)
		}
		return trace.Wrap(err)
	}

	status := user.GetStatus()
	if status.IsLocked && status.RecoveryAttemptLockExpires.After(s.clock.Now().UTC()) {
		return trace.AccessDenied("%v exceeds %v failed account recovery attempts, locked until %v",
			user.GetName(), defaults.MaxRecoveryAttempts, apiutils.HumanTimeFormat(status.RecoveryAttemptLockExpires))
	}

	fnErr := s.verifyRecoveryCode(ctx, username, recoveryCode)
	if fnErr == nil {
		return nil
	}

	// Do not lock user in case if DB is flaky or down.
	if trace.IsConnectionProblem(fnErr) {
		return trace.Wrap(fnErr)
	}

	// Log failed attempt.
	now := s.clock.Now().UTC()
	attempt := types.RecoveryAttempt{Time: now, Expires: now.Add(defaults.AttemptTTL)}
	if err := s.CreateUserRecoveryAttempt(ctx, username, attempt); err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}

	attempts, err := s.Identity.GetUserRecoveryAttempts(ctx, username)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}

	if !types.LastFailedRecoveryAttempt(defaults.MaxRecoveryAttempts, attempts, now) {
		log.Debugf("%v user has less than %v failed account recovery attempts", username, defaults.MaxRecoveryAttempts)
		return trace.Wrap(fnErr)
	}

	// Reached max attempts.
	lockUntil := s.clock.Now().UTC().Add(defaults.AccountLockInterval)
	message := fmt.Sprintf("%v exceeds %v failed account recovery attempts, account locked until %v and %v",
		username, defaults.MaxRecoveryAttempts, apiutils.HumanTimeFormat(lockUntil), AccountRecoveryEmailMarker)

	log.Debug(message)

	// Temp lock both user login and recovery attempts.
	user.SetLockedFromRecoveryAttempt(lockUntil)
	user.SetLocked(lockUntil, accountLockedMsg)

	if err := s.Identity.UpsertUser(user); err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}

	return trace.AccessDenied(message)
}

func (s *Server) verifyRecoveryCode(ctx context.Context, user string, givenCode []byte) error {
	recovery, err := s.GetRecoveryCodes(ctx, user)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	var hashedCodes []types.RecoveryCode
	userFound := true
	if trace.IsNotFound(err) {
		userFound = false
		log.Debugf("Account recovery codes for user %q not found, using fake hashes to mitigate timing attacks.", user)
		hashedCodes = []types.RecoveryCode{{Value: fakeRecoveryCodeHash}, {Value: fakeRecoveryCodeHash}, {Value: fakeRecoveryCodeHash}}
	} else {
		hashedCodes = recovery.Codes
	}

	codeMatch := false
	for i, code := range hashedCodes {
		if err := bcrypt.CompareHashAndPassword(code.Value, givenCode); err == nil {
			if !code.IsUsed && userFound {
				codeMatch = true
				// Mark matched token as used in backend so it can't be used again.
				recovery.Codes[i].IsUsed = true
				if err := s.UpsertRecoveryCodes(ctx, user, *recovery); err != nil {
					log.Error(trace.DebugReport(err))
					return trace.Wrap(err)
				}
				break
			}
		}
	}

	event := &apievents.RecoveryCodeUsed{
		Metadata: apievents.Metadata{
			Type: events.RecoveryCodeUsedEvent,
			Code: events.RecoveryCodeUsedCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: user,
		},
		Status: apievents.Status{
			Success: true,
		},
	}

	if !codeMatch || !userFound {
		event.Status.Success = false
		event.Metadata.Code = events.RecoveryCodeUsedFailureCode
		traceErr := trace.NotFound("user not found")

		if userFound {
			traceErr = trace.BadParameter("recovery code did not match")
		}

		event.Status.Error = traceErr.Error()
		event.Status.UserMessage = traceErr.Error()

		if err := s.emitter.EmitAuditEvent(s.closeCtx, event); err != nil {
			log.WithFields(logrus.Fields{"user": user}).Warn("Failed to emit account recovery code used failed event.")
		}

		return trace.BadParameter("invalid user or recovery code")
	}

	if err := s.emitter.EmitAuditEvent(s.closeCtx, event); err != nil {
		log.WithFields(logrus.Fields{"user": user}).Warn("Failed to emit account recovery code used event.")
	}

	return nil
}

// AuthenticateUserWithRecoveryToken authenticates user defined in token with either password or second factor.
// When a user provides a valid auth cred, the recovery token will be deleted, and an recovery approved token will be created
// for use in next step in recovery flow.
//
// If a user fails to provide correct auth cred some number of times, the recovery token will be deleted and the user
// will have to start the recovery flow again with another recovery code. The user's account will also be locked from logging in.
func (s *Server) AuthenticateUserWithRecoveryToken(ctx context.Context, req *proto.AuthenticateUserWithRecoveryTokenRequest) (types.ResetPasswordToken, error) {
	if err := s.isAccountRecoveryAllowed(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := s.GetResetPasswordToken(ctx, req.GetTokenID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if token.Expiry().Before(s.clock.Now().UTC()) {
		return nil, trace.BadParameter("expired token")
	}

	if token.GetSubKind() != ResetPasswordTokenRecoveryStart {
		return nil, trace.BadParameter("invalid token")
	}

	// This is to verify the username for emailing when user gets locked.
	if token.GetUser() != req.Username {
		return nil, trace.BadParameter("invalid username")
	}

	// Begin authenticating user password or second factor.
	switch req.GetAuthCred().(type) {
	case *proto.AuthenticateUserWithRecoveryTokenRequest_SecondFactorToken:
		return s.authenticateUserWithRecoveryLock(ctx, token, func() error {
			_, err := s.checkOTP(token.GetUser(), req.GetSecondFactorToken())
			return err
		})

	case *proto.AuthenticateUserWithRecoveryTokenRequest_U2FSignResponse:
		return s.authenticateUserWithRecoveryLock(ctx, token, func() error {
			_, err := s.CheckU2FSignResponse(ctx, token.GetUser(), &u2f.AuthenticateChallengeResponse{
				KeyHandle:     req.GetU2FSignResponse().GetKeyHandle(),
				SignatureData: req.GetU2FSignResponse().GetSignature(),
				ClientData:    req.GetU2FSignResponse().GetClientData(),
			})

			return err
		})

	default: // password
		return s.authenticateUserWithRecoveryLock(ctx, token, func() error {
			return s.checkPasswordWOToken(token.GetUser(), req.GetPassword())
		})
	}
}

// authenticateUserWithRecoveryLock counts number of failed attempts at providing a valid password or second factor.
// After max failed attempts, user's account is temporarily locked from logging in, and the reset token is deleted.
func (s *Server) authenticateUserWithRecoveryLock(ctx context.Context, token types.ResetPasswordToken, authenticateFn func() error) (types.ResetPasswordToken, error) {
	user, err := s.Identity.GetUser(token.GetUser(), false)
	if err != nil {
		if trace.IsNotFound(err) {
			// If user is not found, still call authenticateFn. It should
			// always return an error. This prevents username oracles and
			// timing attacks.
			return nil, authenticateFn()
		}
		return nil, trace.Wrap(err)
	}

	fnErr := authenticateFn()
	if fnErr == nil {
		if err := s.DeleteUserRecoveryAttempts(ctx, token.GetUser()); err != nil {
			return nil, trace.Wrap(err)
		}

		// Return a new recovery token that has been marked approved for final step in recovery flow.
		return s.createRecoveryToken(ctx, token.GetUser(), ResetPasswordTokenRecoveryApproved, token.GetRecoverType())
	}

	// Do not lock user in case if DB is flaky or down.
	if trace.IsConnectionProblem(fnErr) {
		return nil, trace.Wrap(fnErr)
	}

	// Log failed attempt.
	now := s.clock.Now().UTC()
	attempt := types.RecoveryAttempt{Time: now, Expires: now.Add(defaults.AttemptTTL)}
	if err := s.CreateUserRecoveryAttempt(ctx, token.GetUser(), attempt); err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.Wrap(fnErr)
	}

	attempts, err := s.Identity.GetUserRecoveryAttempts(ctx, token.GetUser())
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.Wrap(fnErr)
	}

	if !types.LastFailedRecoveryAttempt(defaults.MaxRecoveryAttempts, attempts, now) {
		log.Debugf("%v user has less than %v failed account recovery attempts", token.GetUser(), defaults.MaxRecoveryAttempts)
		return nil, trace.Wrap(fnErr)
	}

	// Reached max attempts.
	lockUntil := s.clock.Now().UTC().Add(defaults.AccountLockInterval)
	message := fmt.Sprintf("%v exceeds %v failed account recovery attempts, account locked until %v and %v",
		token.GetUser(), defaults.MaxRecoveryAttempts, apiutils.HumanTimeFormat(lockUntil), AccountRecoveryEmailMarker)

	log.Debug(message)

	// Delete all token data related to this user, to force user to restart the recovery flow.
	if err := s.deleteResetPasswordTokens(ctx, token.GetUser()); err != nil {
		log.Error(trace.DebugReport(err))
	}

	// Restart the attempt counter.
	if err := s.DeleteUserRecoveryAttempts(ctx, token.GetUser()); err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetLocked(lockUntil, accountLockedMsg)
	if err := s.Identity.UpsertUser(user); err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.Wrap(fnErr)
	}

	return nil, trace.AccessDenied(message)
}

// RecoverAccountWithToken changes a user's password or adds a new second factor.
// The user token provided must be marked authenticated (approved) in order to change auth cred. When successful,
// lock is removed from user (if any) so they can login immediately.
//
// Returns new account recovery tokens.
func (s *Server) RecoverAccountWithToken(ctx context.Context, req *proto.NewUserAuthCredWithTokenRequest) (*proto.RecoverAccountWithTokenResponse, error) {
	if err := s.isAccountRecoveryAllowed(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := s.GetResetPasswordToken(ctx, req.GetTokenID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if token.Expiry().Before(s.clock.Now().UTC()) {
		return nil, trace.BadParameter("expired token")
	}

	if token.GetSubKind() != ResetPasswordTokenRecoveryApproved {
		return nil, trace.BadParameter("invalid token")
	}

	// Check that the correct auth credential is being reset
	switch token.GetRecoverType() {
	case types.RecoverType_RECOVER_PASSWORD:
		if req.GetPassword() == nil {
			return nil, trace.BadParameter("expected a new password")
		}
	case types.RecoverType_RECOVER_U2F:
		if req.GetU2FRegisterResponse() == nil {
			return nil, trace.BadParameter("expected a new u2f register response")
		}
	case types.RecoverType_RECOVER_TOTP:
		if req.GetSecondFactorToken() == "" {
			return nil, trace.BadParameter("expected a second factor token")
		}
	}

	// Set new auth cred.
	if req.GetPassword() != nil {
		// Set a new password.
		if err := s.UpsertPassword(token.GetUser(), req.Password); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Set the new second factor.
		if err := s.changeUserSecondFactor(req, token); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Delete all reset tokens.
	if err = s.deleteResetPasswordTokens(ctx, token.GetUser()); err != nil {
		return nil, trace.Wrap(err)
	}

	recoveryCodes, err := s.generateAndUpsertRecoveryCodes(ctx, token.GetUser())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check and remove user login lock so user can immediately sign in after recovering.
	user, err := s.GetUser(token.GetUser(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if user.GetStatus().IsLocked {
		user.ResetLocks()
		if err := s.Identity.UpsertUser(user); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := s.DeleteUserLoginAttempts(token.GetUser()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &proto.RecoverAccountWithTokenResponse{
		Username:      token.GetUser(),
		RecoveryCodes: recoveryCodes,
	}, nil
}

func (s *Server) generateAndUpsertRecoveryCodes(ctx context.Context, username string) ([]string, error) {
	tokens, err := generateRecoveryCodes()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hashedTokens := make([]types.RecoveryCode, len(tokens))
	for i, token := range tokens {
		hashedToken, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		hashedTokens[i].Value = hashedToken
	}

	rc, err := types.NewRecoveryCodes(hashedTokens, s.GetClock().Now().UTC())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.UpsertRecoveryCodes(ctx, username, *rc); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(s.closeCtx, &apievents.RecoveryCodeGenerate{
		Metadata: apievents.Metadata{
			Type: events.RecoveryCodeGeneratedEvent,
			Code: events.RecoveryCodesGeneratedCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: username,
		},
	}); err != nil {
		log.WithError(err).WithFields(logrus.Fields{"user": username}).Warn("Failed to emit recovery tokens generate event.")
	}

	return tokens, nil
}

// isAccountRecoveryAllowed gets cluster auth configuration and check if cloud, local auth
// and second factor is allowed, which are required for account recovery.
func (s *Server) isAccountRecoveryAllowed(ctx context.Context) error {
	if modules.GetModules().Features().Cloud == false {
		return trace.AccessDenied("account recovery is only available for enterprise cloud")
	}

	authPref, err := s.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if !authPref.GetAllowLocalAuth() {
		return trace.AccessDenied("local auth needs to be enabled")
	}

	if !authPref.IsSecondFactorEnforced() {
		return trace.AccessDenied("second factor must be enabled")
	}

	return nil
}

// generateRecoveryCodes returns an array of tokens where each token
// have 8 random words prefixed with tele and concanatenated with dashes.
func generateRecoveryCodes() ([]string, error) {
	gen, err := diceware.NewGenerator(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokenList := make([]string, numOfRecoveryCodes)
	for i := 0; i < numOfRecoveryCodes; i++ {
		list, err := gen.Generate(numWordsInRecoveryCode)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tokenList[i] = "tele-" + strings.Join(list, "-")
	}

	return tokenList, nil
}

// createRecoveryToken creates a user token for account recovery.
func (s *Server) createRecoveryToken(ctx context.Context, username, tokenType string, recoverType types.RecoverType) (types.ResetPasswordToken, error) {
	req := CreateResetPasswordTokenRequest{
		Name: username,
		Type: tokenType,
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	newToken, err := s.newResetPasswordToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marks what recover type user requested.
	newToken.SetRecoverType(recoverType)

	if _, err := s.Identity.CreateResetPasswordToken(ctx, newToken); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.ResetPasswordTokenCreate{
		Metadata: apievents.Metadata{
			Type: events.ResetPasswordTokenCreateEvent,
			Code: events.ResetPasswordTokenCreateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    req.Name,
			TTL:     req.TTL.String(),
			Expires: s.GetClock().Now().UTC().Add(req.TTL),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit create reset password token event.")
	}

	return s.GetResetPasswordToken(ctx, newToken.GetName())
}
