/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
)

func Test_session_trackSession(t *testing.T) {
	t.Parallel()
	moderatedPolicy := &types.SessionTrackerPolicySet{
		Version: types.V3,
		Name:    "name",
		RequireSessionJoin: []*types.SessionRequirePolicy{
			{
				Name:   "Auditor oversight",
				Filter: fmt.Sprintf("contains(user.spec.roles, %q)", "test"),
				Kinds:  []string{"k8s"},
				Modes:  []string{string(types.SessionModeratorMode)},
				Count:  1,
			},
		},
	}
	nonModeratedPolicy := &types.SessionTrackerPolicySet{
		Version: types.V3,
		Name:    "name",
	}
	type args struct {
		authClient auth.ClientI
		policies   []*types.SessionTrackerPolicySet
	}
	tests := []struct {
		name      string
		args      args
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "ok with moderated session and healthy auth service",
			args: args{
				authClient: &mockSessionTrackerService{},
				policies: []*types.SessionTrackerPolicySet{
					moderatedPolicy,
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "ok with non-moderated session session and healthy auth service",
			args: args{
				authClient: &mockSessionTrackerService{},
				policies: []*types.SessionTrackerPolicySet{
					nonModeratedPolicy,
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "fail with moderated session and unhealthy auth service",
			args: args{
				authClient: &mockSessionTrackerService{
					returnErr: true,
				},
				policies: []*types.SessionTrackerPolicySet{
					moderatedPolicy,
				},
			},
			assertErr: require.Error,
		},
		{
			name: "ok with non-moderated session session and unhealthy auth service",
			args: args{
				authClient: &mockSessionTrackerService{
					returnErr: true,
				},
				policies: []*types.SessionTrackerPolicySet{
					nonModeratedPolicy,
				},
			},
			assertErr: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := &session{
				log:             logrus.New().WithField(trace.Component, "test"),
				id:              uuid.New(),
				req:             &http.Request{},
				podName:         "podName",
				accessEvaluator: auth.NewSessionAccessEvaluator(tt.args.policies, types.KubernetesSessionKind, "username"),
				ctx: authContext{
					Context: authz.Context{
						User: &types.UserV2{
							Metadata: types.Metadata{
								Name: "username",
							},
						},
					},
					teleportCluster: teleportClusterClient{
						name: "name",
					},
					kubeClusterName: "kubeClusterName",
				},
				forwarder: &Forwarder{
					cfg: ForwarderConfig{
						Clock:      clockwork.NewFakeClock(),
						AuthClient: tt.args.authClient,
					},
					ctx: context.Background(),
				},
			}
			p := &party{
				Ctx: sess.ctx,
			}
			err := sess.trackSession(p, tt.args.policies)
			tt.assertErr(t, err)
		})
	}
}

type mockSessionTrackerService struct {
	auth.ClientI
	returnErr bool
}

func (m *mockSessionTrackerService) CreateSessionTracker(ctx context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	if m.returnErr {
		return nil, trace.ConnectionProblem(nil, "mock error")
	}
	return tracker, nil
}
