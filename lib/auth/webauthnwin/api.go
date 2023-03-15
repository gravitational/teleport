// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"os"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
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
	GetAssertion(origin string, in *getAssertionRequest) (*wanlib.CredentialAssertionResponse, error)
	MakeCredential(origin string, in *makeCredentialRequest) (*wanlib.CredentialCreationResponse, error)
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
func Login(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion, loginOpts *LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
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
			Webauthn: wanlib.CredentialAssertionResponseToProto(resp),
		},
	}, "", nil
}

// Register implements Register for Windows Webauthn API.
func Register(
	ctx context.Context,
	origin string, cc *wanlib.CredentialCreation,
) (*proto.MFARegisterResponse, error) {
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
			Webauthn: wanlib.CredentialCreationResponseToProto(resp),
		},
	}, nil
}

var (
	// PromptPlatformMessage is the message shown before Touch ID prompts.
	PromptPlatformMessage = "Using platform authenticator, follow the OS dialogs"
	// PromptWriter is the writer used for prompt messages.
	PromptWriter io.Writer = os.Stderr
)

func promptPlatform() {
	if PromptPlatformMessage != "" {
		fmt.Fprintln(PromptWriter, PromptPlatformMessage)
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
		log.Warn("Webauthn is not supported on this version of Windows, supported from version 1903")
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
func Diag(ctx context.Context, promptOut io.Writer) (*DiagResult, error) {
	res := &DiagResult{}
	if !IsAvailable() {
		return res, nil
	}
	res.Available = true

	// Attempt registration.
	const origin = "localhost"
	cc := &wanlib.CredentialCreation{
		Response: protocol.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: protocol.RelyingPartyEntity{
				ID: "localhost",
				CredentialEntity: protocol.CredentialEntity{
					Name: "test RP",
				},
			},
			User: protocol.UserEntity{
				CredentialEntity: protocol.CredentialEntity{
					Name: "test",
				},
				ID:          []byte("test"),
				DisplayName: "test",
			},
			Parameters: []protocol.CredentialParameter{
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
	assertion := &wanlib.CredentialAssertion{
		Response: protocol.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: cc.Response.RelyingParty.ID,
			AllowedCredentials: []protocol.CredentialDescriptor{
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
