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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
)

type mockClientStubs struct {
	configAnswers        []callAnswer[*autoupdate.AutoUpdateConfig]
	versionAnswers       []callAnswer[*autoupdate.AutoUpdateVersion]
	rolloutAnswers       []callAnswer[*autoupdate.AutoUpdateAgentRollout]
	createRolloutAnswers []callAnswer[*autoupdate.AutoUpdateAgentRollout]
	createRolloutExpects []require.ValueAssertionFunc
	updateRolloutAnswers []callAnswer[*autoupdate.AutoUpdateAgentRollout]
	updateRolloutExpects []require.ValueAssertionFunc
	deleteRolloutAnswers []error
	cmcAnswers           []callAnswer[*types.ClusterMaintenanceConfigV1]
	reportsAnswers       []callAnswer[[]*autoupdate.AutoUpdateAgentReport]
}

type callAnswer[T any] struct {
	result T
	err    error
}

func newMockClient(t *testing.T, stubs mockClientStubs) *testifyMockClient {
	require.Len(t, stubs.createRolloutAnswers, len(stubs.createRolloutExpects), "invalid stubs, create validations and answers slices are not the same length")
	require.Len(t, stubs.updateRolloutAnswers, len(stubs.updateRolloutExpects), "invalid stubs, update validations and answers slices are not the same length")

	clt := &testifyMockClient{t: t, stubs: stubs}

	for _, answer := range stubs.configAnswers {
		clt.On("GetAutoUpdateConfig", mock.Anything).Return(answer.result, answer.err).Once()
	}
	for _, answer := range stubs.versionAnswers {
		clt.On("GetAutoUpdateVersion", mock.Anything).Return(answer.result, answer.err).Once()
	}
	for _, answer := range stubs.rolloutAnswers {
		clt.On("GetAutoUpdateAgentRollout", mock.Anything).Return(answer.result, answer.err).Once()
	}
	for i, answer := range stubs.createRolloutAnswers {
		clt.On("CreateAutoUpdateAgentRollout", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			stubs.createRolloutExpects[i](t, args[1])
		}).Return(answer.result, answer.err).Once()
	}
	for i, answer := range stubs.updateRolloutAnswers {
		clt.On("UpdateAutoUpdateAgentRollout", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			stubs.updateRolloutExpects[i](t, args[1])
		}).Return(answer.result, answer.err).Once()
	}
	for _, answer := range stubs.deleteRolloutAnswers {
		clt.On("DeleteAutoUpdateAgentRollout", mock.Anything).Return(answer).Once()
	}
	for _, answer := range stubs.cmcAnswers {
		clt.On("GetClusterMaintenanceConfig", mock.Anything).Return(answer.result, answer.err).Once()
	}
	for _, answer := range stubs.reportsAnswers {
		clt.On("ListAutoUpdateAgentReports", mock.Anything, mock.Anything, mock.Anything).Return(answer.result, answer.err).Once()
	}

	return clt
}

type testifyMockClient struct {
	mock.Mock
	t     *testing.T
	stubs mockClientStubs
}

func (n *testifyMockClient) GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error) {
	args := n.Called(ctx)
	result := proto.Clone(args.Get(0).(*autoupdate.AutoUpdateConfig))
	return result.(*autoupdate.AutoUpdateConfig), args.Error(1)
}

func (n *testifyMockClient) GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error) {
	args := n.Called(ctx)
	result := proto.Clone(args.Get(0).(*autoupdate.AutoUpdateVersion))
	return result.(*autoupdate.AutoUpdateVersion), args.Error(1)
}

func (n *testifyMockClient) GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error) {
	args := n.Called(ctx)
	result := proto.Clone(args.Get(0).(*autoupdate.AutoUpdateAgentRollout))
	return result.(*autoupdate.AutoUpdateAgentRollout), args.Error(1)
}

func (n *testifyMockClient) CreateAutoUpdateAgentRollout(ctx context.Context, rollout *autoupdate.AutoUpdateAgentRollout) (*autoupdate.AutoUpdateAgentRollout, error) {
	args := n.Called(ctx, rollout)
	result := proto.Clone(args.Get(0).(*autoupdate.AutoUpdateAgentRollout))
	return result.(*autoupdate.AutoUpdateAgentRollout), args.Error(1)
}

func (n *testifyMockClient) UpdateAutoUpdateAgentRollout(ctx context.Context, rollout *autoupdate.AutoUpdateAgentRollout) (*autoupdate.AutoUpdateAgentRollout, error) {
	args := n.Called(ctx, rollout)
	result := proto.Clone(args.Get(0).(*autoupdate.AutoUpdateAgentRollout))
	return result.(*autoupdate.AutoUpdateAgentRollout), args.Error(1)
}

func (n *testifyMockClient) DeleteAutoUpdateAgentRollout(ctx context.Context) error {
	args := n.Called(ctx)
	return args.Error(0)
}

func (n *testifyMockClient) GetClusterMaintenanceConfig(ctx context.Context) (types.ClusterMaintenanceConfig, error) {
	args := n.Called(ctx)
	result := apiutils.CloneProtoMsg(args.Get(0).(*types.ClusterMaintenanceConfigV1))
	return result, args.Error(1)
}

func (n *testifyMockClient) ListAutoUpdateAgentReports(ctx context.Context, pageSize int, nextKey string) ([]*autoupdate.AutoUpdateAgentReport, string, error) {
	args := n.Called(ctx, pageSize, nextKey)
	fixture := args.Get(0).([]*autoupdate.AutoUpdateAgentReport)
	result := make([]*autoupdate.AutoUpdateAgentReport, 0, len(fixture))
	for _, report := range fixture {
		result = append(result, proto.Clone(report).(*autoupdate.AutoUpdateAgentReport))
	}
	return result, "", args.Error(1)
}

func (n *testifyMockClient) checkIfCallsWereDone(t *testing.T) {
	n.AssertNumberOfCalls(t, "GetAutoUpdateConfig", len(n.stubs.configAnswers))
	n.AssertNumberOfCalls(t, "GetAutoUpdateVersion", len(n.stubs.versionAnswers))
	n.AssertNumberOfCalls(t, "GetAutoUpdateAgentRollout", len(n.stubs.rolloutAnswers))
	n.AssertNumberOfCalls(t, "CreateAutoUpdateAgentRollout", len(n.stubs.createRolloutAnswers))
	n.AssertNumberOfCalls(t, "UpdateAutoUpdateAgentRollout", len(n.stubs.updateRolloutAnswers))
	n.AssertNumberOfCalls(t, "DeleteAutoUpdateAgentRollout", len(n.stubs.deleteRolloutAnswers))
	n.AssertNumberOfCalls(t, "GetClusterMaintenanceConfig", len(n.stubs.cmcAnswers))
	n.AssertNumberOfCalls(t, "ListAutoUpdateAgentReports", len(n.stubs.reportsAnswers))
}
