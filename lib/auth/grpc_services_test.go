/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package auth_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/lib/auth/authtest"
)

// TestGRPCServiceEndpointsImplemented verifies that every unary RPC method
// registered on the auth gRPC server has an actual implementation rather than
// falling through to the Unimplemented stub. This catches cases where a method
// is accidentally implemented on the wrong service (e.g., on GRPCServer instead
// of the dedicated service), or added to a proto without a corresponding
// Go implementation.
//
// The test uses [grpc.Server.GetServiceInfo] to discover all registered
// services and methods at runtime so new services/methods are automatically
// covered without manual updates.
func TestGRPCServiceEndpointsImplemented(t *testing.T) {
	t.Parallel()

	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })

	// A recovery interceptor is required because some handlers panic on
	// zero-value requests (e.g., nil proto fields). Without recovery, a
	// single panic kills the entire test process.
	srv, err := as.NewTestTLSServer(
		authtest.WithAdditionalUnaryInterceptors(recovery.UnaryServerInterceptor()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	ctx := context.Background()

	grpcServer, err := srv.TLSServer.GRPCServer().GetServer()
	require.NoError(t, err)

	// Use an admin identity so authz doesn't mask the Unimplemented error
	client, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	conn := client.GetConnection()

	// Services excluded from this check. Each entry should document
	// why the service or specific method is allowed to return Unimplemented.
	skipServices := map[string]string{
		// Enterprise-only; OSS registers a NotImplementedService stub.
		"teleport.loginrule.v1.LoginRuleService":   "enterprise-only",
		"teleport.secreports.v1.SecReportsService": "enterprise-only",

		// proto.AuthService is being wound down. Deprecated RPCs are
		// intentionally removed from GRPCServer as callers migrate to
		// dedicated v1 services (e.g., users.v1, presence.v1).
		"proto.AuthService": "deprecated; methods migrated to v1 services",
	}
	skipMethods := map[string]string{
		// Enterprise-only stubs that fall through to Unimplemented instead
		// of returning a licensing error.
		// TODO(kiosion): Should be brought in-line to return license errs instead.
		"teleport.summarizer.v1.SummarizerService/GetSummary":                             "enterprise-only",
		"teleport.summarizer.v1.SummarizerService/TestInferenceModel":                     "enterprise-only",
		"teleport.summarizer.v1.SummarizerService/CreateRetrievalModel":                   "enterprise-only",
		"teleport.summarizer.v1.SummarizerService/GetRetrievalModel":                      "enterprise-only",
		"teleport.summarizer.v1.SummarizerService/UpdateRetrievalModel":                   "enterprise-only",
		"teleport.summarizer.v1.SummarizerService/UpsertRetrievalModel":                   "enterprise-only",
		"teleport.summarizer.v1.SummarizerService/DeleteRetrievalModel":                   "enterprise-only",
		"teleport.summarizer.v1.SummarizerService/TestRetrievalModel":                     "enterprise-only",
		"teleport.workloadidentity.v1.SigstorePolicyResourceService/UpsertSigstorePolicy": "enterprise-only",

		// Alpha service; these RPCs are deliberately not yet implemented.
		// EvaluateSSHAccess is implemented; these two are not yet.
		"teleport.decision.v1alpha1.DecisionService/EvaluateSSHJoin":        "alpha; not yet implemented",
		"teleport.decision.v1alpha1.DecisionService/EvaluateDatabaseAccess": "alpha; not yet implemented",
	}

	for serviceName, serviceInfo := range grpcServer.GetServiceInfo() {
		if reason, ok := skipServices[serviceName]; ok {
			t.Logf("Skipping service %s (%s)", serviceName, reason)
			continue
		}

		for _, method := range serviceInfo.Methods {
			if method.IsClientStream || method.IsServerStream {
				continue
			}

			fullName := serviceName + "/" + method.Name
			if reason, ok := skipMethods[fullName]; ok {
				t.Logf("Skipping %s (%s)", fullName, reason)
				continue
			}

			t.Run(fmt.Sprintf("%s/%s", serviceName, method.Name), func(t *testing.T) {
				fullMethod := "/" + fullName
				// Invoke with an empty request. The call may fail with bad
				// parameter, permission denied, etc. That's fine; the only
				// failure we're guarding against is Unimplemented, which
				// means the method hit the Unimplemented stub rather than a
				// real handler.
				err := conn.Invoke(ctx, fullMethod, &emptypb.Empty{}, &emptypb.Empty{})
				if err == nil {
					return
				}
				require.False(t, trace.IsNotImplemented(err),
					"RPC %s returned Unimplemented. The method may be missing from its service implementation", fullName)
			})
		}
	}
}
