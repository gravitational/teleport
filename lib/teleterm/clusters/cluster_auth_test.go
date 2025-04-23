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

package clusters

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

func TestPwdlessLoginPrompt_PromptPIN(t *testing.T) {
	stream := &mockLoginPwdlessStream{}

	// Test valid pin.
	stream.assertResp = func(res *api.LoginPasswordlessResponse) error {
		require.Equal(t, api.PasswordlessPrompt_PASSWORDLESS_PROMPT_PIN, res.Prompt)
		return nil
	}
	stream.serverReq = func() (*api.LoginPasswordlessRequest, error) {
		return &api.LoginPasswordlessRequest{Request: &api.LoginPasswordlessRequest_Pin{
			Pin: &api.LoginPasswordlessRequest_LoginPasswordlessPINResponse{
				Pin: "1234"},
		}}, nil
	}

	prompt := newPwdlessLoginPrompt(context.Background(), slog.Default(), stream)
	pin, err := prompt.PromptPIN()
	require.NoError(t, err)
	require.Equal(t, "1234", pin)

	// Test invalid pin.
	stream.serverReq = func() (*api.LoginPasswordlessRequest, error) {
		return &api.LoginPasswordlessRequest{Request: &api.LoginPasswordlessRequest_Pin{
			Pin: &api.LoginPasswordlessRequest_LoginPasswordlessPINResponse{
				Pin: ""},
		}}, nil
	}

	_, err = prompt.PromptPIN()
	require.True(t, trace.IsBadParameter(err))
}

func TestPwdlessLoginPrompt_PromptTouch(t *testing.T) {
	stream := &mockLoginPwdlessStream{}

	stream.assertResp = func(res *api.LoginPasswordlessResponse) error {
		require.Equal(t, api.PasswordlessPrompt_PASSWORDLESS_PROMPT_TAP, res.Prompt)
		return nil
	}

	prompt := newPwdlessLoginPrompt(context.Background(), slog.Default(), stream)
	ackTouch, err := prompt.PromptTouch()
	require.NoError(t, err)
	require.NoError(t, ackTouch())
}

func TestPwdlessLoginPrompt_PromptCredential(t *testing.T) {
	stream := &mockLoginPwdlessStream{}

	unsortedCreds := []*wancli.CredentialInfo{
		{User: wancli.UserInfo{Name: "foo"}}, // will select
		{User: wancli.UserInfo{Name: "bar"}},
		{User: wancli.UserInfo{Name: "ape"}},
		{User: wancli.UserInfo{Name: "llama"}},
	}

	expectedCredResponse := []*api.CredentialInfo{
		{Username: "ape"},
		{Username: "bar"},
		{Username: "foo"},
		{Username: "llama"},
	}

	// Test valid index.
	stream.assertResp = func(res *api.LoginPasswordlessResponse) error {
		require.Equal(t, api.PasswordlessPrompt_PASSWORDLESS_PROMPT_CREDENTIAL, res.Prompt)
		require.Equal(t, expectedCredResponse, res.GetCredentials())
		return nil
	}
	stream.serverReq = func() (*api.LoginPasswordlessRequest, error) {
		return &api.LoginPasswordlessRequest{Request: &api.LoginPasswordlessRequest_Credential{
			Credential: &api.LoginPasswordlessRequest_LoginPasswordlessCredentialResponse{
				Index: 2},
		}}, nil
	}

	prompt := newPwdlessLoginPrompt(context.Background(), slog.Default(), stream)
	cred, err := prompt.PromptCredential(unsortedCreds)
	require.NoError(t, err)
	require.Equal(t, "foo", cred.User.Name)

	// Test invalid index.
	stream.serverReq = func() (*api.LoginPasswordlessRequest, error) {
		return &api.LoginPasswordlessRequest{Request: &api.LoginPasswordlessRequest_Credential{
			Credential: &api.LoginPasswordlessRequest_LoginPasswordlessCredentialResponse{
				Index: 4},
		}}, nil
	}
	_, err = prompt.PromptCredential(unsortedCreds)
	require.True(t, trace.IsBadParameter(err))
}

type mockLoginPwdlessStream struct {
	grpc.ServerStream
	assertResp func(resp *api.LoginPasswordlessResponse) error
	serverReq  func() (*api.LoginPasswordlessRequest, error)
}

func (m *mockLoginPwdlessStream) Send(resp *api.LoginPasswordlessResponse) error {
	if m.assertResp != nil {
		return m.assertResp(resp)
	}
	return trace.NotImplemented("assertResp not implemented")
}

func (m *mockLoginPwdlessStream) Recv() (*api.LoginPasswordlessRequest, error) {
	if m.serverReq != nil {
		return m.serverReq()
	}
	return nil, trace.NotImplemented("serverReq not implemented")
}
