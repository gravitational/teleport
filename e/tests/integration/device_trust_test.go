package integration

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/defaults"
	publicdevicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/e/tests/common"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

// TestPublicDeviceTrustService verifies that the public Device Trust service is
// reachable over the Proxy Service's public gRPC server. That service forwards
// requests to the equivalent service in the Auth Service.
func TestPublicDeviceTrustService(t *testing.T) {
	sut := common.InitSUT(t)

	ctx := t.Context()
	dialer := apiclient.NewDialer(
		ctx,
		defaults.DefaultIdleTimeout,
		defaults.DefaultIOTimeout,
	)
	tlsConfig := utils.TLSConfig(nil)
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.NextProtos = []string{string(alpncommon.ProtocolProxyGRPCInsecure)}
	conn, err := grpc.NewClient(
		sut.Teleport.ReverseTunnel,
		grpc.WithContextDialer(apiclient.GRPCContextDialer(dialer)),
		grpc.WithUnaryInterceptor(metadata.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(metadata.StreamClientInterceptor),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	dtClient := publicdevicepb.NewDeviceTrustServiceClient(conn)
	_, err = dtClient.CreatePairedDeviceEnrollToken(ctx, publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{}.Build())
	// An empty request reaches the Auth Service handler and fails its validation,
	// confirming the request was forwarded end to end rather than rejected as an
	// unknown service by the Proxy Service or Auth Service.
	assert.ErrorAs(t, trail.FromGRPC(err), new(*trace.BadParameterError))
}
