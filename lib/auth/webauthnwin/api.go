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

// Package winwebauthn is wrapper around Windows webauthn API.
// It loads system webauthn.dll and uses its methods.
// It supports API versions 1+.
// API definition: https://github.com/microsoft/webauthn/blob/master/webauthn.h
// As Windows Webauthn device can be used both Windows Hello and FIDO devices.
package webauthnwin

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

// CheckSupport is the result from a Windows webauthn support check.
type CheckSupportResult struct {
	HasCompileSupport  bool
	IsAvailable        bool
	HasPlatformUV      bool
	WebAuthnAPIVersion int
}
