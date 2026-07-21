// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package mfa

import (
	"encoding/base64"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
)

// MarshalAuthPrompt serializes an AuthPrompt to base64-encoded protojson.
func MarshalAuthPrompt(prompt *mfav2.AuthPrompt) (string, error) {
	data, err := protojson.Marshal(prompt)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// UnmarshalAuthPrompt deserializes an AuthPrompt from base64-encoded protojson.
func UnmarshalAuthPrompt(raw string) (*mfav2.AuthPrompt, error) {
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	prompt := &mfav2.AuthPrompt{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, prompt); err != nil {
		return nil, trace.Wrap(err)
	}

	return prompt, nil
}

// NewPrompt returns an AuthPrompt with MFAPrompt set.
func NewPrompt() *mfav2.AuthPrompt {
	prompt := &mfav2.AuthPrompt{}
	prompt.SetMfa(&mfav2.MFAPrompt{})

	return prompt
}

// MarshalPromptResponseToken wraps a JWT in MFAPromptResponseToken and serializes to base64-encoded protojson.
func MarshalPromptResponseToken(token string) (string, error) {
	resp := mfav2.MFAPromptResponse_builder{
		Token: mfav2.MFAPromptResponseToken_builder{
			Token: token,
		}.Build(),
	}.Build()

	data, err := protojson.Marshal(resp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// UnmarshalPromptResponseToken deserializes a token from base64-encoded protojson.
func UnmarshalPromptResponseToken(raw string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", trace.Wrap(err)
	}

	resp := &mfav2.MFAPromptResponse{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, resp); err != nil {
		return "", trace.Wrap(err)
	}

	if resp.GetToken() == nil {
		return "", trace.BadParameter("missing token in MFAPromptResponse")
	}

	return resp.GetToken().GetToken(), nil
}
