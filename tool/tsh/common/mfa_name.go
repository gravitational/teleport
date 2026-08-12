/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth/webauthn/aaguid"
	"github.com/gravitational/teleport/lib/defaults"
)

// maxDefaultNameAttempts bounds the search for a free default device name.
const maxDefaultNameAttempts = 100

// mfaDeviceLister reads the devices a user has already registered.
type mfaDeviceLister interface {
	GetMFADevices(ctx context.Context, req *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error)
}

// defaultDeviceName names a device after the authenticator that registered it, falling back to what the
// credential is for when the authenticator did not identify its make and model. The name is made unique
// against the user's existing devices, since the server rejects duplicates.
//
// Returns an empty name when no default can be settled on. The credential exists by the time this runs,
// so nothing here is worth failing a registration over: the caller asks the user instead.
func defaultDeviceName(ctx context.Context, clt mfaDeviceLister, resp *proto.MFARegisterResponse, devType string, passwordless bool) string {
	// Generated names are within the limit already, but nothing downstream would catch one that grew
	// past it: the server rejects an oversized name rather than trimming it.
	base := strings.TrimSpace(clipToBytes(authenticatorName(resp, devType, passwordless), defaults.MFADeviceNameMaxLen))

	devicesResp, err := clt.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	if err != nil {
		logger.DebugContext(ctx, "Failed to list MFA devices, asking for a device name instead", "error", err)

		return ""
	}

	taken := make(map[string]struct{}, len(devicesResp.Devices))
	for _, dev := range devicesResp.Devices {
		taken[strings.ToLower(dev.GetName())] = struct{}{}
	}

	if _, ok := taken[strings.ToLower(base)]; !ok {
		return base
	}

	// Every credential from a given authenticator resolves to the same name, so a second one needs a
	// counter. The bound is arbitrary: a user with this many identical authenticators can name the next
	// one themselves.
	for n := 2; n <= maxDefaultNameAttempts; n++ {
		suffix := fmt.Sprintf(" (%d)", n)
		candidate := strings.TrimSpace(clipToBytes(base, defaults.MFADeviceNameMaxLen-len(suffix))) + suffix
		if _, ok := taken[strings.ToLower(candidate)]; !ok {
			return candidate
		}
	}

	return ""
}

// promptDeviceName asks the user to name the device, rejecting a blank answer.
func promptDeviceName(ctx context.Context) (string, error) {
	name, err := prompt.Input(ctx, os.Stdout, prompt.Stdin(), "Enter device name")
	if err != nil {
		return "", trace.Wrap(err)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "", trace.BadParameter("device name cannot be empty")
	}

	return name, nil
}

// authenticatorName is the make and model of the authenticator behind a registration response, or a
// generic label when it did not identify itself.
func authenticatorName(resp *proto.MFARegisterResponse, devType string, passwordless bool) string {
	// Touch ID reports no AAGUID of its own, but the user picked it by name.
	if devType == touchIDDeviceType {
		return "Touch ID"
	}

	if name, ok := aaguid.NameFromBytes(aaguidFromResponse(resp)); ok {
		return name
	}

	if passwordless {
		return "Passkey"
	}

	return "Security key"
}

// WebAuthn authenticator data is a 32-byte RP ID hash, a flags byte, a 4-byte signature counter, and
// then the attested credential data, which opens with the AAGUID.
const (
	authDataFlagsOffset  = 32
	authDataAAGUIDOffset = 37
	aaguidLen            = 16
)

// authDataFlagAttestedCredentialData (bit 6) marks the presence of attested credential data. Without
// it the bytes at authDataAAGUIDOffset are extension data rather than the AAGUID.
const authDataFlagAttestedCredentialData = 1 << 6

// aaguidFromResponse reads the AAGUID out of a registration response. Anything that leaves it
// unavailable - a TOTP response, an authenticator that withheld attestation, an attestation object we
// cannot parse - reads as an unidentified authenticator and returns nil, since the only thing riding
// on it is how the device is named.
func aaguidFromResponse(resp *proto.MFARegisterResponse) []byte {
	attObj := resp.GetWebauthn().GetResponse().GetAttestationObject()
	if len(attObj) == 0 {
		return nil
	}

	var parsed protocol.AttestationObject
	if err := webauthncbor.Unmarshal(attObj, &parsed); err != nil {
		return nil
	}

	authData := parsed.RawAuthData
	if len(authData) < authDataAAGUIDOffset+aaguidLen {
		return nil
	}

	if authData[authDataFlagsOffset]&authDataFlagAttestedCredentialData == 0 {
		return nil
	}

	return authData[authDataAAGUIDOffset : authDataAAGUIDOffset+aaguidLen]
}

// clipToBytes truncates s to at most max bytes without splitting a rune. It does not trim what the
// truncation leaves behind; callers that care do it themselves.
func clipToBytes(s string, max int) string {
	if max <= 0 {
		return ""
	}

	if len(s) <= max {
		return s
	}

	var b strings.Builder
	for _, r := range s {
		if b.Len()+utf8.RuneLen(r) > max {
			break
		}

		b.WriteRune(r)
	}

	return b.String()
}
