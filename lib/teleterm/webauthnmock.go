//go:build webauthnmock

/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package teleterm

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
)

const (
	privateKeyEnv   = "E2E_WEBAUTHN_PRIVATE_KEY"
	credentialIDEnv = "E2E_WEBAUTHN_CREDENTIAL_ID"
)

// In builds with the webauthnmock tag, provide a WebAuthn login mock for E2E tests.
func webauthnLoginMock() (client.WebauthnLoginFunc, error) {
	privateKeyB64 := os.Getenv(privateKeyEnv)
	credentialIDB64 := os.Getenv(credentialIDEnv)

	if privateKeyB64 == "" || credentialIDB64 == "" {
		return nil, trace.BadParameter("both %s and %s must be provided in webauthnmock mode", privateKeyEnv, credentialIDEnv)
	}

	privateKeyPKCS8, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return nil, trace.Wrap(err, "decoding %s", privateKeyEnv)
	}
	credentialID, err := base64.StdEncoding.DecodeString(credentialIDB64)
	if err != nil {
		return nil, trace.Wrap(err, "decoding %s", credentialIDEnv)
	}

	keyAny, err := x509.ParsePKCS8PrivateKey(privateKeyPKCS8)
	if err != nil {
		return nil, trace.Wrap(err, "parsing %s", privateKeyEnv)
	}
	privateKey, ok := keyAny.(*ecdsa.PrivateKey)
	if !ok {
		return nil, trace.BadParameter("%s must contain an ECDSA private key, got %T", privateKeyEnv, keyAny)
	}

	device, err := mocku2f.CreateWithKeyHandle(credentialID)
	if err != nil {
		return nil, trace.Wrap(err, "creating mock WebAuthn device")
	}
	device.PrivateKey = privateKey
	device.SetUV = true

	return func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		car, err := device.SignAssertion(origin, assertion)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		return &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wantypes.CredentialAssertionResponseToProto(car),
			},
		}, "", nil
	}, nil
}
