/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package daemon

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

// NewHardwareKeyPrompt returns a new hardware key prompt.
//
// TODO(gzdunek): Improve multi-cluster and multi-hardware keys support.
// The code in yubikey.go doesn't really support using multiple hardware keys (like one per cluster):
// 1. We don't offer a choice which key should be used on the initial login.
// 2. Keys are cached per slot, not per physical key - it's not possible to use different keys with the same slot.
//
// Additionally, using the same hardware key for two clusters is not ideal too.
// Since we cache the keys per slot, if two clusters specify the same one,
// the user will always see the prompt for the same cluster URI. For example, if you are logged into both
// cluster-a and cluster-b, the prompt will always say "Unlock hardware key to access cluster-b."
// It seems that the better option would be to have a prompt per physical key, not per cluster.
// But I will leave that for the future, it's hard to say how common these scenarios will be in Connect.
//
// Because the code in yubikey.go assumes you use a single key, we don't have any mutex here.
// (unlike other modals triggered by tshd).
// We don't expect receiving prompts from different hardware keys.
func (s *Service) NewHardwareKeyPrompt() keys.HardwareKeyPrompt {
	return &hardwareKeyPrompter{s: s}
}

type hardwareKeyPrompter struct {
	s *Service
}

// Touch prompts the user to touch the hardware key.
func (h *hardwareKeyPrompter) Touch(ctx context.Context, keyInfo keys.KeyInfo) error {
	_, err := h.s.tshdEventsClient.PromptHardwareKeyTouch(ctx, &api.PromptHardwareKeyTouchRequest{
		RootClusterUri: keyInfo.ProxyHost,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AskPIN prompts the user for a PIN.
func (h *hardwareKeyPrompter) AskPIN(ctx context.Context, requirement keys.PINPromptRequirement, keyInfo keys.KeyInfo) (string, error) {
	res, err := h.s.tshdEventsClient.PromptHardwareKeyPIN(ctx, &api.PromptHardwareKeyPINRequest{
		RootClusterUri: keyInfo.ProxyHost,
		PinOptional:    requirement == keys.PINOptional,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return res.Pin, nil
}

// ChangePIN asks for a new PIN.
// The Electron app prompt must handle default values for PIN and PUK,
// preventing the user from submitting empty/default values.
func (h *hardwareKeyPrompter) ChangePIN(ctx context.Context, keyInfo keys.KeyInfo) (*keys.PINAndPUK, error) {
	res, err := h.s.tshdEventsClient.PromptHardwareKeyPINChange(ctx, &api.PromptHardwareKeyPINChangeRequest{
		RootClusterUri: keyInfo.ProxyHost,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &keys.PINAndPUK{
		PIN:        res.Pin,
		PUK:        res.Puk,
		PUKChanged: res.PukChanged,
	}, nil
}

// ConfirmSlotOverwrite asks the user if the slot's private key and certificate can be overridden.
func (h *hardwareKeyPrompter) ConfirmSlotOverwrite(ctx context.Context, message string, keyInfo keys.KeyInfo) (bool, error) {
	res, err := h.s.tshdEventsClient.ConfirmHardwareKeySlotOverwrite(ctx, &api.ConfirmHardwareKeySlotOverwriteRequest{
		RootClusterUri: keyInfo.ProxyHost,
		Message:        message,
	})
	if err != nil {
		return false, trace.Wrap(err)
	}
	return res.Confirmed, nil
}
