package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
)

type testClient struct {
	cloud.MockedClient

	_ sync.Mutex
	_ func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error)
}

func (t *testClient) Hostname() string { return "teleport.example.com" }

// Close implements cloud.Client interface for mocked client
func (t *testClient) Close() error { return nil }

// fakeGetFileStream implements cloudapi.TenantsService_GetFileClient for testing.
// It mimics the real server: first chunk carries ContentEncoding metadata, second carries Data.
type fakeGetFileStream struct {
	resp *cloudapi.GetFileResponse
	sent int
}

func (f *fakeGetFileStream) Recv() (*cloudapi.GetFileResponse, error) {
	if f.resp == nil {
		return nil, io.EOF
	}
	switch f.sent {
	case 0:
		f.sent++
		return &cloudapi.GetFileResponse{ContentEncoding: f.resp.ContentEncoding}, nil
	case 1:
		f.sent++
		return &cloudapi.GetFileResponse{Data: f.resp.Data}, nil
	default:
		return nil, io.EOF
	}
}

func (f *fakeGetFileStream) Header() (metadata.MD, error) { return nil, nil }
func (f *fakeGetFileStream) Trailer() metadata.MD         { return nil }
func (f *fakeGetFileStream) CloseSend() error             { return nil }
func (f *fakeGetFileStream) Context() context.Context     { return context.Background() }
func (f *fakeGetFileStream) SendMsg(m any) error          { return nil }
func (f *fakeGetFileStream) RecvMsg(m any) error          { return nil }

func TestPlugin_getBillingSummaryInformationHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/billing-summary", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	pass := &cloudapi.GetBillingSummaryInformationResponse{
		UsageBasedBilling:  false,
		StripeCurrentUsage: nil,
		ProductName:        "",
		UsageUpdatedAt:     0,
		UsageQuota:         nil,
		UsageSummary: &cloudapi.UsageSummary{
			UsageHistory: []*cloudapi.UsageHistoryItem{
				{
					Mau:                 10,
					Tpr:                 10,
					CycleStartFormatted: "Jan 02, 2024",
					CycleEndFormatted:   "Feb 01, 2024",
				},
			},
		},
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetBillingSummaryInformation: func(context.Context, *cloudapi.EmptyRequest, ...grpc.CallOption) (*cloudapi.GetBillingSummaryInformationResponse, error) {
				return pass, nil
			},
		},
	}

	actual, err := s.webPlugin.getBillingSummaryInformationHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, pass, actual)
}

func TestPlugin_getUsageHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	jsonReq := `{"tenants": []}`
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/billing-summary", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	pass := &cloudapi.GetUsageResponse{
		UsageHistory: []*cloudapi.UsageCycle{
			{
				StartFormatted: "Jan 02, 2024",
				EndFormatted:   "Feb 01, 2024",
				Usage: &cloudapi.Usage{
					Igmau:  10,
					Ztamau: 11,
					Mwi:    33,
					Tpr:    10,
				},
				UsageLimits: &cloudapi.UsageLimits{
					Igmau:  11,
					Ztamau: 11,
					Mwi:    30,
					Tpr:    10,
				},
			},
		},
		MissingEntitlements: []string{"Identity"},
		AggregateCount:      4,
		UsageUpdatedAt:      1758864837,
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetUsage: func(ctx context.Context, in *cloudapi.GetUsageRequest, opts ...grpc.CallOption) (*cloudapi.GetUsageResponse, error) {
				return pass, nil
			},
		},
	}

	actual, err := s.webPlugin.getUsageHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, pass, actual)
}

func TestPlugin_getBillingInformationHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/billing", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	pass := &cloudapi.GetBillingInformationResponse{
		ProductName:       "some-product-name",
		Trial:             false,
		SelfEnrolled:      false,
		UpsellAlert:       false,
		UsageBasedBilling: true,
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetBillingInformation: func() (*cloudapi.GetBillingInformationResponse, error) {
				return pass, nil
			},
		},
	}

	actual, err := s.webPlugin.getBillingInformationHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, pass, actual)
}

func TestPlugin_getClusterUpgradeWindowStartHourHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/sites/some-cluster/upgradewindowstart", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.GetAccountUpgradeWindowStartHourResponse{
		UpgradeWindowStartHour: 16,
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetAccountUpgradeWindowStartHour: func() (*cloudapi.GetAccountUpgradeWindowStartHourResponse, error) {
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.getClusterUpgradeWindowStartHourHandle(w, r, wCtx, nil, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPlugin_getEnvironmentProfileHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/environmentprofile", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.GetEnvironmentProfileResponse{
		EnvironmentProfile: "production",
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetEnvironmentProfile: func() (*cloudapi.GetEnvironmentProfileResponse, error) {
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.getEnvironmentProfileHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPlugin_updateEnvironmentProfileHandle(t *testing.T) {
	t.Parallel()
	jsonReq := `{"environment_profile": "staging"}`
	var calledWith *cloudapi.UpdateEnvironmentProfileRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/environmentprofile", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockUpdateEnvironmentProfile: func(in *cloudapi.UpdateEnvironmentProfileRequest) (*cloudapi.GetEnvironmentProfileResponse, error) {
				calledWith = in
				return &cloudapi.GetEnvironmentProfileResponse{EnvironmentProfile: in.EnvironmentProfile}, nil
			},
		},
	}

	actual, err := s.webPlugin.updateEnvironmentProfileHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, "staging", calledWith.EnvironmentProfile)
	require.Equal(t, &cloudapi.GetEnvironmentProfileResponse{EnvironmentProfile: "staging"}, actual)
}

func TestPlugin_surveyResultsHandler(t *testing.T) {
	t.Parallel()
	jsonReq := `{ "companyName": "some-company", "employeeCount": "35", "resources": [], "role": "eng", "team": "marketing", "username": "some-username" }`
	var calledWith *cloudapi.SetSurveyResultsRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/survey", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockSetSurveyResults: func(ctx context.Context, in *cloudapi.SetSurveyResultsRequest, opts ...grpc.CallOption) (*cloudapi.EmptyResponse, error) {
				calledWith = in
				return &cloudapi.EmptyResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.surveyResultsHandler(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, "some-company", calledWith.CompanyName)
	require.Equal(t, "marketing", calledWith.Team)
	require.Equal(t, "35", calledWith.EmployeeCount)
	require.Equal(t, "eng", calledWith.Role)
	require.Equal(t, []string(nil), calledWith.Resources)
}

func TestPlugin_getClusterContactHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/sites/survey/localhost/contact", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	now := time.Now()
	expected := &cloudapi.GetContactsResponse{
		Contacts: []*cloudapi.Contact{
			{
				Name:            "contact1",
				AccountId:       "accid",
				VerifyToken:     "verifytoken2",
				Email:           "email1@goteleport.com",
				ContactType:     int32(3),
				Verified:        true,
				VerifyExpiresAt: now.Unix(),
				State:           cloudapi.ContactState_CONTACT_STATE_ACTIVE,
			},
			{
				Name:            "contact2",
				AccountId:       "accid",
				VerifyToken:     "verifytoken2",
				Email:           "email2@goteleport.com",
				ContactType:     int32(1),
				Verified:        false,
				VerifyExpiresAt: now.AddDate(1, 0, 0).Unix(),
				State:           cloudapi.ContactState_CONTACT_STATE_PENDING,
			},
		},
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetContacts: func(ctx context.Context, in *cloudapi.EmptyRequest, opts ...grpc.CallOption) (*cloudapi.GetContactsResponse, error) {
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.getClusterContactHandle(w, r, wCtx, nil, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPlugin_createClusterContactHandle(t *testing.T) {
	t.Parallel()
	jsonReq := `{ "email": "example@goteleport.com", "contact_type": 1 }`
	var calledWith *cloudapi.CreateContactRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/sites/survey/localhost/contact", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockCreateContact: func(ctx context.Context, in *cloudapi.CreateContactRequest, opts ...grpc.CallOption) (*cloudapi.CreateContactResponse, error) {
				calledWith = in
				return &cloudapi.CreateContactResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.createClusterContactHandle(w, r, wCtx, nil, client)
	require.NoError(t, err)
	require.Equal(t, "example@goteleport.com", calledWith.Email)
	require.Equal(t, int32(1), calledWith.ContactType)
}

func TestPlugin_deleteClusterContactHandle(t *testing.T) {
	t.Parallel()
	jsonReq := `{ "verify_token": "tokenid", "contact_type": 2 }`
	var calledWith *cloudapi.RemoveContactRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/sites/survey/localhost/contact", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockRemoveContact: func(ctx context.Context, in *cloudapi.RemoveContactRequest, opts ...grpc.CallOption) (*cloudapi.RemoveContactResponse, error) {
				calledWith = in
				return &cloudapi.RemoveContactResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.deleteClusterContactHandle(w, r, wCtx, nil, client)
	require.NoError(t, err)
	require.Equal(t, "tokenid", calledWith.VerifyToken)
	require.Equal(t, int32(2), calledWith.ContactType)
}

func TestPlugin_getCloudAssetHandle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		path                    string
		response                *cloudapi.GetFileResponse
		clientErr               error
		expectedStatus          int
		expectedContentType     string
		expectedContentEncoding string
		expectedBody            []byte
	}{
		{
			name: "returns js file with correct content-type",
			path: "/enterprise/cloud/assets/app.js",
			response: &cloudapi.GetFileResponse{
				Data: []byte("console.log('hello')"),
			},
			expectedStatus:      http.StatusOK,
			expectedContentType: "text/javascript; charset=utf-8",
			expectedBody:        []byte("console.log('hello')"),
		},
		{
			name: "sets Content-Encoding when present",
			path: "/enterprise/cloud/assets/app.js",
			response: &cloudapi.GetFileResponse{
				Data: []byte("gzip-data"),
				// decompression is handled by the browser
				ContentEncoding: "gzip",
			},
			expectedStatus:          http.StatusOK,
			expectedContentType:     "text/javascript; charset=utf-8",
			expectedContentEncoding: "gzip",
			expectedBody:            []byte("gzip-data"),
		},
		{
			name:           "returns error when client fails",
			path:           "/enterprise/cloud/assets/app.js",
			clientErr:      trace.NotFound("file not found"),
			expectedStatus: http.StatusOK, // error is propagated via return value, not written to w
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := newWebSuite(t)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))

			client := &testClient{
				MockedClient: cloud.MockedClient{
					MockGetFile: func(ctx context.Context, in *cloudapi.GetFileRequest, opts ...grpc.CallOption) (cloudapi.TenantsService_GetFileClient, error) {
						if tt.clientErr != nil {
							return nil, tt.clientErr
						}
						return &fakeGetFileStream{resp: tt.response}, nil
					},
				},
			}

			_, err := s.webPlugin.getCloudAssetHandle(w, r, nil, client)
			if tt.clientErr != nil {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedContentType, w.Header().Get("Content-Type"))
			require.Equal(t, tt.expectedContentEncoding, w.Header().Get("Content-Encoding"))
			require.Equal(t, tt.expectedBody, w.Body.Bytes())
		})
	}
}

func TestPlugin_getMAUBreakdownHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)

	pass := &cloudapi.GetMAUDailyBreakdownResponse{
		Days: []*cloudapi.MAUDailyPoint{
			{Day: 1775001600, NewInWindow: 2, ContributingClusters: 1},
		},
		RangeStart: 1775001600,
		RangeEnd:   1776124800,
	}

	tests := []struct {
		name      string
		query     string
		wantReq   *cloudapi.GetMAUDailyBreakdownRequest
		clientErr error
		wantErr   bool
	}{
		{
			name:    "missing window errors",
			query:   "",
			wantErr: true,
		},
		{
			name:  "tenants passed as repeated param",
			query: "tenants=acc1&tenants=acc2&window-cycle=1775001600",
			wantReq: &cloudapi.GetMAUDailyBreakdownRequest{
				Tenants: []string{"acc1", "acc2"},
				Window: &cloudapi.DailyBreakdownWindow{
					Window: &cloudapi.DailyBreakdownWindow_Cycle{Cycle: 1775001600},
				},
			},
		},
		{
			name:  "cycle window",
			query: "window-cycle=1775001600",
			wantReq: &cloudapi.GetMAUDailyBreakdownRequest{
				Window: &cloudapi.DailyBreakdownWindow{
					Window: &cloudapi.DailyBreakdownWindow_Cycle{Cycle: 1775001600},
				},
			},
		},
		{
			name:  "range window",
			query: "window-range-start=1775001600&window-range-end=1776124800",
			wantReq: &cloudapi.GetMAUDailyBreakdownRequest{
				Window: &cloudapi.DailyBreakdownWindow{
					Window: &cloudapi.DailyBreakdownWindow_Range{
						Range: &cloudapi.DateRange{Start: 1775001600, End: 1776124800},
					},
				},
			},
		},
		{
			name:    "range start after end errors",
			query:   "window-range-start=1776124800&window-range-end=1775001600",
			wantErr: true,
		},
		{
			name:      "client error is propagated",
			query:     "window-cycle=1775001600",
			clientErr: trace.NotFound("not found"),
			wantErr:   true,
		},
		{
			name:      "access denied is propagated",
			query:     "window-cycle=1775001600",
			clientErr: trace.AccessDenied("access denied"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/billing/breakdown/mau?"+tc.query, nil)
			w := httptest.NewRecorder()
			r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))

			var calledWith *cloudapi.GetMAUDailyBreakdownRequest
			client := &testClient{
				MockedClient: cloud.MockedClient{
					MockGetMAUDailyBreakdown: func(ctx context.Context, in *cloudapi.GetMAUDailyBreakdownRequest, opts ...grpc.CallOption) (*cloudapi.GetMAUDailyBreakdownResponse, error) {
						calledWith = in
						if tc.clientErr != nil {
							return nil, tc.clientErr
						}
						return pass, nil
					},
				},
			}

			actual, err := s.webPlugin.getMAUBreakdownHandle(w, r, &web.SessionContext{}, client)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, pass, actual)
			require.Equal(t, tc.wantReq, calledWith)
		})
	}
}

func TestPlugin_getTPRBreakdownHandle(t *testing.T) {
	t.Parallel()

	s := newWebSuite(t)

	pass := &cloudapi.GetTPRDailyBreakdownResponse{
		Days: []*cloudapi.TPRDailyPoint{
			{Day: 1775001600, PeriodAvg: 5, ContributingClusters: 1},
		},
		RangeStart: 1775001600,
		RangeEnd:   1776124800,
	}

	tests := []struct {
		name      string
		query     string
		wantReq   *cloudapi.GetTPRDailyBreakdownRequest
		clientErr error
		wantErr   bool
	}{
		{
			name:    "missing window errors",
			query:   "",
			wantErr: true,
		},
		{
			name:  "tenants passed as repeated param",
			query: "tenants=acc1&tenants=acc2&window-cycle=1775001600",
			wantReq: &cloudapi.GetTPRDailyBreakdownRequest{
				Tenants: []string{"acc1", "acc2"},
				Window: &cloudapi.DailyBreakdownWindow{
					Window: &cloudapi.DailyBreakdownWindow_Cycle{Cycle: 1775001600},
				},
			},
		},
		{
			name:  "cycle window",
			query: "window-cycle=1775001600",
			wantReq: &cloudapi.GetTPRDailyBreakdownRequest{
				Window: &cloudapi.DailyBreakdownWindow{
					Window: &cloudapi.DailyBreakdownWindow_Cycle{Cycle: 1775001600},
				},
			},
		},
		{
			name:  "range window",
			query: "window-range-start=1775001600&window-range-end=1776124800",
			wantReq: &cloudapi.GetTPRDailyBreakdownRequest{
				Window: &cloudapi.DailyBreakdownWindow{
					Window: &cloudapi.DailyBreakdownWindow_Range{
						Range: &cloudapi.DateRange{Start: 1775001600, End: 1776124800},
					},
				},
			},
		},
		{
			name:    "range start after end errors",
			query:   "window-range-start=1776124800&window-range-end=1775001600",
			wantErr: true,
		},
		{
			name:      "client error is propagated",
			query:     "window-range-start=1775001600&window-range-end=1776124800",
			clientErr: trace.NotFound("not found"),
			wantErr:   true,
		},
		{
			name:      "access denied is propagated",
			query:     "window-range-start=1775001600&window-range-end=1776124800",
			clientErr: trace.AccessDenied("access denied"),
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/billing/breakdown/tpr?"+tc.query, nil)
			r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))

			var calledWith *cloudapi.GetTPRDailyBreakdownRequest
			client := &testClient{
				MockedClient: cloud.MockedClient{
					MockGetTPRDailyBreakdown: func(ctx context.Context, in *cloudapi.GetTPRDailyBreakdownRequest, opts ...grpc.CallOption) (*cloudapi.GetTPRDailyBreakdownResponse, error) {
						calledWith = in
						if tc.clientErr != nil {
							return nil, tc.clientErr
						}
						return pass, nil
					},
				},
			}

			actual, err := s.webPlugin.getTPRBreakdownHandle(w, r, &web.SessionContext{}, client)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, pass, actual)
			require.Equal(t, tc.wantReq, calledWith)
		})
	}
}

func TestPlugin_withCloudCache(t *testing.T) {
	t.Parallel()
	counter := 0
	webPlugin, err := NewPlugin(Config{})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodGet, "/foo/bar", nil)
	fn := func(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, client cloud.Client) (any, error) {
		counter++
		if counter == 1 {
			return nil, errors.New("error")
		}
		if counter == 2 {
			return "ok", nil
		}
		if counter == 3 {
			return nil, errors.New("error")
		}
		if counter == 4 {
			return "ok2", nil
		}
		if counter == 5 {
			return nil, trace.AccessDenied("error")
		}

		return nil, nil
	}

	// error when cache is empty returns error
	handler := webPlugin.withCloudCache(fn)
	_, err = handler(httptest.NewRecorder(), r, nil, nil)
	require.Error(t, err)

	// successful response returns
	res2, err := handler(httptest.NewRecorder(), r, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "ok", res2)

	// error when cache is populated returns cache
	res3, err := handler(httptest.NewRecorder(), r, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "ok", res3)

	// successful response returns when cache is populated
	res4, err := handler(httptest.NewRecorder(), r, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "ok2", res4)

	// unauthorized response returns error
	_, err = handler(httptest.NewRecorder(), r, nil, nil)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestPlugin_withCloudClusterCache(t *testing.T) {
	t.Parallel()
	counter := 0
	webPlugin, err := NewPlugin(Config{})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodGet, "/foo/bar", nil)
	fn := func(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error) {
		counter++
		if counter == 1 {
			return nil, errors.New("error")
		}
		if counter == 2 {
			return "ok", nil
		}
		if counter == 3 {
			return nil, errors.New("error")
		}
		if counter == 4 {
			return "ok2", nil
		}
		if counter == 5 {
			return nil, trace.AccessDenied("error")
		}

		return nil, nil
	}

	// error when cache is empty returns error
	handler := webPlugin.withCloudClusterCache(fn)
	_, err = handler(httptest.NewRecorder(), r, nil, &mockCluster{name: "localhost"}, nil)
	require.Error(t, err)

	// successful response returns
	res2, err := handler(httptest.NewRecorder(), r, nil, &mockCluster{name: "localhost"}, nil)
	require.NoError(t, err)
	require.Equal(t, "ok", res2)

	// error when cache is populated returns cache
	res3, err := handler(httptest.NewRecorder(), r, nil, &mockCluster{name: "localhost"}, nil)
	require.NoError(t, err)
	require.Equal(t, "ok", res3)

	// successful response returns when cache is populated
	res4, err := handler(httptest.NewRecorder(), r, nil, &mockCluster{name: "localhost"}, nil)
	require.NoError(t, err)
	require.Equal(t, "ok2", res4)

	// unauthorized response returns error
	_, err = handler(httptest.NewRecorder(), r, nil, &mockCluster{name: "localhost"}, nil)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestPlugin_getStripeConfigHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/stripe/config", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.GetStripeConfigResponse{
		PublicKey:        "pk_test_123",
		StripeCustomerId: "cus_123",
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetStripeConfig: func(ctx context.Context, in *cloudapi.GetStripeConfigRequest, opts ...grpc.CallOption) (*cloudapi.GetStripeConfigResponse, error) {
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.getStripeConfigHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPlugin_stripeListCardsHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/stripe/cards", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeListCardsResponse{
		StripeCards: []*cloudapi.StripeCard{
			{Id: "card_1", Last4: "4242", Brand: "visa"},
		},
		StripeDefaultSourceId:      "card_1",
		StripeMissingPaymentMethod: false,
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeListCards: func(ctx context.Context, in *cloudapi.StripeListCardsRequest, opts ...grpc.CallOption) (*cloudapi.StripeListCardsResponse, error) {
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeListCardsHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPlugin_stripeCreateSetupIntentHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/stripe/setup-intent", strings.NewReader(`{}`))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeCreateSetupIntentResponse{ClientSecret: "seti_secret_abc"}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeCreateSetupIntent: func(ctx context.Context, in *cloudapi.StripeCreateSetupIntentRequest, opts ...grpc.CallOption) (*cloudapi.StripeCreateSetupIntentResponse, error) {
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeCreateSetupIntentHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPlugin_stripeCreateCardHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	jsonReq := `{"cardId":"pm_123","isDefault":true}`
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/stripe/cards", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeCreateCardResponse{}

	var got *cloudapi.StripeCreateCardRequest
	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeCreateCard: func(ctx context.Context, in *cloudapi.StripeCreateCardRequest, opts ...grpc.CallOption) (*cloudapi.StripeCreateCardResponse, error) {
				got = in
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeCreateCardHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.Equal(t, "pm_123", got.CardId)
	require.True(t, got.IsDefault)
}

func TestPlugin_stripeUpdateCardHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	jsonReq := `{"prevCardId":"pm_a","nextCardId":"pm_b","isDefault":true}`
	r := httptest.NewRequest(http.MethodPut, "/enterprise/cloud/stripe/cards", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeUpdateCardResponse{}

	var got *cloudapi.StripeUpdateCardRequest
	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeUpdateCard: func(ctx context.Context, in *cloudapi.StripeUpdateCardRequest, opts ...grpc.CallOption) (*cloudapi.StripeUpdateCardResponse, error) {
				got = in
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeUpdateCardHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.Equal(t, "pm_a", got.PrevCardId)
	require.Equal(t, "pm_b", got.NextCardId)
	require.True(t, got.IsDefault)
}

func TestPlugin_stripeDeleteCardHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	jsonReq := `{"cardId":"pm_123"}`
	r := httptest.NewRequest(http.MethodDelete, "/enterprise/cloud/stripe/cards", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeDeleteCardResponse{}

	var got *cloudapi.StripeDeleteCardRequest
	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeDeleteCard: func(ctx context.Context, in *cloudapi.StripeDeleteCardRequest, opts ...grpc.CallOption) (*cloudapi.StripeDeleteCardResponse, error) {
				got = in
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeDeleteCardHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.Equal(t, "pm_123", got.CardId)
}

func TestPlugin_stripeListInvoicesHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/stripe/invoices", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeListInvoicesResponse{
		StripeInvoices: []*cloudapi.StripeInvoice{
			{InvoiceId: "IN-001", Status: "paid", AmountDue: 1000, AmountPaid: 1000},
		},
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeListInvoices: func(ctx context.Context, in *cloudapi.StripeListInvoicesRequest, opts ...grpc.CallOption) (*cloudapi.StripeListInvoicesResponse, error) {
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeListInvoicesHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPlugin_stripeGetSettingsHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/stripe/invoice-settings", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeGetSettingsResponse{
		StripeInvoiceEmail:       "billing@example.com",
		StripeInvoicePrefix:      "ACME",
		StripeCustomerName:       "Acme Corp",
		StripeSubscriptionStatus: "active",
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeGetSettings: func(ctx context.Context, in *cloudapi.StripeGetSettingsRequest, opts ...grpc.CallOption) (*cloudapi.StripeGetSettingsResponse, error) {
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeGetSettingsHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPlugin_stripeUpdateEmailHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	jsonReq := `{"email":"billing@example.com"}`
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/stripe/invoice-settings/email", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeUpdateEmailResponse{}

	var got *cloudapi.StripeUpdateEmailRequest
	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeUpdateEmail: func(ctx context.Context, in *cloudapi.StripeUpdateEmailRequest, opts ...grpc.CallOption) (*cloudapi.StripeUpdateEmailResponse, error) {
				got = in
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeUpdateEmailHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.Equal(t, "billing@example.com", got.Email)
}

func TestPlugin_stripeUpdatePOPrefixHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	jsonReq := `{"po":"ACME"}`
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/stripe/invoice-settings/po-prefix", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeUpdatePOPrefixResponse{}

	var got *cloudapi.StripeUpdatePOPrefixRequest
	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeUpdatePOPrefix: func(ctx context.Context, in *cloudapi.StripeUpdatePOPrefixRequest, opts ...grpc.CallOption) (*cloudapi.StripeUpdatePOPrefixResponse, error) {
				got = in
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeUpdatePOPrefixHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.Equal(t, "ACME", got.Po)
}

func TestPlugin_stripeUpdateStripeAddressHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	jsonReq := `{"name":"Acme Corp","address":{"addressLine1":"1 Main","addressCity":"Oakland","addressCountry":"US"}}`
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/stripe/invoice-settings/address", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeUpdateStripeAddressResponse{}

	var got *cloudapi.StripeUpdateStripeAddressRequest
	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeUpdateStripeAddress: func(ctx context.Context, in *cloudapi.StripeUpdateStripeAddressRequest, opts ...grpc.CallOption) (*cloudapi.StripeUpdateStripeAddressResponse, error) {
				got = in
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeUpdateStripeAddressHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.Equal(t, "Acme Corp", got.Name)
	require.Equal(t, "1 Main", got.Address.AddressLine1)
	require.Equal(t, "Oakland", got.Address.AddressCity)
	require.Equal(t, "US", got.Address.AddressCountry)
}

func TestPlugin_stripeCancelHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/stripe/cancel", strings.NewReader(`{}`))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	expected := &cloudapi.StripeCancelResponse{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockStripeCancel: func(ctx context.Context, in *cloudapi.StripeCancelRequest, opts ...grpc.CallOption) (*cloudapi.StripeCancelResponse, error) {
				return expected, nil
			},
		},
	}

	actual, err := s.webPlugin.stripeCancelHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
