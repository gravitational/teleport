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

package subcav1

import (
	"context"
	"net"
	"slices"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// EnvParams hold creation parameters for [Env].
type EnvParams struct {
	StorageEnv      *subcaenv.Env // Optional. Created using StorageParams if nil.
	StorageParams   subcaenv.EnvParams
	KeystoreManager KeystoreManager
	Authorizer      authz.Authorizer
}

// Env is a gRPC test environment for subcav1.
type Env struct {
	*subcaenv.Env // storage Env

	KeystoreManager KeystoreManager
	MockEmitter     *eventstest.MockRecorderEmitter
	SubCAClient     subcav1.SubCAServiceClient

	processCancel context.CancelFunc
	stoppers      []func()
}

// Stop stops the Env service watchers, gRPC server and client.
// Useful to simulate a premature stop. It's not necessary to routinely call
// Stop.
func (e *Env) Stop() {
	if e.processCancel != nil {
		e.processCancel()
	}
	for _, stopper := range slices.Backward(e.stoppers) {
		stopper()
	}
}

// NewEnv creates a new gRPC test environment.
func NewEnv(t *testing.T, p EnvParams) *Env {
	t.Helper()

	storageEnv := p.StorageEnv
	if storageEnv == nil {
		storageEnv = subcaenv.New(t, p.StorageParams)
	}

	env := &Env{
		Env: storageEnv,
	}
	var processCtx context.Context
	processCtx, env.processCancel = context.WithCancel(t.Context())
	t.Cleanup(env.processCancel)

	// ClusterConfigurationService and cluster name.
	ccs, err := local.NewClusterConfigurationService(env.Backend)
	require.NoError(t, err, "NewClusterConfigurationService()")
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: env.ClusterName,
		ClusterID:   "40bf199c-b468-4294-97ed-06ed085d6c25", // "Random".
	})
	require.NoError(t, err, "NewClusterName()")
	switch err := ccs.SetClusterName(cn); {
	case trace.IsAlreadyExists(err):
		// OK, multi-Auth test setup.
	default:
		require.NoError(t, err, "SetClusterName()")
	}

	env.KeystoreManager = p.KeystoreManager
	if env.KeystoreManager == nil {
		var err error
		env.KeystoreManager, err = keystore.NewManager(t.Context(),
			&servicecfg.KeystoreConfig{},
			&keystore.Options{
				ClusterName:          cn,
				AuthPreferenceGetter: ccs,
				Clock:                env.Clock,
			})
		require.NoError(t, err, "keystore.NewManager()")
	}

	env.MockEmitter = &eventstest.MockRecorderEmitter{}

	authorizer := p.Authorizer
	if authorizer == nil {
		authorizer = fakeAuthorizer{}
	}

	// subcav1.Service.
	service, err := New(ServiceParams{
		Clock:                   env.Clock,
		Logger:                  logtest.NewLogger(),
		CachedClusterNameGetter: ccs,
		CachedSubCA:             env.SubCA,
		SubCA:                   env.SubCA,
		PendingCSR:              env.SubCA,
		Trust:                   env.Trust,
		WatcherContext:          processCtx,
		WatcherSource:           local.NewEventsService(env.Backend),
		KeystoreManager:         env.KeystoreManager,
		Authorizer:              authorizer,
		Emitter:                 env.MockEmitter,
	})
	require.NoError(t, err, "New() service")

	// gRPC server.
	const bufSize = 100 // arbitrary
	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() {
		assert.NoError(t, lis.Close(), "bufconn.Listener.Close()")
	})
	s := grpc.NewServer(
		// Options below are similar to auth.GRPCServer.
		grpc.ChainStreamInterceptor(
			interceptors.GRPCServerStreamErrorInterceptor,
		),
		grpc.ChainUnaryInterceptor(
			interceptors.GRPCServerUnaryErrorInterceptor,
		),
	)
	env.stoppers = append(env.stoppers, s.Stop)
	env.stoppers = append(env.stoppers, s.GracefulStop)

	subcav1.RegisterSubCAServiceServer(s, service)

	// Start gRPC server.
	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(t, s.Serve(lis), "grpc.Server.Serve()")
	})
	t.Cleanup(func() {
		s.GracefulStop()
		s.Stop()
		wg.Wait()
	})

	// gRPC client.
	cc, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
	)
	require.NoError(t, err, "grpc.NewClient()")
	t.Cleanup(func() { _ = cc.Close() })
	env.stoppers = append(env.stoppers, func() { _ = cc.Close() })

	env.SubCAClient = subcav1.NewSubCAServiceClient(cc)

	return env
}

type fakeAuthorizer struct{}

func (fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker: fakeChecker{},
		// Pass all admin actions checks.
		AdminActionAuthState: authz.AdminActionAuthMFAVerified,
	}, nil
}

type fakeChecker struct {
	services.AccessChecker
}

func (fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string) error {
	return nil
}
