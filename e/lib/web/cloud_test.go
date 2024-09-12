package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/web"
)

type testClient struct {
	cloud.MockedClient

	_ sync.Mutex
	_ func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error)
}

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
	p := httprouter.Params{}

	pass := &cloudapi.SurveyCompanyResponse{
		CompanyName:   "TeleCompany",
		EmployeeCount: "50",
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

	actual, err := s.webPlugin.surveyCompanyResponsesHandler(w, r, p, client)
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
