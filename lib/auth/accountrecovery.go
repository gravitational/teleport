/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net/mail"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	logutil "github.com/gravitational/teleport/lib/utils/log"
)

const (
	numOfRecoveryCodes     = 3
	numWordsInRecoveryCode = 8

	startRecoveryGenericErrMsg  = "unable to start account recovery, please try again or contact your system administrator"
	startRecoveryBadAuthnErrMsg = "invalid username or recovery code"

	verifyRecoveryGenericErrMsg  = "unable to verify account recovery, please contact your system administrator"
	verifyRecoveryBadAuthnErrMsg = "invalid username, password, or second factor"

	completeRecoveryGenericErrMsg = "unable to recover your account, please contact your system administrator"
)

// fakeRecoveryCodeHash is bcrypt hash for "fake-barbaz x 8".
// This is a fake hash used to mitigate timing attacks against invalid usernames or if user does
// exist but does not have recovery codes.
var fakeRecoveryCodeHash = []byte(`$2a$10$c2.h4pF9AA25lbrWo6U0D.ZmnYpFDaNzN3weNNYNC3jAkYEX9kpzu`)

// StartAccountRecovery implements AuthService.StartAccountRecovery.
func (a *Server) StartAccountRecovery(ctx context.Context, req *proto.StartAccountRecoveryRequest) (types.UserToken, error) {
	if err := a.isAccountRecoveryAllowed(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	// Only user's with email as their username can start recovery.
	if _, err := mail.ParseAddress(req.GetUsername()); err != nil {
		a.logger.DebugContext(
			ctx, "Failed to start account recovery, username is not in valid email format",
			"user", req.GetUsername(),
			"error", err,
		)
		return nil, trace.AccessDenied("%s", startRecoveryGenericErrMsg)
	}

	if err := a.verifyRecoveryCode(ctx, req.GetUsername(), req.GetRecoveryCode()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove any other existing tokens for this user before creating a token.
	if err := a.deleteUserTokens(ctx, req.Username); err != nil {
		a.logger.ErrorContext(
			ctx, "Failed to delete existing recovery tokens for user",
			"user", req.GetUsername(),
			"error", err,
		)
		return nil, trace.AccessDenied("%s", startRecoveryGenericErrMsg)
	}

	token, err := a.createRecoveryToken(ctx, req.GetUsername(), authclient.UserTokenTypeRecoveryStart, req.GetRecoverType())
	if err != nil {
		a.logger.ErrorContext(
			ctx, "Failed to create recovery token for user",
			"user", req.GetUsername(),
			"error", err,
		)
		return nil, trace.AccessDenied("%s", startRecoveryGenericErrMsg)
	}

	return token, nil
}

// verifyRecoveryCode validates the recovery code for the user and will unlock their account if the code is valid.
func (a *Server) verifyRecoveryCode(ctx context.Context, username string, recoveryCode []byte) (errResult error) {
	switch user, err := a.Services.GetUser(ctx, username, false); {
	case trace.IsNotFound(err):
		// In the case of not found, we still want to perform the comparison.
		// It will result in an error but this avoids timing attacks which expose account presence.
	case err != nil:
		a.logger.ErrorContext(ctx, "Failed to fetch user to verify account recovery", "error", err)
		return trace.AccessDenied("%s", startRecoveryGenericErrMsg)
	case user.GetUserType() != types.UserTypeLocal:
		return trace.AccessDenied("only local users may perform account recovery")
	}
	hasRecoveryCodes := false
	defer func() { // check for result condition in defer func and send the appropriate audit event
		event := &apievents.RecoveryCodeUsed{
			Metadata: apievents.Metadata{
				Type: events.RecoveryCodeUsedEvent,
				Code: events.RecoveryCodeUseSuccessCode,
			},
			UserMetadata: authz.ClientUserMetadataWithUser(ctx, username),
			Status: apievents.Status{
				Success: errResult == nil,
			},
		}
		if errResult == nil {
			if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
				a.logger.WarnContext(
					ctx, "Failed to emit account recovery code used event",
					"user", username,
					"error", err,
				)
			}
		} else {
			event.Metadata.Code = events.RecoveryCodeUseFailureCode
			if hasRecoveryCodes {
				event.Status.Error = "recovery code did not match"
			} else {
				event.Status.Error = "invalid user or user does not have recovery codes"
			}

			if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
				a.logger.WarnContext(
					ctx, "Failed to emit account recovery code used failed event",
					"user", username,
					"error", err,
				)
			}
		}
	}()

	recovery, err := a.GetRecoveryCodes(ctx, username, true /* withSecrets */)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	hashedCodes := make([]types.RecoveryCode, numOfRecoveryCodes)
	if trace.IsNotFound(err) {
		a.logger.DebugContext(
			ctx, "Account recovery codes not found for user, using fake hashes to mitigate timing attacks",
			"user", username,
		)
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
		err := bcrypt.CompareHashAndPassword(code.HashedCode, recoveryCode)
		if err != nil || code.IsUsed || !hasRecoveryCodes {
			continue
		}
		codeMatch = true
		// Mark matched token as used in backend, so it can't be used again.
		recovery.GetCodes()[i].IsUsed = true
		if err := a.UpsertRecoveryCodes(ctx, username, recovery); err != nil {
			a.logger.ErrorContext(ctx, "Failed to update recovery code as used", "error", err)
			return trace.AccessDenied("%s", startRecoveryGenericErrMsg)
		}
		break
	}

	if !codeMatch || !hasRecoveryCodes {
		return trace.AccessDenied("%s", startRecoveryBadAuthnErrMsg)
	}

	return nil
}

// VerifyAccountRecovery implements AuthService.VerifyAccountRecovery.
func (a *Server) VerifyAccountRecovery(ctx context.Context, req *proto.VerifyAccountRecoveryRequest) (types.UserToken, error) {
	if err := a.isAccountRecoveryAllowed(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	startToken, err := a.GetUserToken(ctx, req.GetRecoveryStartTokenID())
	switch {
	case err != nil:
		return nil, trace.AccessDenied("%s", verifyRecoveryGenericErrMsg)
	case startToken.GetUser() != req.Username:
		return nil, trace.AccessDenied("%s", verifyRecoveryBadAuthnErrMsg)
	}

	if err := a.verifyUserToken(ctx, startToken, authclient.UserTokenTypeRecoveryStart); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check that correct authentication method is provided before verifying.
	switch req.GetAuthnCred().(type) {
	case *proto.VerifyAccountRecoveryRequest_Password:
		if startToken.GetUsage() == types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD {
			a.logger.DebugContext(
				ctx,
				"Failed to verify account recovery, expected mfa authn response, but received password",
			)
			return nil, trace.AccessDenied("%s", verifyRecoveryBadAuthnErrMsg)
		}

		if err := a.verifyAuthnRecovery(ctx, startToken, func() error {
			return a.checkPasswordWOToken(ctx, startToken.GetUser(), req.GetPassword())
		}); err != nil {
			return nil, trace.Wrap(err)
		}

	case *proto.VerifyAccountRecoveryRequest_MFAAuthenticateResponse:
		if startToken.GetUsage() == types.UserTokenUsage_USER_TOKEN_RECOVER_MFA {
			a.logger.DebugContext(
				ctx,
				"Failed to verify account recovery, expected password, but received a mfa authn response",
			)
			return nil, trace.AccessDenied("%s", verifyRecoveryBadAuthnErrMsg)
		}

		if err := a.verifyAuthnRecovery(ctx, startToken, func() error {
			requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ACCOUNT_RECOVERY}
			_, err := a.ValidateMFAAuthResponse(ctx, req.GetMFAAuthenticateResponse(), startToken.GetUser(), requiredExt)
			return err
		}); err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.AccessDenied("unsupported authentication method")
	}

	approvedToken, err := a.createRecoveryToken(ctx, startToken.GetUser(), authclient.UserTokenTypeRecoveryApproved, startToken.GetUsage())
	if err != nil {
		return nil, trace.AccessDenied("%s", verifyRecoveryGenericErrMsg)
	}

	// Delete start token to invalidate the recovery link sent to users.
	if err := a.DeleteUserToken(ctx, startToken.GetName()); err != nil {
		a.logger.ErrorContext(ctx, "Failed to delete account recovery token", "error", err)
	}

	return approvedToken, nil
}

// verifyAuthnRecovery validates the recovery code (through authenticateFn).
func (a *Server) verifyAuthnRecovery(ctx context.Context, startToken types.UserToken, authenticateFn func() error) error {
	// Determine user exists first since an existence of token
	// does not guarantee the user defined in token exists anymore.
	_, err := a.Services.GetUser(ctx, startToken.GetUser(), false)
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to fetch user to verify account recovery", "error", err)
		return trace.AccessDenied("%s", verifyRecoveryGenericErrMsg)
	}

	// The error returned from authenticateFn does not guarantee sensitive info is not leaked.
	// So we will return an obscured message to user when there are errors, while logging out real error.
	verifyAuthnErr := authenticateFn()
	switch {
	case trace.IsConnectionProblem(verifyAuthnErr):
		a.logger.DebugContext(
			ctx, "Encountered connection problem when verifying account recovery",
			"error", verifyAuthnErr,
		)
		return trace.AccessDenied("%s", verifyRecoveryBadAuthnErrMsg)
	case verifyAuthnErr == nil:
		return nil
	}

	return trace.AccessDenied("%s", verifyRecoveryBadAuthnErrMsg)
}

// CompleteAccountRecovery implements AuthService.CompleteAccountRecovery.
func (a *Server) CompleteAccountRecovery(ctx context.Context, req *proto.CompleteAccountRecoveryRequest) error {
	if err := a.isAccountRecoveryAllowed(ctx); err != nil {
		return trace.Wrap(err)
	}

	approvedToken, err := a.GetUserToken(ctx, req.GetRecoveryApprovedTokenID())
	if err != nil {
		a.logger.ErrorContext(ctx, "Encountered error when fetching recovery token", "error", err)
		return trace.AccessDenied("%s", completeRecoveryGenericErrMsg)
	}

	if err := a.verifyUserToken(ctx, approvedToken, authclient.UserTokenTypeRecoveryApproved); err != nil {
		return trace.Wrap(err)
	}

	// Check that the correct auth credential is being recovered before setting a new one.
	switch req.GetNewAuthnCred().(type) {
	case *proto.CompleteAccountRecoveryRequest_NewPassword:
		if approvedToken.GetUsage() != types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD {
			a.logger.DebugContext(
				ctx, "Failed to recover account, did not receive password as expected",
				"received_type", logutil.TypeAttr(req.GetNewAuthnCred()),
			)
			return trace.AccessDenied("%s", completeRecoveryGenericErrMsg)
		}

		if err := services.VerifyPassword(req.GetNewPassword()); err != nil {
			return trace.Wrap(err)
		}

		if err := a.UpsertPassword(approvedToken.GetUser(), req.GetNewPassword()); err != nil {
			a.logger.ErrorContext(ctx, "Failed to upsert new password for user", "error", err)
			return trace.AccessDenied("%s", completeRecoveryGenericErrMsg)
		}

	case *proto.CompleteAccountRecoveryRequest_NewMFAResponse:
		if approvedToken.GetUsage() != types.UserTokenUsage_USER_TOKEN_RECOVER_MFA {
			a.logger.DebugContext(
				ctx, "Failed to recover account, did not receive MFA register response as expected",
				"received_type", logutil.TypeAttr(req.GetNewAuthnCred()),
			)
			return trace.AccessDenied("%s", completeRecoveryGenericErrMsg)
		}

		_, err = a.verifyMFARespAndAddDevice(ctx, &newMFADeviceFields{
			username:      approvedToken.GetUser(),
			newDeviceName: req.GetNewDeviceName(),
			tokenID:       approvedToken.GetName(),
			deviceResp:    req.GetNewMFAResponse(),
		})
		if err != nil {
			return trace.Wrap(err)
		}

	default:
		return trace.AccessDenied("unsupported authentication method")
	}

	// Check and remove user locks so user can immediately sign in after finishing recovering.
	user, err := a.Services.GetUser(ctx, approvedToken.GetUser(), false /* without secrets */)
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to fetch user to complete account recovery", "error", err)
		return trace.AccessDenied("%s", completeRecoveryGenericErrMsg)
	}

	if user.GetStatus().IsLocked {
		user.ResetLocks()
		_, err = a.UpsertUser(ctx, user)
		if err != nil {
			a.logger.ErrorContext(ctx, "Failed to upsert user completing account recovery", "error", err)
			return trace.AccessDenied("%s", completeRecoveryGenericErrMsg)
		}

		if err := a.DeleteUserLoginAttempts(approvedToken.GetUser()); err != nil {
			a.logger.ErrorContext(ctx, "Failed to delete user login attempts after completing account recovery", "error", err)
			return trace.AccessDenied("%s", completeRecoveryGenericErrMsg)
		}
	}

	return nil
}

// CreateAccountRecoveryCodes implements AuthService.CreateAccountRecoveryCodes.
func (a *Server) CreateAccountRecoveryCodes(ctx context.Context, req *proto.CreateAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	const unableToCreateCodesMsg = "unable to create new recovery codes, please contact your system administrator"

	if err := a.isAccountRecoveryAllowed(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := a.GetUserToken(ctx, req.GetTokenID())
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to fetch existing user recovery token", "error", err)
		return nil, trace.AccessDenied("%s", unableToCreateCodesMsg)
	}

	if _, err := mail.ParseAddress(token.GetUser()); err != nil {
		a.logger.DebugContext(ctx, "Failed to create new recovery codes, username is not a valid email", "user", token.GetUser(), "error", err)
		return nil, trace.AccessDenied("%s", unableToCreateCodesMsg)
	}

	// Verify if the user is local.
	switch user, err := a.GetUser(ctx, token.GetUser(), false /* withSecrets */); {
	case err != nil:
		// err swallowed on purpose.
		return nil, trace.AccessDenied("%s", unableToCreateCodesMsg)
	case user.GetUserType() != types.UserTypeLocal:
		return nil, trace.AccessDenied("only local users may create recovery codes")
	}

	if err := a.verifyUserToken(ctx, token, authclient.UserTokenTypeRecoveryApproved, authclient.UserTokenTypePrivilege); err != nil {
		return nil, trace.Wrap(err)
	}

	newRecovery, err := a.generateAndUpsertRecoveryCodes(ctx, token.GetUser())
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to generate and upsert new recovery codes", "error", err)
		return nil, trace.AccessDenied("%s", unableToCreateCodesMsg)
	}

	if err := a.deleteUserTokens(ctx, token.GetUser()); err != nil {
		a.logger.ErrorContext(ctx, "Failed to delete user tokens", "error", err)
	}

	return newRecovery, nil
}

// GetAccountRecoveryToken implements AuthService.GetAccountRecoveryToken.
func (a *Server) GetAccountRecoveryToken(ctx context.Context, req *proto.GetAccountRecoveryTokenRequest) (types.UserToken, error) {
	token, err := a.GetUserToken(ctx, req.GetRecoveryTokenID())
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to get user token", "error", err)
		return nil, trace.AccessDenied("access denied")
	}

	if err := a.verifyUserToken(ctx, token, authclient.UserTokenTypeRecoveryStart, authclient.UserTokenTypeRecoveryApproved); err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
}

// GetAccountRecoveryCodes implements AuthService.GetAccountRecoveryCodes.
func (a *Server) GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rc, err := a.GetRecoveryCodes(ctx, username, false /* without secrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.RecoveryCodes{
		Created: rc.Spec.Created,
	}, nil
}

func (a *Server) generateAndUpsertRecoveryCodes(ctx context.Context, username string) (*proto.RecoveryCodes, error) {
	codes, err := generateRecoveryCodes()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hashedCodes := make([]types.RecoveryCode, len(codes))
	for i, token := range codes {
		hashedCode, err := utils.BcryptFromPassword([]byte(token), a.bcryptCost())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		hashedCodes[i].HashedCode = hashedCode
	}

	rc, err := types.NewRecoveryCodes(hashedCodes, a.GetClock().Now().UTC(), username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.UpsertRecoveryCodes(ctx, username, rc); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.RecoveryCodeGenerate{
		Metadata: apievents.Metadata{
			Type: events.RecoveryCodeGeneratedEvent,
			Code: events.RecoveryCodesGenerateCode,
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, username),
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit recovery tokens generate event", "error", err, "user", username)
	}

	return &proto.RecoveryCodes{
		Codes:   codes,
		Created: rc.Spec.Created,
	}, nil
}

// isAccountRecoveryAllowed gets cluster auth configuration and check if cloud, local auth
// and second factor is allowed, which are required for account recovery.
func (a *Server) isAccountRecoveryAllowed(ctx context.Context) error {
	if !modules.GetModules().Features().RecoveryCodes {
		return trace.AccessDenied("account recovery is only available for Teleport enterprise")
	}

	authPref, err := a.GetAuthPreference(ctx)
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
	tokenList := make([]string, 0, numOfRecoveryCodes)

	for i := 0; i < numOfRecoveryCodes; i++ {
		wordIDs := make([]uint16, numWordsInRecoveryCode)
		if err := binary.Read(rand.Reader, binary.NativeEndian, wordIDs); err != nil {
			return nil, trace.Wrap(err)
		}

		words := make([]string, 0, 1+len(wordIDs))
		words = append(words, "tele")
		for _, id := range wordIDs {
			words = append(words, encodeProquint(id))
		}

		tokenList = append(tokenList, strings.Join(words, "-"))
	}

	return tokenList, nil
}

// encodeProquint returns a five-letter word based on a uint16.
// This proquint implementation is adapted from upspin.io:
// https://github.com/upspin/upspin/blob/master/key/proquint/proquint.go
// For the algorithm, see https://arxiv.org/html/0901.4016
func encodeProquint(x uint16) string {
	const consonants = "bdfghjklmnprstvz"
	const vowels = "aiou"

	cons3 := x & 0b1111
	vow2 := (x >> 4) & 0b11
	cons2 := (x >> 6) & 0b1111
	vow1 := (x >> 10) & 0b11
	cons1 := x >> 12

	return string([]byte{
		consonants[cons1],
		vowels[vow1],
		consonants[cons2],
		vowels[vow2],
		consonants[cons3],
	})
}
