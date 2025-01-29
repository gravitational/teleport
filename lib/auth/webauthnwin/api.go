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

// Package webauthnwin is wrapper around Windows webauthn API.
// It loads system webauthn.dll and uses its methods.
// It supports API versions 1+.
// API definition: https://github.com/microsoft/webauthn/blob/master/webauthn.h
// As Windows Webauthn device can be used both Windows Hello and FIDO devices.
package webauthnwin

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// LoginOpts groups non-mandatory options for Login.
type LoginOpts struct {
	// AuthenticatorAttachment specifies the desired authenticator attachment.
	AuthenticatorAttachment AuthenticatorAttachment
}

type AuthenticatorAttachment int

const (
	AttachmentAuto AuthenticatorAttachment = iota
	AttachmentCrossPlatform
	AttachmentPlatform
)

// nativeWebauthn represents the native windows webauthn interface.
// Implementors must provide a global variable called `native`.
type nativeWebauthn interface {
	CheckSupport() CheckSupportResult
	GetAssertion(origin string, in *getAssertionRequest) (*wantypes.CredentialAssertionResponse, error)
	MakeCredential(origin string, in *makeCredentialRequest) (*wantypes.CredentialCreationResponse, error)
}

type getAssertionRequest struct {
	rpID                  *uint16
	clientData            *webauthnClientData
	jsonEncodedClientData []byte
	opts                  *webauthnAuthenticatorGetAssertionOptions
}

type makeCredentialRequest struct {
	rp                    *webauthnRPEntityInformation
	user                  *webauthnUserEntityInformation
	credParameters        *webauthnCoseCredentialParameters
	clientData            *webauthnClientData
	jsonEncodedClientData []byte
	opts                  *webauthnAuthenticatorMakeCredentialOptions
}

// Login implements Login for Windows Webauthn API.
func Login(_ context.Context, origin string, assertion *wantypes.CredentialAssertion, loginOpts *LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
	if origin == "" {
		return nil, "", trace.BadParameter("origin required")
	}
	if err := assertion.Validate(); err != nil {
		return nil, "", trace.Wrap(err)
	}

	rpid, err := utf16PtrFromString(assertion.Response.RelyingPartyID)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	cd, jsonEncodedCD, err := clientDataToCType(assertion.Response.Challenge.String(), origin, string(protocol.AssertCeremony))
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	assertOpts, err := assertOptionsToCType(assertion.Response, loginOpts)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	promptPlatform()
	resp, err := native.GetAssertion(origin, &getAssertionRequest{
		rpID:                  rpid,
		clientData:            cd,
		jsonEncodedClientData: jsonEncodedCD,
		opts:                  assertOpts,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(resp),
		},
	}, "", nil
}

// Register implements Register for Windows Webauthn API.
func Register(_ context.Context, origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, error) {
	if origin == "" {
		return nil, trace.BadParameter("origin required")
	}
	if err := cc.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	rp, err := rpToCType(cc.Response.RelyingParty)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u, err := userToCType(cc.Response.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credParam, err := credParamToCType(cc.Response.Parameters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cd, jsonEncodedCD, err := clientDataToCType(cc.Response.Challenge.String(), origin, string(protocol.CreateCeremony))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opts, err := makeCredOptionsToCType(cc.Response)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	promptPlatform()
	resp, err := native.MakeCredential(origin, &makeCredentialRequest{
		rp:                    rp,
		user:                  u,
		credParameters:        credParam,
		clientData:            cd,
		jsonEncodedClientData: jsonEncodedCD,
		opts:                  opts,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.MFARegisterResponse{
		Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: wantypes.CredentialCreationResponseToProto(resp),
		},
	}, nil
}

const defaultPromptMessage = "Using platform authenticator, follow the OS dialogs"

// promptPlatformMessage is the message shown before system prompts.
var promptPlatformMessage = struct {
	mu      sync.Mutex
	message string
}{
	message: defaultPromptMessage,
}

// PromptWriter is the writer used for prompt messages.
var PromptWriter io.Writer = os.Stderr

// SetPromptPlatformMessage assigns a new prompt platform message. The prompt
// platform message is shown by [Login] or [Register] when prompting for a
// device touch.
//
// See [ResetPromptPlatformMessage].
func SetPromptPlatformMessage(message string) {
	promptPlatformMessage.mu.Lock()
	promptPlatformMessage.message = message
	promptPlatformMessage.mu.Unlock()
}

// ResetPromptPlatformMessage resets the prompt platform message to its original
// state.
//
// See [SetPromptPlatformMessage].
func ResetPromptPlatformMessage() {
	SetPromptPlatformMessage(defaultPromptMessage)
}

func promptPlatform() {
	promptPlatformMessage.mu.Lock()
	defer promptPlatformMessage.mu.Unlock()

	if msg := promptPlatformMessage.message; msg != "" {
		fmt.Fprintln(PromptWriter, msg)
	}
}

// CheckSupport is the result from a Windows webauthn support check.
type CheckSupportResult struct {
	HasCompileSupport  bool
	IsAvailable        bool
	HasPlatformUV      bool
	WebAuthnAPIVersion int
}

// IsAvailable returns true if Windows webauthn library is available in the
// system. Typically, a series of checks is performed in an attempt to avoid
// false positives.
// See CheckSupport.
func IsAvailable() bool {
	supports := CheckSupport()
	if supports.HasCompileSupport && !supports.IsAvailable {
		slog.WarnContext(context.Background(), "Webauthn is not supported on this version of Windows, supported from version 1903")
	}

	return supports.IsAvailable
}

// CheckSupport return information whether Windows Webauthn is supported and
// information about API version.
func CheckSupport() CheckSupportResult {
	return native.CheckSupport()
}

type DiagResult struct {
	Available                           bool
	RegisterSuccessful, LoginSuccessful bool
}

// Diag runs a few diagnostic commands and returns the result.
// User interaction is required.
func Diag(ctx context.Context) (*DiagResult, error) {
	res := &DiagResult{}
	if !IsAvailable() {
		return res, nil
	}
	res.Available = true

	// Attempt registration.
	const origin = "localhost"
	cc := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: wantypes.RelyingPartyEntity{
				ID: "localhost",
				CredentialEntity: wantypes.CredentialEntity{
					Name: "test RP",
				},
			},
			User: wantypes.UserEntity{
				CredentialEntity: wantypes.CredentialEntity{
					Name: "test",
				},
				ID:          []byte("test"),
				DisplayName: "test",
			},
			Parameters: []wantypes.CredentialParameter{
				{
					Type:      protocol.PublicKeyCredentialType,
					Algorithm: webauthncose.AlgES256,
				},
				{
					Type:      protocol.PublicKeyCredentialType,
					Algorithm: webauthncose.AlgRS256,
				},
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}
	ccr, err := Register(ctx, origin, cc)
	if err != nil {
		return res, trace.Wrap(err)
	}
	res.RegisterSuccessful = true

	// Attempt login.
	assertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: cc.Response.RelyingParty.ID,
			AllowedCredentials: []wantypes.CredentialDescriptor{
				{
					Type:         protocol.PublicKeyCredentialType,
					CredentialID: ccr.GetWebauthn().GetRawId(),
				},
			},
			UserVerification: protocol.VerificationDiscouraged,
		},
	}
	if _, _, err := Login(ctx, origin, assertion, &LoginOpts{}); err != nil {
		return res, trace.Wrap(err)
	}
	res.LoginSuccessful = true

	return res, nil
}
