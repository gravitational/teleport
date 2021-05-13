package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
)

var (
	authAddr = "localhost:3025"
	server   = newMockServer()
)

// mockServer mocks an Auth Server.
type mockServer struct {
	grpc *grpc.Server
	*proto.UnimplementedAuthServiceServer
}

func newMockServer() *mockServer {
	m := &mockServer{
		grpc.NewServer(),
		&proto.UnimplementedAuthServiceServer{},
	}
	proto.RegisterAuthServiceServer(m.grpc, m)
	return m
}

func (m *mockServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{}, nil
}

// mockInsecureCredentials mocks insecure Client credentials.
// it returns a nil tlsConfig which allows the client to run in insecure mode.
type mockInsecureCredentials struct{}

func (mc *mockInsecureCredentials) Dialer(cfg Config) (ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

func (mc *mockInsecureCredentials) TLSConfig() (*tls.Config, error) {
	return nil, nil
}

func (mc *mockInsecureCredentials) SSHClientConfig() (*ssh.ClientConfig, error) {
	return nil, trace.NotImplemented("no ssh config")
}

// TestMain starts mock server listeners and runs the tests.
func TestMain(m *testing.M) {
	authListener, err := net.Listen("tcp", authAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	go server.grpc.Serve(authListener)

	result := m.Run()
	authListener.Close()
	os.Exit(result)
}

func TestNew(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		desc      string
		config    Config
		assertErr require.ErrorAssertionFunc
	}{{
		desc: "successfully dial tcp address.",
		config: Config{
			Addrs: []string{authAddr},
			Credentials: []Credentials{
				&mockInsecureCredentials{},
			},
			DialOpts: []grpc.DialOption{
				grpc.WithInsecure(),
			},
		},
		assertErr: require.NoError,
	}, {
		desc: "synchronously dial addr/cred pairs and successfully dial with the 1 good pair.",
		config: Config{
			Addrs: []string{"bad addr", "bad addr", authAddr},
			Credentials: []Credentials{
				&tlsConfigCreds{nil},
				&tlsConfigCreds{nil},
				&mockInsecureCredentials{},
			},
			DialOpts: []grpc.DialOption{
				grpc.WithInsecure(),
			},
		},
		assertErr: require.NoError,
	}, {
		desc: "fail to dial with a bad address.",
		config: Config{
			DialTimeout: time.Second,
			Addrs:       []string{"bad addr"},
			Credentials: []Credentials{
				&mockInsecureCredentials{},
			},
			DialOpts: []grpc.DialOption{
				grpc.WithInsecure(),
			},
		},
		assertErr: func(t require.TestingT, err error, _ ...interface{}) {
			require.Error(t, err)
			require.Containsf(t, err.Error(), "context deadline exceeded", "")
		},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			clt, err := New(ctx, tt.config)
			tt.assertErr(t, err)

			if err == nil {
				_, err = clt.Ping(ctx)
				require.NoError(t, err)
			}
		})
	}
}

func TestNewDialBackground(t *testing.T) {
	ctx := context.Background()

	// Create client before the server is listening.
	clt, err := New(ctx, Config{
		DialInBackground: true,
		Addrs:            []string{authAddr},
		Credentials: []Credentials{
			&mockInsecureCredentials{},
		},
		DialOpts: []grpc.DialOption{
			grpc.WithInsecure(),
		},
	})
	require.NoError(t, err)

	delayedListener, err := net.Listen("tcp", "localhost:0000")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	t.Cleanup(func() { require.NoError(t, delayedListener.Close()) })
	go server.grpc.Serve(delayedListener)

	_, err = clt.Ping(ctx)
	require.NoError(t, err)
}
