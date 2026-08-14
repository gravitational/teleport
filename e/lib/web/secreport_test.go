package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestSecurityReports(t *testing.T) {
	t.Parallel()
	svcMock := &mockSecurityReportsService{}
	plug := &mockPlugin{
		service: svcMock,
	}
	s := newWebSuite(t,
		withPlugin(plug),
		withModules(&modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.DeviceTrust: {Enabled: true},
				},
			},
		}),
	)
	webPack := s.newAuthWebPack(t, "alice")

	t.Run("GetReportState", func(t *testing.T) {
		endpoint := webPack.clt.Endpoint("webapi", "sites", "localhost", "audit", "reports", "foobar", "state", "days", "7")
		svcMock.getReportStateFunc = func(_ context.Context, req *pb.GetReportStateRequest) (*pb.ReportState, error) {
			assert.Equal(t, "foobar", req.GetName())
			assert.Equal(t, uint32(7), req.GetDays())
			return pb.ReportState_builder{
				Header: headerv1.ResourceHeader_builder{
					Metadata: headerv1.Metadata_builder{Name: "security_report"}.Build(),
				}.Build(),
				Spec: pb.ReportStateSpec_builder{
					State:     "READY",
					UpdatedAt: "2009-11-10T23:00:00Z",
				}.Build(),
			}.Build(), nil

		}
		resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		want := ui.SecurityReportState{
			Status:    "READY",
			UpdatedAt: "2009-11-10T23:00:00Z",
		}
		assertResponseBody(t, want, resp.Bytes())
	})

	t.Run("GetSchema", func(t *testing.T) {
		endpoint := webPack.clt.Endpoint("webapi", "sites", "localhost", "audit", "schema")
		svcMock.getSchemaFunc = func(_ context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
			return pb.GetSchemaResponse_builder{
				Views: []*pb.GetSchemaResponse_ViewDesc{
					pb.GetSchemaResponse_ViewDesc_builder{
						Name: "name",
						Desc: "desc",
						Columns: []*pb.GetSchemaResponse_ViewDesc_ColumnDesc{
							pb.GetSchemaResponse_ViewDesc_ColumnDesc_builder{Name: "name", Type: "type", Desc: "desc"}.Build(),
						},
					}.Build(),
				},
			}.Build(), nil
		}
		resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		want := ui.SecurityReportSchema{
			Views: []*ui.SecurityReportSchemaView{
				{
					Name: "name",
					Desc: "desc",
					Columns: []*ui.SecurityReportSchemaColumns{
						{Name: "name", Type: "type", Desc: "desc"},
					},
				},
			},
		}
		assertResponseBody(t, want, resp.Bytes())
	})
	t.Run("GetAuditQuery", func(t *testing.T) {
		endpoint := webPack.clt.Endpoint("webapi", "sites", "localhost", "audit", "queries", "name")
		svcMock.getAuditQueryFunc = func(_ context.Context, req *pb.GetAuditQueryRequest) (*pb.AuditQuery, error) {
			assert.Equal(t, "name", req.GetName())
			return pb.AuditQuery_builder{
				Header: headerv1.ResourceHeader_builder{
					Metadata: headerv1.Metadata_builder{Name: "name"}.Build(),
				}.Build(),
				Spec: pb.AuditQuerySpec_builder{
					Name:        "name",
					Title:       "title",
					Query:       "query",
					Description: "description",
				}.Build(),
			}.Build(), nil
		}
		resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())

		want := ui.SecurityAuditQuery{
			Name:        "name",
			Title:       "title",
			Description: "description",
			Query:       "query",
		}
		assertResponseBody(t, want, resp.Bytes())
	})
	t.Run("ListAuditQueries", func(t *testing.T) {
		endpoint := webPack.clt.Endpoint("webapi", "sites", "localhost", "audit", "queries")
		svcMock.listAuditQueriesFunc = func(_ context.Context, _ *pb.ListAuditQueriesRequest) (*pb.ListAuditQueriesResponse, error) {
			return pb.ListAuditQueriesResponse_builder{
				Queries: []*pb.AuditQuery{
					pb.AuditQuery_builder{
						Header: headerv1.ResourceHeader_builder{
							Metadata: headerv1.Metadata_builder{Name: "name"}.Build(),
						}.Build(),
						Spec: pb.AuditQuerySpec_builder{
							Name:        "name",
							Title:       "title",
							Query:       "query",
							Description: "description",
						}.Build(),
					}.Build(),
				},
			}.Build(), nil
		}
		resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())
		want := ui.SecurityAuditQueries{
			{
				Name:        "name",
				Title:       "title",
				Description: "description",
				Query:       "query",
			},
		}
		assertResponseBody(t, want, resp.Bytes())
	})
}

func assertResponseBody[T any](t *testing.T, want T, buff []byte) {
	bb, err := json.Marshal(want)
	require.NoError(t, err)
	require.Equal(t, string(buff), string(bb))
}

type mockSecurityReportsService struct {
	pb.UnimplementedSecReportsServiceServer
	updateAuditQueryFunc func(_ context.Context, _ *pb.UpsertAuditQueryRequest) (*emptypb.Empty, error)
	getAuditQueryFunc    func(_ context.Context, _ *pb.GetAuditQueryRequest) (*pb.AuditQuery, error)
	listAuditQueriesFunc func(_ context.Context, _ *pb.ListAuditQueriesRequest) (*pb.ListAuditQueriesResponse, error)
	getReportStateFunc   func(_ context.Context, _ *pb.GetReportStateRequest) (*pb.ReportState, error)
	getSchemaFunc        func(_ context.Context, _ *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error)
}

func (m mockSecurityReportsService) UpsertAuditQuery(ctx context.Context, request *pb.UpsertAuditQueryRequest) (*emptypb.Empty, error) {
	return m.updateAuditQueryFunc(ctx, request)
}

func (m mockSecurityReportsService) GetAuditQuery(ctx context.Context, request *pb.GetAuditQueryRequest) (*pb.AuditQuery, error) {
	return m.getAuditQueryFunc(ctx, request)
}

func (m mockSecurityReportsService) ListAuditQueries(ctx context.Context, request *pb.ListAuditQueriesRequest) (*pb.ListAuditQueriesResponse, error) {
	return m.listAuditQueriesFunc(ctx, request)
}

func (m mockSecurityReportsService) GetReportState(ctx context.Context, request *pb.GetReportStateRequest) (*pb.ReportState, error) {
	return m.getReportStateFunc(ctx, request)
}

func (m mockSecurityReportsService) GetSchema(ctx context.Context, request *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	return m.getSchemaFunc(ctx, request)
}

type mockPlugin struct {
	service pb.SecReportsServiceServer
}

func (l mockPlugin) GetName() string                            { return "auth.enterprise" }
func (l mockPlugin) RegisterProxyWebHandlers(handler any) error { return nil }
func (l mockPlugin) RegisterAuthWebHandlers(service any) error  { return nil }
func (l mockPlugin) RegisterAuthServices(ctx context.Context, server any, getClientCert getCertFunc) error {
	authServer, ok := server.(*auth.GRPCServer)
	if !ok {
		return trace.BadParameter("unsupported auth server type %T", server)
	}
	gRPCServer, err := authServer.GetServer()
	if err != nil {
		return trace.BadParameter("missing proto server")
	}
	pb.RegisterSecReportsServiceServer(gRPCServer, l.service)
	return nil
}
