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
	"net/mail"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/sethvargo/go-diceware/diceware"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const (
	numOfRecoveryCodes     = 3
	numWordsInRecoveryCode = 8

	// accountLockedMsg is the reason used to update a user's status locked message.
	accountLockedMsg = "user has exceeded maximum failed account recovery attempts"

	startRecoveryGenericErrMsg           = "unable to start account recovery, please try again or contact your system administrator"
	startRecoveryBadAuthnErrMsg          = "invalid username or recovery code"
	startRecoveryMaxFailedAttemptsErrMsg = "too many incorrect attempts, please try again later"

	approveRecoveryGenericErrMsg  = "unable to approve account recovery, please contact your system administrator"
	approveRecoveryBadAuthnErrMsg = "invalid username, password, or second factor"

	completeRecoveryGenericErrMsg = "unable to recover your account, please contact your system administrator"

	// MaxFailedAttemptsFromStartRecoveryErrMsg is a user friendly error message to try again later.
	// This error is defined in a variable so that the root caller can determine if an email needs to be sent.
	MaxFailedAttemptsFromStartRecoveryErrMsg = "you have reached max attempts, please try again later"

	// MaxFailedAttemptsFromApproveRecoveryErrMsg is a user friendly error message to start over.
	// This error is defined in a variable so that the root caller can determine if an email needs to be sent.
	MaxFailedAttemptsFromApproveRecoveryErrMsg = "too many incorrect attempts, please start over with a new recovery code"
)

// fakeRecoveryCodeHash is bcrypt hash for "fake-barbaz x 8".
// This is a fake hash used to mitigate timing attacks against invalid usernames or if user does
// exist but does not have recovery codes.
var fakeRecoveryCodeHash = []byte(`$2a$10$c2.h4pF9AA25lbrWo6U0D.ZmnYpFDaNzN3weNNYNC3jAkYEX9kpzu`)

// StartAccountRecovery implements AuthService.StartAccountRecovery.
func (s *Server) StartAccountRecovery(ctx context.Context, req *proto.StartAccountRecoveryRequest) (types.UserToken, error) {
	if err := s.isAccountRecoveryAllowed(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	// Only user's with email as their username can start recovery.
	if _, err := mail.ParseAddress(req.GetUsername()); err != nil {
		log.Debugf("Failed to start account recovery, user %s is not in valid email format", req.GetUsername())
		return nil, trace.AccessDenied(startRecoveryGenericErrMsg)
	}

	if err := s.verifyCodeWithRecoveryLock(ctx, req.GetUsername(), req.GetRecoveryCode()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove any other existing tokens for this user before creating a token.
	if err := s.deleteUserTokens(ctx, req.Username); err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.AccessDenied(startRecoveryGenericErrMsg)
	}

	token, err := s.createRecoveryToken(ctx, req.GetUsername(), UserTokenTypeRecoveryStart, req.GetRecoverType())
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.AccessDenied(startRecoveryGenericErrMsg)
	}

	return token, nil
}

// verifyCodeWithRecoveryLock counts number of failed attempts at providing a valid recovery code.
// After MaxAccountRecoveryAttempts, user is temporarily locked from further attempts at recovering and also
// locked from logging in. Modeled after existing function WithUserLock.
func (s *Server) verifyCodeWithRecoveryLock(ctx context.Context, username string, recoveryCode []byte) error {
	user, err := s.Identity.GetUser(username, false)
	switch {
	case trace.IsNotFound(err):
		// If user is not found, still authenticate. It should always return an error.
		// This prevents username oracles and timing attacks.
		return s.verifyRecoveryCode(ctx, username, recoveryCode)
	case err != nil:
		log.Error(trace.DebugReport(err))
		return trace.AccessDenied(startRecoveryGenericErrMsg)
	}

	status := user.GetStatus()
	if status.IsLocked && status.RecoveryAttemptLockExpires.After(s.clock.Now().UTC()) {
		log.Debugf("%v exceeds %v failed account recovery attempts, locked until %v",
			user.GetName(), defaults.MaxAccountRecoveryAttempts, apiutils.HumanTimeFormat(status.RecoveryAttemptLockExpires))
		return trace.AccessDenied(startRecoveryMaxFailedAttemptsErrMsg)
	}

	verifyCodeErr := s.verifyRecoveryCode(ctx, username, recoveryCode)
	switch {
	case trace.IsConnectionProblem(verifyCodeErr):
		return trace.Wrap(verifyCodeErr)
	case verifyCodeErr == nil:
		return nil
	}

	lockedUntil, maxedAttempts, err := s.recordFailedRecoveryAttempt(ctx, username)
	switch {
	case err != nil:
		log.Error(trace.DebugReport(err))
		return trace.Wrap(verifyCodeErr)
	case !maxedAttempts:
		return trace.Wrap(verifyCodeErr)
	}

	// Temp lock both user login and recovery attempts.
	user.SetRecoveryAttemptLockExpires(lockedUntil, accountLockedMsg)
	if err := s.Identity.UpsertUser(user); err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(verifyCodeErr)
	}

	return trace.AccessDenied(MaxFailedAttemptsFromStartRecoveryErrMsg)
}

func (s *Server) verifyRecoveryCode(ctx context.Context, user string, givenCode []byte) error {
	recovery, err := s.GetRecoveryCodes(ctx, user, true /* withSecrets */)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	hashedCodes := make([]types.RecoveryCode, numOfRecoveryCodes)
	hasRecoveryCodes := false
	if trace.IsNotFound(err) {
		log.Debugf("Account recovery codes for user %q not found, using fake hashes to mitigate timing attacks.", user)
		for i := 0; i < numOfRecoveryCodes; i++ {
			hashedCodes[i].HashedCode = fakeRecoveryCodeHash
		}
	} else {
		hasRecoveryCodes = true
		hashedCodes = recovery.GetCodes()
	}

	codeMatch := false
	for i, code := range hashedCodes {
		// Always take the time to check, but ignore the result if the code was
		// previously used or if checking against fakes.
		err := bcrypt.CompareHashAndPassword(code.HashedCode, givenCode)
		if err != nil || code.IsUsed || !hasRecoveryCodes {
			continue
		}
		codeMatch = true
		// Mark matched token as used in backend so it can't be used again.
		recovery.GetCodes()[i].IsUsed = true
		if err := s.UpsertRecoveryCodes(ctx, user, recovery); err != nil {
			log.Error(trace.DebugReport(err))
			return trace.AccessDenied(startRecoveryGenericErrMsg)
		}
		break
	}

	event := &apievents.RecoveryCodeUsed{
		Metadata: apievents.Metadata{
			Type: events.RecoveryCodeUsedEvent,
			Code: events.RecoveryCodeUseSuccessCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: user,
		},
		Status: apievents.Status{
			Success: true,
		},
	}

	if !codeMatch || !hasRecoveryCodes {
		event.Status.Success = false
		event.Metadata.Code = events.RecoveryCodeUseFailureCode
		traceErr := trace.NotFound("invalid user or user does not have recovery codes")

		if hasRecoveryCodes {
			traceErr = trace.BadParameter("recovery code did not match")
		}

		event.Status.Error = traceErr.Error()
		event.Status.UserMessage = traceErr.Error()

		if err := s.emitter.EmitAuditEvent(s.closeCtx, event); err != nil {
			log.WithFields(logrus.Fields{"user": user}).Warn("Failed to emit account recovery code used failed event.")
		}

		return trace.AccessDenied(startRecoveryBadAuthnErrMsg)
	}

	if err := s.emitter.EmitAuditEvent(s.closeCtx, event); err != nil {
		log.WithFields(logrus.Fields{"user": user}).Warn("Failed to emit account recovery code used event.")
	}

	return nil
}

// ApproveAccountRecovery implements AuthService.ApproveAccountRecovery.
func (s *Server) ApproveAccountRecovery(ctx context.Context, req *proto.ApproveAccountRecoveryRequest) (types.UserToken, error) {
	if err := s.isAccountRecoveryAllowed(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	startToken, err := s.GetUserToken(ctx, req.GetRecoveryStartTokenID())
	switch {
	case err != nil:
		return nil, trace.AccessDenied(approveRecoveryGenericErrMsg)
	case startToken.GetUser() != req.Username:
		return nil, trace.AccessDenied(approveRecoveryBadAuthnErrMsg)
	}

	if err := s.verifyUserToken(startToken, UserTokenTypeRecoveryStart); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check that correct authentication method is provided before verifying.
	switch req.GetAuthnCred().(type) {
	case *proto.ApproveAccountRecoveryRequest_Password:
		if startToken.GetUsage() == types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD {
			log.Debugf("Failed to approve account recovery, expected mfa authn response, but received password.")
			return nil, trace.AccessDenied(approveRecoveryBadAuthnErrMsg)
		}

		if err := s.verifyAuthnWithRecoveryLock(ctx, startToken, func() error {
			return s.checkPasswordWOToken(startToken.GetUser(), req.GetPassword())
		}); err != nil {
			return nil, trace.Wrap(err)
		}

	case *proto.ApproveAccountRecoveryRequest_MFAAuthenticateResponse:
		if startToken.GetUsage() == types.UserTokenUsage_USER_TOKEN_RECOVER_MFA {
			log.Debugf("Failed to approve account recovery, expected password, but received a mfa authn response.")
			return nil, trace.AccessDenied(approveRecoveryBadAuthnErrMsg)
		}

		if err := s.verifyAuthnWithRecoveryLock(ctx, startToken, func() error {
			_, err := s.validateMFAAuthResponse(ctx, startToken.GetUser(), req.GetMFAAuthenticateResponse(), s.Identity)
			return err
		}); err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.AccessDenied("unsupported authentication method")
	}

	approvedToken, err := s.createRecoveryToken(ctx, startToken.GetUser(), UserTokenTypeRecoveryApproved, startToken.GetUsage())
	if err != nil {
		return nil, trace.AccessDenied(approveRecoveryGenericErrMsg)
	}

	// Delete start token to invalidate the recovery link sent to users.
	if err := s.DeleteUserToken(ctx, startToken.GetName()); err != nil {
		log.Error(trace.DebugReport(err))
	}

	return approvedToken, nil
}

// verifyAuthnWithRecoveryLock counts number of failed attempts at providing a valid password or second factor.
// After MaxAccountRecoveryAttempts, user's account is temporarily locked from logging in, recovery attempts are reset,
// and all user's tokens are deleted. Modeled after existing function WithUserLock.
func (s *Server) verifyAuthnWithRecoveryLock(ctx context.Context, startToken types.UserToken, authenticateFn func() error) error {
	// Determine user exists first since an existence of token
	// does not guarantee the user defined in token exists anymore.
	user, err := s.Identity.GetUser(startToken.GetUser(), false)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.AccessDenied(approveRecoveryGenericErrMsg)
	}

	// The error returned from authenticateFn does not guarantee sensitive info is not leaked.
	// So we will return an obscured message to user when there are errors, while logging out real error.
	verifyAuthnErr := authenticateFn()
	switch {
	case trace.IsConnectionProblem(verifyAuthnErr):
		log.Error(trace.DebugReport(verifyAuthnErr))
		return trace.AccessDenied(approveRecoveryBadAuthnErrMsg)

	case verifyAuthnErr == nil:
		// Reset attempt counter.
		if err := s.DeleteUserRecoveryAttempts(ctx, startToken.GetUser()); err != nil {
			log.Error(trace.DebugReport(err))
		}

		return nil
	}

	log.Error(trace.DebugReport(verifyAuthnErr))

	lockedUntil, maxedAttempts, err := s.recordFailedRecoveryAttempt(ctx, startToken.GetUser())
	switch {
	case err != nil:
		log.Error(trace.DebugReport(err))
		return trace.AccessDenied(approveRecoveryBadAuthnErrMsg)
	case !maxedAttempts:
		return trace.AccessDenied(approveRecoveryBadAuthnErrMsg)
	}

	// Delete all tokens related to this user, to force user to restart the recovery flow.
	if err := s.deleteUserTokens(ctx, startToken.GetUser()); err != nil {
		log.Error(trace.DebugReport(err))
		return trace.AccessDenied(approveRecoveryGenericErrMsg)
	}

	// Restart the attempt counter, to not block users from trying again with another recovery code.
	if err := s.DeleteUserRecoveryAttempts(ctx, startToken.GetUser()); err != nil {
		log.Error(trace.DebugReport(err))
		return trace.AccessDenied(approveRecoveryGenericErrMsg)
	}

	// Lock the user from logging in.
	user.SetLocked(lockedUntil, accountLockedMsg)
	if err := s.Identity.UpsertUser(user); err != nil {
		log.Error(trace.DebugReport(err))
		return trace.AccessDenied(approveRecoveryBadAuthnErrMsg)
	}

	return trace.AccessDenied(MaxFailedAttemptsFromApproveRecoveryErrMsg)
}

// recordFailedRecoveryAttempt creates and inserts a recovery attempt and if user has reached max failed attempts,
// returns the locked until time. The boolean determines if user reached maxed failed attempts (true) or not (false).
func (s *Server) recordFailedRecoveryAttempt(ctx context.Context, username string) (time.Time, bool, error) {
	maxedAttempts := true

	// Record and log failed attempt.
	now := s.clock.Now().UTC()
	attempt := &types.RecoveryAttempt{Time: now, Expires: now.Add(defaults.AttemptTTL)}
	if err := s.CreateUserRecoveryAttempt(ctx, username, attempt); err != nil {
		return time.Time{}, !maxedAttempts, trace.Wrap(err)
	}

	// Collect all attempts.
	attempts, err := s.Identity.GetUserRecoveryAttempts(ctx, username)
	if err != nil {
		return time.Time{}, !maxedAttempts, trace.Wrap(err)
	}

	if !types.IsMaxFailedRecoveryAttempt(defaults.MaxAccountRecoveryAttempts, attempts, now) {
		log.Debugf("%v user has less than %v failed account recovery attempts", username, defaults.MaxAccountRecoveryAttempts)
		return time.Time{}, !maxedAttempts, nil
	}

	// At this point, user has reached max attempts.
	lockUntil := s.clock.Now().UTC().Add(defaults.AccountLockInterval)
	log.Debugf("%v exceeds %v failed account recovery attempts, account locked until %v and an email has been sent",
		username, defaults.MaxAccountRecoveryAttempts, apiutils.HumanTimeFormat(lockUntil))

	return lockUntil, maxedAttempts, nil
}

// CompleteAccountRecovery implements AuthService.CompleteAccountRecovery.
func (s *Server) CompleteAccountRecovery(ctx context.Context, req *proto.CompleteAccountRecoveryRequest) error {
	if err := s.isAccountRecoveryAllowed(ctx); err != nil {
		return trace.Wrap(err)
	}

	approvedToken, err := s.GetUserToken(ctx, req.GetRecoveryApprovedTokenID())
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.AccessDenied(completeRecoveryGenericErrMsg)
	}

	if err := s.verifyUserToken(approvedToken, UserTokenTypeRecoveryApproved); err != nil {
		return trace.Wrap(err)
	}

	// Check that the correct auth credential is being recovered before setting a new one.
	switch req.GetNewAuthnCred().(type) {
	case *proto.CompleteAccountRecoveryRequest_NewPassword:
		if approvedToken.GetUsage() != types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD {
			log.Debugf("Failed to recover account, expected new password, but received %T.", req.GetNewAuthnCred())
			return trace.AccessDenied(completeRecoveryGenericErrMsg)
		}

		if err := services.VerifyPassword(req.GetNewPassword()); err != nil {
			return trace.Wrap(err)
		}

		if err := s.UpsertPassword(approvedToken.GetUser(), req.GetNewPassword()); err != nil {
			log.Error(trace.DebugReport(err))
			return trace.AccessDenied(completeRecoveryGenericErrMsg)
		}

	case *proto.CompleteAccountRecoveryRequest_NewMFAResponse:
		if approvedToken.GetUsage() != types.UserTokenUsage_USER_TOKEN_RECOVER_MFA {
			log.Debugf("Failed to recover account, expected new MFA register response, but received %T.", req.GetNewAuthnCred())
			return trace.AccessDenied(completeRecoveryGenericErrMsg)
		}

		_, err = s.verifyMFARespAndAddDevice(ctx, req.GetNewMFAResponse(), &newMFADeviceFields{
			username:      approvedToken.GetUser(),
			newDeviceName: req.GetNewDeviceName(),
			tokenID:       approvedToken.GetName(),
		})
		if err != nil {
			return trace.Wrap(err)
		}

	default:
		return trace.AccessDenied("unsupported authentication method")
	}

	// Check and remove user locks so user can immediately sign in after finishing recovering.
	user, err := s.GetUser(approvedToken.GetUser(), false /* without secrets */)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.AccessDenied(completeRecoveryGenericErrMsg)
	}

	if user.GetStatus().IsLocked {
		user.ResetLocks()
		if err := s.Identity.UpsertUser(user); err != nil {
			log.Error(trace.DebugReport(err))
			return trace.AccessDenied(completeRecoveryGenericErrMsg)
		}

		if err := s.DeleteUserLoginAttempts(approvedToken.GetUser()); err != nil {
			log.Error(trace.DebugReport(err))
			return trace.AccessDenied(completeRecoveryGenericErrMsg)
		}
	}

	return nil
}

// CreateAccountRecoveryCodes implements AuthService.CreateAccountRecoveryCodes.
func (s *Server) CreateAccountRecoveryCodes(ctx context.Context, req *proto.CreateAccountRecoveryCodesRequest) (*proto.CreateAccountRecoveryCodesResponse, error) {
	const unableToCreateCodesMsg = "unable to create new recovery codes, please contact your system administrator"

	if err := s.isAccountRecoveryAllowed(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := s.GetUserToken(ctx, req.GetTokenID())
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.AccessDenied(unableToCreateCodesMsg)
	}

	if _, err := mail.ParseAddress(token.GetUser()); err != nil {
		log.Debugf("Failed to create new recovery codes, username %q is not a valid email: %v.", token.GetUser(), err)
		return nil, trace.AccessDenied(unableToCreateCodesMsg)
	}

	if err := s.verifyUserToken(token, UserTokenTypeRecoveryApproved, UserTokenTypePrivilege); err != nil {
		return nil, trace.Wrap(err)
	}

	codes, err := s.generateAndUpsertRecoveryCodes(ctx, token.GetUser())
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.AccessDenied(unableToCreateCodesMsg)
	}

	// If used as part of the recovery flow, getting new recovery codes marks the end of the flow in the UI.
	if token.GetSubKind() == UserTokenTypeRecoveryApproved {
		if err := s.deleteUserTokens(ctx, token.GetUser()); err != nil {
			log.Error(trace.DebugReport(err))
		}
	}

	return &proto.CreateAccountRecoveryCodesResponse{
		RecoveryCodes: codes,
	}, nil
}

// GetAccountRecoveryToken implements AuthService.GetAccountRecoveryToken.
func (s *Server) GetAccountRecoveryToken(ctx context.Context, req *proto.GetAccountRecoveryTokenRequest) (types.UserToken, error) {
	token, err := s.GetUserToken(ctx, req.GetRecoveryTokenID())
	if err != nil {
		log.Error(trace.DebugReport(err))
		return nil, trace.AccessDenied("access denied")
	}

	if err := s.verifyUserToken(token, UserTokenTypeRecoveryStart, UserTokenTypeRecoveryApproved); err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
}

// GetAccountRecoveryCodes implements AuthService.GetAccountRecoveryCodes.
func (s *Server) GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*types.RecoveryCodesV1, error) {
	username, err := GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rc, err := s.GetRecoveryCodes(ctx, username, false /* without secrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rc, nil
}

func (s *Server) generateAndUpsertRecoveryCodes(ctx context.Context, username string) ([]string, error) {
	codes, err := generateRecoveryCodes()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hashedCodes := make([]types.RecoveryCode, len(codes))
	for i, token := range codes {
		hashedCode, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		hashedCodes[i].HashedCode = hashedCode
	}

	rc, err := types.NewRecoveryCodes(hashedCodes, s.GetClock().Now().UTC(), username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.UpsertRecoveryCodes(ctx, username, rc); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(s.closeCtx, &apievents.RecoveryCodeGenerate{
		Metadata: apievents.Metadata{
			Type: events.RecoveryCodeGeneratedEvent,
			Code: events.RecoveryCodesGenerateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: username,
		},
	}); err != nil {
		log.WithError(err).WithFields(logrus.Fields{"user": username}).Warn("Failed to emit recovery tokens generate event.")
	}

	return codes, nil
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
	gen, err := diceware.NewGenerator(nil /* use default word list */)
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
