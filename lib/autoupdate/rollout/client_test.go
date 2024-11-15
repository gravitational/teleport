/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package rollout

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

// mockClient is a mock implementation if the Client interface for testing purposes.
// This is used to precisely check which calls are made by the reconciler during tests.
// Use newMockClient to create one from stubs. Once the test is over, you must call
// mockClient.checkIfEmpty to validate all expected calls were made.
type mockClient struct {
	getAutoUpdateConfig          *getHandler[*autoupdate.AutoUpdateConfig]
	getAutoUpdateVersion         *getHandler[*autoupdate.AutoUpdateVersion]
	getAutoUpdateAgentRollout    *getHandler[*autoupdate.AutoUpdateAgentRollout]
	createAutoUpdateAgentRollout *createUpdateHandler[*autoupdate.AutoUpdateAgentRollout]
	updateAutoUpdateAgentRollout *createUpdateHandler[*autoupdate.AutoUpdateAgentRollout]
	deleteAutoUpdateAgentRollout *deleteHandler
}

func (m mockClient) GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error) {
	return m.getAutoUpdateConfig.handle(ctx)
}

func (m mockClient) GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error) {
	return m.getAutoUpdateVersion.handle(ctx)
}

func (m mockClient) GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error) {
	return m.getAutoUpdateAgentRollout.handle(ctx)
}

func (m mockClient) CreateAutoUpdateAgentRollout(ctx context.Context, rollout *autoupdate.AutoUpdateAgentRollout) (*autoupdate.AutoUpdateAgentRollout, error) {
	return m.createAutoUpdateAgentRollout.handle(ctx, rollout)
}

func (m mockClient) UpdateAutoUpdateAgentRollout(ctx context.Context, rollout *autoupdate.AutoUpdateAgentRollout) (*autoupdate.AutoUpdateAgentRollout, error) {
	return m.updateAutoUpdateAgentRollout.handle(ctx, rollout)
}

func (m mockClient) DeleteAutoUpdateAgentRollout(ctx context.Context) error {
	return m.deleteAutoUpdateAgentRollout.handle(ctx)
}

func (m mockClient) checkIfEmpty(t *testing.T) {
	require.True(t, m.getAutoUpdateConfig.isEmpty(), "Get autoupdate_config mock not empty")
	require.True(t, m.getAutoUpdateVersion.isEmpty(), "Get autoupdate_version mock not empty")
	require.True(t, m.getAutoUpdateAgentRollout.isEmpty(), "Get autoupdate_agent_rollout mock not empty")
	require.True(t, m.createAutoUpdateAgentRollout.isEmpty(), "Create autoupdate_agent_rollout mock not empty")
	require.True(t, m.updateAutoUpdateAgentRollout.isEmpty(), "Update autoupdate_agent_rollout mock not empty")
	require.True(t, m.deleteAutoUpdateAgentRollout.isEmpty(), "Delete autoupdate_agent_rollout mock not empty")
}

func newMockClient(t *testing.T, stubs mockClientStubs) *mockClient {
	// Fail early if there's a mismatch
	require.Equal(t, len(stubs.createRolloutAnswers), len(stubs.createRolloutExpects), "invalid stubs, create validations and answers slices are not the same length")
	require.Equal(t, len(stubs.updateRolloutAnswers), len(stubs.updateRolloutExpects), "invalid stubs, update validations and answers slices are not the same length")

	return &mockClient{
		getAutoUpdateConfig:          &getHandler[*autoupdate.AutoUpdateConfig]{t, stubs.configAnswers},
		getAutoUpdateVersion:         &getHandler[*autoupdate.AutoUpdateVersion]{t, stubs.versionAnswers},
		getAutoUpdateAgentRollout:    &getHandler[*autoupdate.AutoUpdateAgentRollout]{t, stubs.rolloutAnswers},
		createAutoUpdateAgentRollout: &createUpdateHandler[*autoupdate.AutoUpdateAgentRollout]{t, stubs.createRolloutExpects, stubs.createRolloutAnswers},
		updateAutoUpdateAgentRollout: &createUpdateHandler[*autoupdate.AutoUpdateAgentRollout]{t, stubs.updateRolloutExpects, stubs.updateRolloutAnswers},
		deleteAutoUpdateAgentRollout: &deleteHandler{t, stubs.deleteRolloutAnswers},
	}
}

type mockClientStubs struct {
	configAnswers        []callAnswer[*autoupdate.AutoUpdateConfig]
	versionAnswers       []callAnswer[*autoupdate.AutoUpdateVersion]
	rolloutAnswers       []callAnswer[*autoupdate.AutoUpdateAgentRollout]
	createRolloutAnswers []callAnswer[*autoupdate.AutoUpdateAgentRollout]
	createRolloutExpects []require.ValueAssertionFunc
	updateRolloutAnswers []callAnswer[*autoupdate.AutoUpdateAgentRollout]
	updateRolloutExpects []require.ValueAssertionFunc
	deleteRolloutAnswers []error
}

type callAnswer[T any] struct {
	result T
	err    error
}

// getHandler is used in a mock client to answer get resource requests during tests.
// It takes a list of answers and errors and will return them when invoked.
// If there are no stubs left it fails the test.
type getHandler[T proto.Message] struct {
	t       *testing.T
	answers []callAnswer[T]
}

func (h *getHandler[T]) handle(_ context.Context) (T, error) {
	if len(h.answers) == 0 {
		require.Fail(h.t, "no answers left")
	}

	entry := h.answers[0]
	h.answers = h.answers[1:]

	// We need to deep copy because the reconciler might do updates in place.
	// We don't want the original resource to be edited as this would mess with other tests.
	return proto.Clone(entry.result).(T), entry.err
}

// isEmpty returns true only if all stubs were consumed
func (h *getHandler[T]) isEmpty() bool {
	return len(h.answers) == 0
}

// createUpdateHandler is used in a mock client to answer create or update resource requests during tests (any request whose arity is 2).
// It first validates the input using the provided validation function, then it returns the predefined answer and error.
// If there are no stubs left it fails the test.
type createUpdateHandler[T proto.Message] struct {
	t       *testing.T
	expect  []require.ValueAssertionFunc
	answers []callAnswer[T]
}

func (h *createUpdateHandler[T]) handle(_ context.Context, object T) (T, error) {
	if len(h.expect) == 0 {
		require.Fail(h.t, "not expecting more calls")
	}
	h.expect[0](h.t, object)
	h.expect = h.expect[1:]

	if len(h.answers) == 0 {
		require.Fail(h.t, "no answers left")
	}

	entry := h.answers[0]
	h.answers = h.answers[1:]

	// We need to deep copy because the reconciler might do updates in place.
	// We don't want the original resource to be edited as this would mess with other tests.
	return proto.Clone(entry.result).(T), entry.err
}

// isEmpty returns true only if all stubs were consumed
func (h *createUpdateHandler[T]) isEmpty() bool {
	return len(h.answers) == 0 && len(h.expect) == 0
}

// deleteHandler is used in a mock client to answer delete resource requests during tests.
// It takes a list of errors and returns them when invoked.
// If there are no stubs left it fails the test.
type deleteHandler struct {
	t       *testing.T
	answers []error
}

func (h *deleteHandler) handle(_ context.Context) error {
	if len(h.answers) == 0 {
		require.Fail(h.t, "no answers left")
	}

	entry := h.answers[0]
	h.answers = h.answers[1:]

	return entry
}

// isEmpty returns true only if all stubs were consumed
func (h *deleteHandler) isEmpty() bool {
	return len(h.answers) == 0
}
