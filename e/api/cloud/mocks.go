package cloud

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
)

// MockedClient mocks cloud client APIs
type MockedClient struct {
	// MockSubmitUsageReports reports usage
	MockSubmitUsageReports func(in *v1.SubmitUsageReportsRequest) (*v1.EmptyResponse, error)
	// MockGetBillingInformation returns customer billing information
	MockGetBillingInformation func() (*v1.GetBillingInformationResponse, error)
	// MockCreateSetupIntent creates an intent in stripe and returns the client secret
	MockCreateSetupIntent func(context.Context, *v1.EmptyRequest, ...grpc.CallOption) (*v1.CreateSetupIntentResponse, error)
	// MockAddCard adds a new credit card to customer account
	MockAddCard func(in *v1.AddCardRequest) (*v1.EmptyResponse, error)
	// MockRemoveCardRequest removes a credit card from tenant account
	MockRemoveCard func(in *v1.RemoveCardRequest) (*v1.EmptyResponse, error)
	// MockUpdateCard updates tenant credit card
	MockUpdateCard func(in *v1.UpdateCardRequest) (*v1.EmptyResponse, error)
	// MockSendAccountRecoveryLink sends an email with a recovery link to user.
	MockSendAccountRecoveryLink func() (*v1.EmptyResponse, error)
	// MockSendAccountLocked sends an email notifying user their account was locked.
	MockSendAccountLocked func() (*v1.EmptyResponse, error)
	// MockSendAccountRecovered sends an email notifying user their account was successfully recovered.
	MockSendAccountRecovered func() (*v1.EmptyResponse, error)
	// MockGetAccountUpgradeWindowStartHour returns tenant account upgrade window start
	MockGetAccountUpgradeWindowStartHour func() (*v1.GetAccountUpgradeWindowStartHourResponse, error)
	// MockUpdateAccountUpgradeWindowStartHour updates tenant account upgrade window start
	MockUpdateAccountUpgradeWindowStartHour func() (*v1.EmptyResponse, error)
	// MockGetFeatures returns the subscription features
	MockGetFeatures func(context.Context, *v1.EmptyRequest) (*v1.GetFeaturesResponse, error)
	// MockGetBillingSummaryInformation returns the users Billing Summary Information
	MockGetBillingSummaryInformation func(context.Context, *v1.EmptyRequest, ...grpc.CallOption) (*v1.GetBillingSummaryInformationResponse, error)
	// MockGetPaymentsInvoicesInformation returns users the Payments Invoices Information
	MockGetPaymentsInvoicesInformation func(context.Context, *v1.EmptyRequest, ...grpc.CallOption) (*v1.GetPaymentsInvoicesInformationResponse, error)
	// MockGetInvoiceSettingsInformation returns the users Invoice Settings Information
	MockGetInvoiceSettingsInformation func(context.Context, *v1.EmptyRequest, ...grpc.CallOption) (*v1.GetInvoiceSettingsInformationResponse, error)
	// MockUpdateStripeAddress updates customer address information in Stripe
	MockUpdateStripeAddress func(context.Context, *v1.StripeBillingAddressRequest, ...grpc.CallOption) (*v1.EmptyResponse, error)
	// MockUpdateEmail updates email address information
	MockUpdateEmail func(ctx context.Context, in *v1.UpdateEmailRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error)
	// MockUpdatePurchaseOrderPrefix updates purchase order prefix
	MockUpdatePurchaseOrderPrefix func(ctx context.Context, in *v1.UpdatePurchaseOrderPrefixRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error)
	// MockCancelSubscription cancels the customers subscription
	MockCancelSubscription func(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error)
	// GetSurveyCompany returns the company survey responses for the account associated with the current user.
	// These are answered by only the first user who completes the survey
	MockGetSurveyCompany func(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.SurveyCompanyResponse, error)
	// MockSetSurveyResults updates the account object with onboarding survey results
	MockSetSurveyResults func(ctx context.Context, in *v1.SetSurveyResultsRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error)
	// MockSendTeleportInvite sends a Teleport invite to a new user in an existing cluster
	MockSendTeleportInvite func(ctx context.Context, req *v1.SendTeleportInviteRequest) (*v1.EmptyResponse, error)
}

func (m *MockedClient) SubmitUsageReports(ctx context.Context, in *v1.SubmitUsageReportsRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockSubmitUsageReports != nil {
		return m.MockSubmitUsageReports(in)
	}

	return nil, trace.NotImplemented("SubmitUsageReports is not implemented")
}

func (m *MockedClient) GetBillingInformation(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.GetBillingInformationResponse, error) {
	if m.MockGetBillingInformation != nil {
		return m.MockGetBillingInformation()
	}

	return nil, trace.NotImplemented("GetBillingInformation is not implemented")
}

func (m *MockedClient) CreateSetupIntent(ctx context.Context, in *v1.EmptyRequest, _ ...grpc.CallOption) (*v1.CreateSetupIntentResponse, error) {
	if m.MockCreateSetupIntent != nil {
		return m.MockCreateSetupIntent(ctx, in)
	}

	return nil, trace.NotImplemented("CreateSetupIntent is not implemented")
}

func (m *MockedClient) AddCard(ctx context.Context, in *v1.AddCardRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockAddCard != nil {
		return m.MockAddCard(in)
	}

	return nil, trace.NotImplemented("AddCard is not implemented")
}

func (m *MockedClient) RemoveCard(ctx context.Context, in *v1.RemoveCardRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockRemoveCard != nil {
		return m.MockRemoveCard(in)
	}

	return nil, trace.NotImplemented("RemoveCard is not implemented")
}

func (m *MockedClient) UpdateCard(ctx context.Context, in *v1.UpdateCardRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockUpdateCard != nil {
		return m.MockUpdateCard(in)
	}

	return nil, trace.NotImplemented("UpdateCard is not implemented")
}

func (m *MockedClient) SendAccountRecoveryLink(ctx context.Context, in *v1.SendAccountRecoveryLinkRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockSendAccountRecoveryLink != nil {
		return m.MockSendAccountRecoveryLink()
	}

	return nil, trace.NotImplemented("SendAccountRecoveryLink is not implemented")
}

func (m *MockedClient) SendAccountLocked(ctx context.Context, in *v1.SendAccountLockedRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockSendAccountLocked != nil {
		return m.MockSendAccountLocked()
	}

	return nil, trace.NotImplemented("SendAccountLocked is not implemented")
}

func (m *MockedClient) SendAccountRecovered(ctx context.Context, in *v1.SendAccountRecoveredRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockSendAccountRecovered != nil {
		return m.MockSendAccountRecovered()
	}

	return nil, trace.NotImplemented("SendAccountRecovered is not implemented")
}

func (m *MockedClient) GetAccountUpgradeWindowStartHour(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.GetAccountUpgradeWindowStartHourResponse, error) {
	if m.MockGetAccountUpgradeWindowStartHour != nil {
		return m.MockGetAccountUpgradeWindowStartHour()
	}

	return nil, trace.NotImplemented("GetAccountUpgradeWindowStartHour is not implemented")
}

func (m *MockedClient) UpdateAccountUpgradeWindowStartHour(ctx context.Context, in *v1.UpdateAccountUpgradeWindowStartHourRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockUpdateAccountUpgradeWindowStartHour != nil {
		return m.MockUpdateAccountUpgradeWindowStartHour()
	}

	return nil, trace.NotImplemented("MockUpdateAccountUpgradeWindowStartHour is not implemented")
}

func (m *MockedClient) GetFeatures(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.GetFeaturesResponse, error) {
	if m.MockGetFeatures != nil {
		return m.MockGetFeatures(ctx, in)
	}

	return nil, trace.NotImplemented("MockGetFeatures is not implemented")
}

func (m *MockedClient) GetBillingSummaryInformation(ctx context.Context, in *v1.EmptyRequest, _ ...grpc.CallOption) (*v1.GetBillingSummaryInformationResponse, error) {
	if m.MockGetBillingSummaryInformation != nil {
		return m.MockGetBillingSummaryInformation(ctx, in)
	}

	return nil, trace.NotImplemented("GetBillingSummaryInformation is not implemented")
}

func (m *MockedClient) GetPaymentsInvoicesInformation(ctx context.Context, in *v1.EmptyRequest, _ ...grpc.CallOption) (*v1.GetPaymentsInvoicesInformationResponse, error) {
	if m.MockGetPaymentsInvoicesInformation != nil {
		return m.MockGetPaymentsInvoicesInformation(ctx, in)
	}

	return nil, trace.NotImplemented("GetPaymentsInvoicesInformation is not implemented")
}

func (m *MockedClient) GetInvoiceSettingsInformation(ctx context.Context, in *v1.EmptyRequest, _ ...grpc.CallOption) (*v1.GetInvoiceSettingsInformationResponse, error) {
	if m.MockGetInvoiceSettingsInformation != nil {
		return m.MockGetInvoiceSettingsInformation(ctx, in)
	}

	return nil, trace.NotImplemented("GetInvoiceSettingsInformation is not implemented")
}

func (m *MockedClient) UpdateStripeAddress(ctx context.Context, in *v1.StripeBillingAddressRequest, _ ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockUpdateStripeAddress != nil {
		return m.MockUpdateStripeAddress(ctx, in)
	}

	return nil, trace.NotImplemented("UpdateStripeAddress is not implemented")
}

func (m *MockedClient) UpdateEmail(ctx context.Context, in *v1.UpdateEmailRequest, _ ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockUpdateEmail != nil {
		return m.MockUpdateEmail(ctx, in)
	}

	return nil, trace.NotImplemented("UpdateEmail is not implemented")
}

func (m *MockedClient) UpdatePurchaseOrderPrefix(ctx context.Context, in *v1.UpdatePurchaseOrderPrefixRequest, _ ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockUpdatePurchaseOrderPrefix != nil {
		return m.MockUpdatePurchaseOrderPrefix(ctx, in)
	}

	return nil, trace.NotImplemented("UpdatePurchaseOrderPrefix is not implemented")
}

func (m *MockedClient) CancelSubscription(ctx context.Context, in *v1.EmptyRequest, _ ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockCancelSubscription != nil {
		return m.MockCancelSubscription(ctx, in)
	}

	return nil, trace.NotImplemented("CancelSubscription is not implemented")
}

func (m *MockedClient) GetSurveyCompany(ctx context.Context, in *v1.EmptyRequest, _ ...grpc.CallOption) (*v1.SurveyCompanyResponse, error) {
	if m.MockGetSurveyCompany != nil {
		return m.MockGetSurveyCompany(ctx, in)
	}

	return nil, trace.NotImplemented("GetSurveyCompany is not implemented")
}

func (m *MockedClient) SetSurveyResults(ctx context.Context, in *v1.SetSurveyResultsRequest, _ ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockSetSurveyResults != nil {
		return m.MockSetSurveyResults(ctx, in)
	}

	return nil, trace.NotImplemented("SetSurveyResults is not implemented")
}

func (m *MockedClient) SendTeleportInvite(ctx context.Context, req *v1.SendTeleportInviteRequest, _ ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockSendTeleportInvite != nil {
		return m.MockSendTeleportInvite(ctx, req)
	}

	return nil, trace.NotImplemented("SendTeleportInvite is not implemented")
}
