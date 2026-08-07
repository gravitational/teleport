package cloud

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
)

// MockedClient mocks cloud client APIs
type MockedClient struct {
	v1.TenantsServiceClient

	// MockSubmitUsageReports reports usage
	MockSubmitUsageReports func(in *v1.SubmitUsageReportsRequest) (*v1.EmptyResponse, error)
	// MockGetBillingInformation returns customer billing information
	MockGetBillingInformation func() (*v1.GetBillingInformationResponse, error)
	// MockSendAccountRecoveryLink sends an email with a recovery link to user.
	MockSendAccountRecoveryLink func() (*v1.EmptyResponse, error)
	// MockSendAccountLocked sends an email notifying user their account was locked.
	MockSendAccountLocked func() (*v1.EmptyResponse, error)
	// MockSendAccountRecovered sends an email notifying user their account was successfully recovered.
	MockSendAccountRecovered func() (*v1.EmptyResponse, error)
	// MockGetAccountUpgradeWindowStartHour returns tenant account upgrade window start
	MockGetAccountUpgradeWindowStartHour func() (*v1.GetAccountUpgradeWindowStartHourResponse, error)
	// MockUpdateAccountUpgradeWindowStartHour updates tenant account upgrade window start
	MockUpdateAccountUpgradeWindowStartHour func(in *v1.UpdateAccountUpgradeWindowStartHourRequest) (*v1.EmptyResponse, error)
	// MockGetFeatures returns the subscription features
	MockGetFeatures func(context.Context, *v1.EmptyRequest) (*v1.GetFeaturesResponse, error)
	// MockGetBillingSummaryInformation returns the users Billing Summary Information
	MockGetBillingSummaryInformation func(context.Context, *v1.EmptyRequest, ...grpc.CallOption) (*v1.GetBillingSummaryInformationResponse, error)
	// MockGetUsage returns usage information for one or more tenants.
	MockGetUsage func(ctx context.Context, in *v1.GetUsageRequest, opts ...grpc.CallOption) (*v1.GetUsageResponse, error)
	// GetSurveyCompany returns the company survey responses for the account associated with the current user.
	// These are answered by only the first user who completes the survey
	MockGetSurveyCompany func(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.SurveyCompanyResponse, error)
	// MockSetSurveyResults updates the account object with onboarding survey results
	MockSetSurveyResults func(ctx context.Context, in *v1.SetSurveyResultsRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error)
	// MockSendTeleportInvite sends a Teleport invite to a new user in an existing cluster
	MockSendTeleportInvite func(ctx context.Context, req *v1.SendTeleportInviteRequest) (*v1.EmptyResponse, error)
	// MockClusterAlertInfo is a mock implementation of ClusterAlertInfo which returns information about a cluster that
	// will determine if the Teleport usage reporter should generate a cluster alert
	MockClusterAlertInfo func(context.Context, *v1.EmptyRequest) (*v1.ClusterAlertInfoResponse, error)
	// MockGetUpdatedLicense returns the customer's license if it is different from the license
	// provided as the mTLS peer certificate.
	MockGetUpdatedLicense func(ctx context.Context, req *v1.GetUpdatedLicenseRequest, opts ...grpc.CallOption) (*v1.GetUpdatedLicenseResponse, error)
	// MockGetContacts returns the customer's contacts
	MockGetContacts func(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.GetContactsResponse, error)
	// MockCreateContact creates a new contact
	MockCreateContact func(ctx context.Context, req *v1.CreateContactRequest, opts ...grpc.CallOption) (*v1.CreateContactResponse, error)
	// MockRemoveContact removes a contact type from a contact. If the contact has no other
	// flags set, the contact itself will be removed.
	MockRemoveContact func(ctx context.Context, in *v1.RemoveContactRequest, opts ...grpc.CallOption) (*v1.RemoveContactResponse, error)
	// MockGetFile returns a static file to be rendered on the Teleport UI.
	MockGetFile func(ctx context.Context, in *v1.GetFileRequest, opts ...grpc.CallOption) (v1.TenantsService_GetFileClient, error)
	// MockGetMAUDailyBreakdown returns a per-day MAU breakdown.
	MockGetMAUDailyBreakdown func(ctx context.Context, in *v1.GetMAUDailyBreakdownRequest, opts ...grpc.CallOption) (*v1.GetMAUDailyBreakdownResponse, error)
	// MockGetTPRDailyBreakdown returns a per-day TPR breakdown.
	MockGetTPRDailyBreakdown func(ctx context.Context, in *v1.GetTPRDailyBreakdownRequest, opts ...grpc.CallOption) (*v1.GetTPRDailyBreakdownResponse, error)
	// MockGetEnvironmentProfile returns the tenant environment profile
	MockGetEnvironmentProfile func() (*v1.GetEnvironmentProfileResponse, error)
	// MockUpdateEnvironmentProfile updates the tenant environment profile
	MockUpdateEnvironmentProfile func(in *v1.UpdateEnvironmentProfileRequest) (*v1.GetEnvironmentProfileResponse, error)
	// MockGetStripeConfig returns the Stripe publishable key and customer id.
	MockGetStripeConfig func(ctx context.Context, in *v1.GetStripeConfigRequest, opts ...grpc.CallOption) (*v1.GetStripeConfigResponse, error)
	// MockStripeListCards lists the customer's saved Stripe cards.
	MockStripeListCards func(ctx context.Context, in *v1.StripeListCardsRequest, opts ...grpc.CallOption) (*v1.StripeListCardsResponse, error)
	// MockStripeCreateSetupIntent creates a Stripe SetupIntent for attaching a new card.
	MockStripeCreateSetupIntent func(ctx context.Context, in *v1.StripeCreateSetupIntentRequest, opts ...grpc.CallOption) (*v1.StripeCreateSetupIntentResponse, error)
	// MockStripeCreateCard attaches a new card to the customer.
	MockStripeCreateCard func(ctx context.Context, in *v1.StripeCreateCardRequest, opts ...grpc.CallOption) (*v1.StripeCreateCardResponse, error)
	// MockStripeUpdateCard swaps the customer's card, and can set the default source.
	MockStripeUpdateCard func(ctx context.Context, in *v1.StripeUpdateCardRequest, opts ...grpc.CallOption) (*v1.StripeUpdateCardResponse, error)
	// MockStripeDeleteCard removes a Stripe card from the customer.
	MockStripeDeleteCard func(ctx context.Context, in *v1.StripeDeleteCardRequest, opts ...grpc.CallOption) (*v1.StripeDeleteCardResponse, error)
	// MockStripeListInvoices lists the customer's Stripe invoices.
	MockStripeListInvoices func(ctx context.Context, in *v1.StripeListInvoicesRequest, opts ...grpc.CallOption) (*v1.StripeListInvoicesResponse, error)
	// MockStripeGetSettings returns the customer's invoice settings.
	MockStripeGetSettings func(ctx context.Context, in *v1.StripeGetSettingsRequest, opts ...grpc.CallOption) (*v1.StripeGetSettingsResponse, error)
	// MockStripeUpdateEmail updates the invoice email.
	MockStripeUpdateEmail func(ctx context.Context, in *v1.StripeUpdateEmailRequest, opts ...grpc.CallOption) (*v1.StripeUpdateEmailResponse, error)
	// MockStripeUpdatePOPrefix updates the invoice purchase-order prefix.
	MockStripeUpdatePOPrefix func(ctx context.Context, in *v1.StripeUpdatePOPrefixRequest, opts ...grpc.CallOption) (*v1.StripeUpdatePOPrefixResponse, error)
	// MockStripeUpdateStripeAddress updates the customer name and billing address.
	MockStripeUpdateStripeAddress func(ctx context.Context, in *v1.StripeUpdateStripeAddressRequest, opts ...grpc.CallOption) (*v1.StripeUpdateStripeAddressResponse, error)
	// MockStripeCancel cancels the Stripe subscription.
	MockStripeCancel func(ctx context.Context, in *v1.StripeCancelRequest, opts ...grpc.CallOption) (*v1.StripeCancelResponse, error)
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
		return m.MockUpdateAccountUpgradeWindowStartHour(in)
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

func (m *MockedClient) GetUsage(ctx context.Context, in *v1.GetUsageRequest, _ ...grpc.CallOption) (*v1.GetUsageResponse, error) {
	if m.MockGetUsage != nil {
		return m.MockGetUsage(ctx, in)
	}

	return nil, trace.NotImplemented("GetUsage is not implemented")
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

func (m *MockedClient) ClusterAlertInfo(ctx context.Context, in *v1.EmptyRequest, _ ...grpc.CallOption) (*v1.ClusterAlertInfoResponse, error) {
	if m.MockClusterAlertInfo != nil {
		return m.MockClusterAlertInfo(ctx, in)
	}

	return nil, trace.NotImplemented("ClusterAlertInfo is not implemented")
}

func (m *MockedClient) GetUpdatedLicense(ctx context.Context, req *v1.GetUpdatedLicenseRequest, _ ...grpc.CallOption) (*v1.GetUpdatedLicenseResponse, error) {
	if m.MockGetUpdatedLicense != nil {
		return m.MockGetUpdatedLicense(ctx, req)
	}

	return nil, trace.NotImplemented("GetUpdatedLicense is not implemented")
}

// CreateContact calls MockCreateContact if it exists and returns trace.NotImplemented otherwise.
func (m *MockedClient) CreateContact(ctx context.Context, req *v1.CreateContactRequest, _ ...grpc.CallOption) (*v1.CreateContactResponse, error) {
	if m.MockCreateContact != nil {
		return m.MockCreateContact(ctx, req)
	}

	return nil, trace.NotImplemented("CreateContact is not implemented")
}

// RemoveContact calls MockRemoveContact if it exists and returns trace.NotImplemented otherwise.
func (m *MockedClient) RemoveContact(ctx context.Context, req *v1.RemoveContactRequest, _ ...grpc.CallOption) (*v1.RemoveContactResponse, error) {
	if m.MockRemoveContact != nil {
		return m.MockRemoveContact(ctx, req)
	}

	return nil, trace.NotImplemented("RemoveContact is not implemented")
}

// GetContacts calls MockGetContacts if it exists and returns trace.NotImplemented otherwise.
func (m *MockedClient) GetContacts(ctx context.Context, req *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.GetContactsResponse, error) {
	if m.MockGetContacts != nil {
		return m.MockGetContacts(ctx, req)
	}

	return nil, trace.NotImplemented("GetContacts is not implemented")
}

// GetFile calls MockGetFile if it exists and returns trace.NotImplemented otherwise.
func (m *MockedClient) GetFile(ctx context.Context, in *v1.GetFileRequest, opts ...grpc.CallOption) (v1.TenantsService_GetFileClient, error) {
	if m.MockGetFile != nil {
		return m.MockGetFile(ctx, in)
	}

	return nil, trace.NotImplemented("GetFile is not implemented")
}

// GetMAUDailyBreakdown calls MockGetMAUDailyBreakdown if it exists and returns trace.NotImplemented otherwise.
func (m *MockedClient) GetMAUDailyBreakdown(ctx context.Context, in *v1.GetMAUDailyBreakdownRequest, opts ...grpc.CallOption) (*v1.GetMAUDailyBreakdownResponse, error) {
	if m.MockGetMAUDailyBreakdown != nil {
		return m.MockGetMAUDailyBreakdown(ctx, in, opts...)
	}

	return nil, trace.NotImplemented("GetMAUDailyBreakdown is not implemented")
}

// GetTPRDailyBreakdown calls MockGetTPRDailyBreakdown if it exists and returns trace.NotImplemented otherwise.
func (m *MockedClient) GetTPRDailyBreakdown(ctx context.Context, in *v1.GetTPRDailyBreakdownRequest, opts ...grpc.CallOption) (*v1.GetTPRDailyBreakdownResponse, error) {
	if m.MockGetTPRDailyBreakdown != nil {
		return m.MockGetTPRDailyBreakdown(ctx, in, opts...)
	}

	return nil, trace.NotImplemented("GetTPRDailyBreakdown is not implemented")
}

func (m *MockedClient) GetEnvironmentProfile(ctx context.Context, in *v1.GetEnvironmentProfileRequest, opts ...grpc.CallOption) (*v1.GetEnvironmentProfileResponse, error) {
	if m.MockGetEnvironmentProfile != nil {
		return m.MockGetEnvironmentProfile()
	}

	return nil, trace.NotImplemented("GetEnvironmentProfile is not implemented")
}

func (m *MockedClient) UpdateEnvironmentProfile(ctx context.Context, in *v1.UpdateEnvironmentProfileRequest, opts ...grpc.CallOption) (*v1.GetEnvironmentProfileResponse, error) {
	if m.MockUpdateEnvironmentProfile != nil {
		return m.MockUpdateEnvironmentProfile(in)
	}

	return nil, trace.NotImplemented("UpdateEnvironmentProfile is not implemented")
}

func (m *MockedClient) GetStripeConfig(ctx context.Context, in *v1.GetStripeConfigRequest, opts ...grpc.CallOption) (*v1.GetStripeConfigResponse, error) {
	if m.MockGetStripeConfig != nil {
		return m.MockGetStripeConfig(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("GetStripeConfig is not implemented")
}

func (m *MockedClient) StripeListCards(ctx context.Context, in *v1.StripeListCardsRequest, opts ...grpc.CallOption) (*v1.StripeListCardsResponse, error) {
	if m.MockStripeListCards != nil {
		return m.MockStripeListCards(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeListCards is not implemented")
}

func (m *MockedClient) StripeCreateSetupIntent(ctx context.Context, in *v1.StripeCreateSetupIntentRequest, opts ...grpc.CallOption) (*v1.StripeCreateSetupIntentResponse, error) {
	if m.MockStripeCreateSetupIntent != nil {
		return m.MockStripeCreateSetupIntent(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeCreateSetupIntent is not implemented")
}

func (m *MockedClient) StripeCreateCard(ctx context.Context, in *v1.StripeCreateCardRequest, opts ...grpc.CallOption) (*v1.StripeCreateCardResponse, error) {
	if m.MockStripeCreateCard != nil {
		return m.MockStripeCreateCard(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeCreateCard is not implemented")
}

func (m *MockedClient) StripeUpdateCard(ctx context.Context, in *v1.StripeUpdateCardRequest, opts ...grpc.CallOption) (*v1.StripeUpdateCardResponse, error) {
	if m.MockStripeUpdateCard != nil {
		return m.MockStripeUpdateCard(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeUpdateCard is not implemented")
}

func (m *MockedClient) StripeDeleteCard(ctx context.Context, in *v1.StripeDeleteCardRequest, opts ...grpc.CallOption) (*v1.StripeDeleteCardResponse, error) {
	if m.MockStripeDeleteCard != nil {
		return m.MockStripeDeleteCard(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeDeleteCard is not implemented")
}

func (m *MockedClient) StripeListInvoices(ctx context.Context, in *v1.StripeListInvoicesRequest, opts ...grpc.CallOption) (*v1.StripeListInvoicesResponse, error) {
	if m.MockStripeListInvoices != nil {
		return m.MockStripeListInvoices(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeListInvoices is not implemented")
}

func (m *MockedClient) StripeGetSettings(ctx context.Context, in *v1.StripeGetSettingsRequest, opts ...grpc.CallOption) (*v1.StripeGetSettingsResponse, error) {
	if m.MockStripeGetSettings != nil {
		return m.MockStripeGetSettings(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeGetSettings is not implemented")
}

func (m *MockedClient) StripeUpdateEmail(ctx context.Context, in *v1.StripeUpdateEmailRequest, opts ...grpc.CallOption) (*v1.StripeUpdateEmailResponse, error) {
	if m.MockStripeUpdateEmail != nil {
		return m.MockStripeUpdateEmail(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeUpdateEmail is not implemented")
}

func (m *MockedClient) StripeUpdatePOPrefix(ctx context.Context, in *v1.StripeUpdatePOPrefixRequest, opts ...grpc.CallOption) (*v1.StripeUpdatePOPrefixResponse, error) {
	if m.MockStripeUpdatePOPrefix != nil {
		return m.MockStripeUpdatePOPrefix(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeUpdatePOPrefix is not implemented")
}

func (m *MockedClient) StripeUpdateStripeAddress(ctx context.Context, in *v1.StripeUpdateStripeAddressRequest, opts ...grpc.CallOption) (*v1.StripeUpdateStripeAddressResponse, error) {
	if m.MockStripeUpdateStripeAddress != nil {
		return m.MockStripeUpdateStripeAddress(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeUpdateStripeAddress is not implemented")
}

func (m *MockedClient) StripeCancel(ctx context.Context, in *v1.StripeCancelRequest, opts ...grpc.CallOption) (*v1.StripeCancelResponse, error) {
	if m.MockStripeCancel != nil {
		return m.MockStripeCancel(ctx, in, opts...)
	}
	return nil, trace.NotImplemented("StripeCancel is not implemented")
}
