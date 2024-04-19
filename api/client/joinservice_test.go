package client

import (
	"context"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

type mockJoinServiceServer struct {
	*proto.UnimplementedJoinServiceServer
	registerUsingTPMMethod func(srv proto.JoinService_RegisterUsingTPMMethodServer) error
}

func (m *mockJoinServiceServer) RegisterUsingTPMMethod(srv proto.JoinService_RegisterUsingTPMMethodServer) error {
	return m.registerUsingTPMMethod(srv)
}

func TestJoinServiceClient_RegisterUsingTPMMethod(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	lis := bufconn.Listen(100)
	t.Cleanup(func() {
		assert.NoError(t, lis.Close())
	})

	mockInitReq := &proto.RegisterUsingTPMMethodInitialRequest{
		JoinRequest: &types.RegisterUsingTokenRequest{
			Token: "token",
		},
	}
	mockChallenge := &proto.TPMEncryptedCredential{
		CredentialBlob: []byte("cred-blob"),
		Secret:         []byte("secret"),
	}
	mockChallengeResp := &proto.RegisterUsingTPMMethodChallengeResponse{
		Solution: []byte("solution"),
	}
	mockCerts := &proto.Certs{
		TLS: []byte("cert"),
	}
	mockService := &mockJoinServiceServer{
		registerUsingTPMMethod: func(srv proto.JoinService_RegisterUsingTPMMethodServer) error {
			req, err := srv.Recv()
			require.NoError(t, err)
			assert.Empty(t, cmp.Diff(req.GetInit(), mockInitReq))

			err = srv.Send(&proto.RegisterUsingTPMMethodResponse{
				Payload: &proto.RegisterUsingTPMMethodResponse_ChallengeRequest{
					ChallengeRequest: mockChallenge,
				},
			})
			require.NoError(t, err)

			req, err = srv.Recv()
			require.NoError(t, err)
			assert.Empty(t, cmp.Diff(req.GetChallengeResponse(), mockChallengeResp))

			err = srv.Send(&proto.RegisterUsingTPMMethodResponse{
				Payload: &proto.RegisterUsingTPMMethodResponse_Certs{
					Certs: mockCerts,
				},
			})
			require.NoError(t, err)
			return nil
		},
	}
	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})
	proto.RegisterJoinServiceServer(srv, mockService)

	go func() {
		err := srv.Serve(lis)
		assert.NotErrorIs(t, err, grpc.ErrServerStopped)
		cancel()
	}()

	c, err := grpc.NewClient("example.com", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	joinClient := NewJoinServiceClient(proto.NewJoinServiceClient(c))

	certs, err := joinClient.RegisterUsingTPMMethod(
		ctx,
		mockInitReq,
		func(challenge *proto.TPMEncryptedCredential) (*proto.RegisterUsingTPMMethodChallengeResponse, error) {
			assert.Empty(t, cmp.Diff(mockChallenge, challenge))
			return mockChallengeResp, nil
		},
	)
	if assert.NoError(t, err) {
		assert.Empty(t, cmp.Diff(mockCerts, certs))
	}
}
