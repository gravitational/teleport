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
