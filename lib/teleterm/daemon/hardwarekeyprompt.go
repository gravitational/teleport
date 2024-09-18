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
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func (s *Service) TshdHardwareKeyPrompt(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt {
	return &TshdPrompter{s: s, rootClusterURI: rootClusterURI}
}

type TshdPrompter struct {
	s              *Service
	rootClusterURI uri.ResourceURI
}

func (r *TshdPrompter) Touch(ctx context.Context, hint string) error {
	if err := r.s.importantModalSemaphore.Acquire(ctx); err != nil {
		return trace.Wrap(err)
	}
	defer r.s.importantModalSemaphore.Release()
	_, err := r.s.tshdEventsClient.PromptHardwareKeyTouch(ctx, &api.PromptHardwareKeyTouchRequest{
		RootClusterUri: r.rootClusterURI.String(),
		Hint:           hint,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (r *TshdPrompter) AskPIN(ctx context.Context, question string) (string, error) {
	if err := r.s.importantModalSemaphore.Acquire(ctx); err != nil {
		return "", trace.Wrap(err)
	}
	defer r.s.importantModalSemaphore.Release()
	res, err := r.s.tshdEventsClient.PromptHardwareKeyPIN(ctx, &api.PromptHardwareKeyPINRequest{
		RootClusterUri: r.rootClusterURI.String(),
		Question:       question,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return res.Pin, nil
}

func (r *TshdPrompter) ChangePIN(ctx context.Context) (*keys.PINAndPUK, error) {
	if err := r.s.importantModalSemaphore.Acquire(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	defer r.s.importantModalSemaphore.Release()
	res, err := r.s.tshdEventsClient.PromptHardwareKeyChangePIN(ctx, &api.PromptHardwareKeyChangePINRequest{
		RootClusterUri: r.rootClusterURI.String(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &keys.PINAndPUK{
		PIN:        res.Pin,
		PUK:        res.Puk,
		ChangedPUK: res.ChangedPuk,
	}, nil
}

func (r *TshdPrompter) ConfirmSlotOverwrite(ctx context.Context, message string) (bool, error) {
	if err := r.s.importantModalSemaphore.Acquire(ctx); err != nil {
		return false, trace.Wrap(err)
	}
	defer r.s.importantModalSemaphore.Release()
	res, err := r.s.tshdEventsClient.PromptHardwareKeySlotOverwrite(ctx, &api.PromptHardwareKeySlotOverwriteRequest{
		RootClusterUri: r.rootClusterURI.String(),
	})
	if err != nil {
		return false, trace.Wrap(err)
	}
	return res.Confirmed, nil
}
