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

package testing

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

// SpannerTestClient wraps a [spanner.Client] and provides direct access to the
// underlying [grpc.ClientConn] of the client.
type SpannerTestClient struct {
	ClientConn *grpc.ClientConn
	*spanner.Client
}

// WaitForConnectionState waits until the spanner client's underlying gRPC
// connection transitions into the given state or the context expires.
func (c *SpannerTestClient) WaitForConnectionState(ctx context.Context, wantState connectivity.State) error {
	for {
		s := c.ClientConn.GetState()
		if s == wantState {
			return nil
		}
		if s == connectivity.Shutdown {
			return trace.Errorf("spanner test client connection has shutdown")
		}
		if !c.ClientConn.WaitForStateChange(ctx, s) {
			return ctx.Err()
		}
	}
}

func MakeTestClient(ctx context.Context, config common.TestClientConfig) (*SpannerTestClient, error) {
	return makeTestClient(ctx, config, false)
}

func makeTestClient(ctx context.Context, config common.TestClientConfig, useTLS bool) (*SpannerTestClient, error) {
	databaseID, err := getDatabaseID(ctx, config.RouteToDatabase, config.AuthServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var transportOpt grpc.DialOption
	if useTLS {
		tlsCfg, err := common.MakeTestClientTLSConfig(config)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		transportOpt = grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))
	} else {
		transportOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	cc, err := grpc.NewClient(config.Address, transportOpt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts := []option.ClientOption{
		option.WithGRPCConn(cc),
		// client should not bring any GCP credentials
		option.WithoutAuthentication(),
	}

	clientCfg := spanner.ClientConfig{
		DisableNativeMetrics: true,
		SessionPoolConfig:    spanner.DefaultSessionPoolConfig,
	}
	clt, err := spanner.NewClientWithConfig(ctx, databaseID, clientCfg, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SpannerTestClient{
		ClientConn: cc,
		Client:     clt,
	}, nil
}

func getDatabaseID(ctx context.Context, route tlsca.RouteToDatabase, getter services.DatabaseServersGetter) (string, error) {
	const timeout = 10 * time.Second
	const step = time.Second

	var server types.DatabaseServer
	var err error
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		server, err = getDBServer(ctx, route.ServiceName, getter)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return "", trace.Wrap(err)
		default:
		}
		time.Sleep(step)
	}

	gcp := server.GetDatabase().GetGCP()
	id := fmt.Sprintf("projects/%s/instances/%s/databases/%s", gcp.ProjectID, gcp.InstanceID, route.Database)
	return id, nil
}

func getDBServer(ctx context.Context, name string, getter services.DatabaseServersGetter) (types.DatabaseServer, error) {
	servers, err := getter.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, s := range servers {
		if s.GetName() == name {
			return s, nil
		}
	}
	return nil, trace.NotFound("db_server %q not found", name)
}

type TestServer struct {
	srv      *grpc.Server
	listener net.Listener
	port     string
	spannerpb.UnimplementedSpannerServer
}

func NewTestServer(config common.TestServerConfig) (tsrv *TestServer, err error) {
	err = config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer config.CloseOnError(&err)

	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	port, err := config.Port()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	checker := credentialChecker{expectToken: "Bearer " + config.AuthToken}
	testServer := &TestServer{
		srv: grpc.NewServer(
			grpc.Creds(credentials.NewTLS(tlsConfig)),
			grpc.ChainUnaryInterceptor(unaryAuthInterceptor(checker)),
			grpc.ChainStreamInterceptor(streamingAuthInterceptor(checker)),
		),
		listener: config.Listener,
		port:     port,
	}
	spannerpb.RegisterSpannerServer(testServer.srv, testServer)

	return testServer, nil
}

func unaryAuthInterceptor(c credentialChecker) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if err := c.check(ctx); err != nil {
			return nil, trace.Wrap(err)
		}
		return handler(ctx, req)
	}
}

func streamingAuthInterceptor(c credentialChecker) grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if err := c.check(stream.Context()); err != nil {
			return trace.Wrap(err)
		}
		return handler(srv, stream)
	}
}

type credentialChecker struct {
	expectToken string
}

func (c credentialChecker) check(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return trace.AccessDenied("missing metadata")
	}
	tokens, ok := md["authorization"]
	if !ok || len(tokens) < 1 {
		return trace.AccessDenied("missing credentials in metadata")
	}
	if c.expectToken != tokens[0] {
		return trace.AccessDenied("invalid RPC auth token. wanted: %q got: %q", c, tokens)
	}
	return nil
}

func (s *TestServer) Serve() error {
	return s.srv.Serve(s.listener)
}

func (s *TestServer) Port() string {
	return s.port
}

func (s *TestServer) Close() error {
	s.srv.GracefulStop()
	return nil
}

func (s *TestServer) BatchCreateSessions(ctx context.Context, req *spannerpb.BatchCreateSessionsRequest) (*spannerpb.BatchCreateSessionsResponse, error) {
	tpl := req.SessionTemplate
	if tpl == nil {
		tpl = &spannerpb.Session{CreatorRole: "test"}
	}
	var sessions []*spannerpb.Session
	for range int(req.SessionCount) {
		name := req.GetDatabase() + "/sessions/" + uuid.NewString()
		sessions = append(sessions, &spannerpb.Session{
			Name:        name,
			Labels:      tpl.Labels,
			CreatorRole: tpl.CreatorRole,
			Multiplexed: tpl.Multiplexed,
		})
	}
	res := &spannerpb.BatchCreateSessionsResponse{
		Session: sessions,
	}
	return res, nil
}

func (s *TestServer) ExecuteStreamingSql(req *spannerpb.ExecuteSqlRequest, stream spannerpb.Spanner_ExecuteStreamingSqlServer) error {
	q := strings.TrimSpace(strings.ToLower(req.GetSql()))
	parts := strings.Split(q, " ")
	if len(parts) != 2 || parts[0] != "select" {
		return trace.BadParameter("test server only supports basic select statement")
	}
	_, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return trace.BadParameter("test server only supports selecting an int64")
	}

	return stream.Send(&spannerpb.PartialResultSet{
		Metadata: &spannerpb.ResultSetMetadata{
			RowType: &spannerpb.StructType{
				Fields: []*spannerpb.StructType_Field{{
					Name: "", // no column name
					Type: &spannerpb.Type{
						Code: spannerpb.TypeCode_INT64,
					},
				}},
			},
		},
		Values: []*structpb.Value{
			// integers get encoded as a string in base 10.
			structpb.NewStringValue(parts[1]),
		},
	})
}
