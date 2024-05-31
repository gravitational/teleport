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
		UsageBasedBilling:            false,
		StripePublicKey:              "",
		StripeCustomerId:             "",
		StripeCurrentUsage:           nil,
		StripeTrial:                  false,
		StripeTrialEnd:               0,
		StripeMissingPaymentMethod:   false,
		ProductName:                  "",
		StripeSubscriptionStatus:     "",
		StripeSubscriptionCancelAt:   0,
		StripeSubscriptionCanceledAt: 0,
		UsageUpdatedAt:               0,
		UsageQuota:                   nil,
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

func TestPlugin_removeCardHandle(t *testing.T) {
	t.Parallel()
	jsonReq := `{"cardId": "pm_1OV1F1LFy3hZi6txaLZGPWRp"}`
	var calledWith *cloudapi.RemoveCardRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/enterprise/cloud/card", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockRemoveCard: func(in *cloudapi.RemoveCardRequest) (*cloudapi.EmptyResponse, error) {
				calledWith = in
				return &cloudapi.EmptyResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.removeCardHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, "pm_1OV1F1LFy3hZi6txaLZGPWRp", calledWith.CardId)
}

func TestPlugin_addCardHandle(t *testing.T) {
	t.Parallel()
	jsonReq := `{ "cardId": "pm_1PId4JLFy3hZi6txw7bzUYbK", "isDefault": false }`
	var calledWith *cloudapi.AddCardRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/card", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockAddCard: func(in *cloudapi.AddCardRequest) (*cloudapi.EmptyResponse, error) {
				calledWith = in
				return &cloudapi.EmptyResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.addCardHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, "pm_1PId4JLFy3hZi6txw7bzUYbK", calledWith.CardId)
	require.False(t, calledWith.IsDefault)
}

func TestPlugin_updateCardHandle(t *testing.T) {
	t.Parallel()
	jsonReq := `{ "prevCardId": "pm_1PId4JLFy3hZi6txw7bzUYbK", "nextCardId": "pm_1PId4JLFy3hZi6txw7bzUYbK", "isDefault": true }`
	var calledWith *cloudapi.UpdateCardRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/card", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockUpdateCard: func(in *cloudapi.UpdateCardRequest) (*cloudapi.EmptyResponse, error) {
				calledWith = in
				return &cloudapi.EmptyResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.updateCardHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, "pm_1PId4JLFy3hZi6txw7bzUYbK", calledWith.PrevCardId)
	require.Equal(t, "pm_1PId4JLFy3hZi6txw7bzUYbK", calledWith.NextCardId)
	require.True(t, calledWith.IsDefault)
}

func TestPlugin_getBillingInformationHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/billing", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	pass := &cloudapi.GetBillingInformationResponse{
		DefaultPaymentMethodId:       "",
		Cards:                        nil,
		StripePublicKey:              "some-public-key",
		ProductName:                  "some-product-name",
		Trial:                        false,
		SelfEnrolled:                 false,
		UpsellAlert:                  false,
		UsageBasedBilling:            true,
		StripeTrial:                  false,
		StripeTrialEnd:               0,
		StripeMissingPaymentMethod:   true,
		StripeCustomerId:             "",
		StripeSubscriptionStatus:     "active",
		StripeSubscriptionCancelAt:   0,
		StripeSubscriptionCanceledAt: 0,
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

func TestPlugin_cancelSubscriptionHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/enterprise/cloud/billing", nil)
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockCancelSubscription: func(ctx context.Context, in *cloudapi.EmptyRequest, opts ...grpc.CallOption) (*cloudapi.EmptyResponse, error) {
				return &cloudapi.EmptyResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.cancelSubscriptionHandle(w, r, wCtx, client)
	require.NoError(t, err)
}

func TestPlugin_getPaymentsInvoicesInformationHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/payments-invoices", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	pass := &cloudapi.GetPaymentsInvoicesInformationResponse{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetPaymentsInvoicesInformation: func(context.Context, *cloudapi.EmptyRequest, ...grpc.CallOption) (*cloudapi.GetPaymentsInvoicesInformationResponse, error) {
				return pass, nil
			},
		},
	}

	actual, err := s.webPlugin.getPaymentsInvoicesInformationHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, pass, actual)
}

func TestPlugin_getInvoiceSettingsInformationHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/enterprise/cloud/invoice-settings", nil)
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	pass := &cloudapi.GetInvoiceSettingsInformationResponse{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockGetInvoiceSettingsInformation: func(context.Context, *cloudapi.EmptyRequest, ...grpc.CallOption) (*cloudapi.GetInvoiceSettingsInformationResponse, error) {
				return pass, nil
			},
		},
	}

	actual, err := s.webPlugin.getInvoiceSettingsInformationHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, pass, actual)

}

func TestPlugin_updateStripeAddressHandle(t *testing.T) {
	t.Parallel()
	jsonReq := `{ "name": "First Last", "address": { "addressCity": "City", "addressCountry": "US", "addressLine1": "1111 Street", "addressLine2": "XX Floor", "addressPostalCode": "99999", "addressState": "NY" } }`
	var calledWith *cloudapi.StripeBillingAddressRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/enterprise/cloud/address", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockUpdateStripeAddress: func(_ context.Context, in *cloudapi.StripeBillingAddressRequest, _ ...grpc.CallOption) (*cloudapi.EmptyResponse, error) {
				calledWith = in
				return &cloudapi.EmptyResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.updateStripeAddressHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, "City", calledWith.Address.AddressCity)
	require.Equal(t, "US", calledWith.Address.AddressCountry)
	require.Equal(t, "1111 Street", calledWith.Address.AddressLine1)
	require.Equal(t, "XX Floor", calledWith.Address.AddressLine2)
	require.Equal(t, "99999", calledWith.Address.AddressPostalCode)
	require.Equal(t, "NY", calledWith.Address.AddressState)
	require.Equal(t, "First Last", calledWith.Name)
}

func TestPlugin_createSetupIntentHandle(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/setupintent", nil)
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockCreateSetupIntent: func(context.Context, *cloudapi.EmptyRequest, ...grpc.CallOption) (*cloudapi.CreateSetupIntentResponse, error) {
				return &cloudapi.CreateSetupIntentResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.createSetupIntentHandle(w, r, wCtx, client)
	require.NoError(t, err)
}

func TestPlugin_updateEmailHandle(t *testing.T) {
	t.Parallel()
	jsonReq := `{ "email": "user+111@goteleport.com" }`
	var calledWith *cloudapi.UpdateEmailRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/enterprise/cloud/billing-email", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockUpdateEmail: func(_ context.Context, in *cloudapi.UpdateEmailRequest, _ ...grpc.CallOption) (*cloudapi.EmptyResponse, error) {
				calledWith = in
				return &cloudapi.EmptyResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.updateEmailHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, "user+111@goteleport.com", calledWith.Email)
}

func TestPlugin_updatePurchaseOrderHandle(t *testing.T) {
	t.Parallel()
	jsonReq := `{ "po": "888" }`
	var calledWith *cloudapi.UpdatePurchaseOrderPrefixRequest

	s := newWebSuite(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/enterprise/cloud/billing-po", strings.NewReader(jsonReq))
	r.Header.Add("content-type", "application/json")
	r = r.WithContext(authz.ContextWithUser(context.Background(), authz.LocalUser{}))
	wCtx := &web.SessionContext{}

	client := &testClient{
		MockedClient: cloud.MockedClient{
			MockUpdatePurchaseOrderPrefix: func(ctx context.Context, in *cloudapi.UpdatePurchaseOrderPrefixRequest, opts ...grpc.CallOption) (*cloudapi.EmptyResponse, error) {
				calledWith = in
				return &cloudapi.EmptyResponse{}, nil
			},
		},
	}

	_, err := s.webPlugin.updatePurchaseOrderHandle(w, r, wCtx, client)
	require.NoError(t, err)
	require.Equal(t, "888", calledWith.Po)
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
