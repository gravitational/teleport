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

package ztpki

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/hiyosi/hawk"
)

type Credential = hawk.Credential

func NewClientWithHawk(server string, credentials Credential) (*ClientWithResponses, error) {
	return NewClientWithResponses(server, WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		hawkClient := &hawk.Client{
			Credential: &credentials,
			Option: &hawk.Option{
				TimeStamp: time.Now().Unix(),
				Nonce:     hex.EncodeToString(binary.NativeEndian.AppendUint64(nil, rand.Uint64())),
			},
		}

		if req.Body != nil && req.Body != http.NoBody {
			if req.GetBody == nil {
				return errors.New("missing GetBody for HAWK-authenticated request with body")
			}

			body, err := req.GetBody()
			if err != nil {
				return err
			}
			defer body.Close()

			bodyData, err := io.ReadAll(body)
			if err != nil {
				return err
			}

			hawkClient.Option.Payload = string(bodyData)
			hawkClient.Option.ContentType = req.Header.Get("Content-Type")
		}

		authorization, err := hawkClient.Header(req.Method, req.URL.String())
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", authorization)

		return nil
	}))
}
