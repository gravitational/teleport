// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package ztpki provides a generated OpenAPI client to use the Venafi Zero
// Touch PKI (ZTPKI) API, as well as some helpers to use the authentication
// method required by the server.
//
// Example:
//
//	clt, err := NewClientWithResponses(
//		StagingServer,
//		WithHawkAuthentication(func(context.Context, *http.Request) (Credential, error) {
//			return Credential{
//				ID:  "aabbcc",
//				Key: "abcd1234",
//			}
//		}),
//		WithHTTPClient(hc),
//	)
package ztpki

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/hiyosi/hawk"
)

const (
	ProductionServer   = "https://ztpki.venafi.com/api/v2"
	ProductionServerEU = "https://ztpki.eu.venafi.com/api/v2"

	StagingServer   = "https://ztpki-staging.venafi.com/api/v2"
	StagingServerEU = "https://ztpki-staging.eu.venafi.com/api/v2"
)

// Credential contains the Hawk ID and key to use for requests to the ZTPKI API.
type Credential = hawk.Credential

// CredentialGetter should return the Hawk credentials to use for the given
// request. The context is derived from context passed to the client method by
// the original caller.
type CredentialGetter = func(ctx context.Context, req *http.Request) (Credential, error)

// WithHawkAuthentication is a [ClientOption] to use Hawk authentication with a
// client returned by [NewClient] or [NewClientWithResponses].
func WithHawkAuthentication(getCredentials CredentialGetter) ClientOption {
	return WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		credentials, err := getCredentials(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		hawkClient := &hawk.Client{
			Credential: &credentials,
			Option: &hawk.Option{
				TimeStamp: time.Now().Unix(),
				Nonce:     hex.EncodeToString(binary.NativeEndian.AppendUint64(nil, rand.Uint64())),
			},
		}

		if req.Body != nil && req.Body != http.NoBody {
			if req.GetBody == nil {
				return trace.BadParameter("missing GetBody for HAWK-authenticated request with body")
			}

			body, err := req.GetBody()
			if err != nil {
				return trace.Wrap(err)
			}
			defer body.Close()

			bodyData, err := io.ReadAll(body)
			if err != nil {
				return trace.Wrap(err)
			}

			hawkClient.Option.Payload = string(bodyData)
			hawkClient.Option.ContentType = req.Header.Get("Content-Type")
		}

		authorization, err := hawkClient.Header(req.Method, req.URL.String())
		if err != nil {
			return trace.Wrap(err)
		}
		req.Header.Set("Authorization", authorization)

		return nil
	})
}
