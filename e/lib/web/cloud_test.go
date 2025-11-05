package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

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

func TestPlugin_getUpgradeWindowStartHourHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/upgradewindowstart", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	pass := &cloudapi.GetAccountUpgradeWindowStartHourResponse{
		UpgradeWindowStartHour: 16,
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetAccountUpgradeWindowStartHour: func() (*cloudapi.GetAccountUpgradeWindowStartHourResponse, error) {
				return pass, nil
			},
		},
	}

	actual, err := s.webPlugin.getUpgradeWindowStartHourHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, pass, actual)
}

func TestPlugin_surveyCompanyResponsesHandler(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/survey/company", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	pass := &cloudapi.SurveyCompanyResponse{
		MarketingParams: &cloudapi.MarketingParamData{
			Campaign: "some-camp",
			Source:   "some-source",
			Medium:   "some-medium",
			Intent:   "some-intent",
		},
	}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetSurveyCompany: func(context.Context, *cloudapi.EmptyRequest, ...grpc.CallOption) (*cloudapi.SurveyCompanyResponse, error) {
				return pass, nil
			},
		},
	}

	actual, err := s.webPlugin.surveyCompanyResponsesHandler(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, pass, actual)
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
