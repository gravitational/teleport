package cloud

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
)

// MockedClient mocks cloud client APIs
type MockedClient struct {
	// SubmitUsageReports reports usage
	MockSubmitUsageReports func(in *v1.SubmitUsageReportsRequest) (*v1.EmptyResponse, error)
	// ListInvoices lists customer invoices
	MockListInvoices func() (*v1.ListInvoicesResponse, error)
	// GetBillingInformation returns customer billing information
	MockGetBillingInformation func() (*v1.GetBillingInformationResponse, error)
	// AddCard adds a new credit card to customer account
	MockAddCard func(in *v1.AddCardRequest) (*v1.EmptyResponse, error)
	// RemoveCardRequest removes a credit card from tenant account
	MockRemoveCard func(in *v1.RemoveCardRequest) (*v1.EmptyResponse, error)
	// UpdateCard updates tenant credit card
	MockUpdateCard func(in *v1.UpdateCardRequest) (*v1.EmptyResponse, error)
	// UpdateAccount updates tenant account information
	MockUpdateAccount func(in *v1.UpdateAccountRequest) (*v1.EmptyResponse, error)
	// ListBillingCycles lists tenant billing cycles
	MockListBillingCycles func() (*v1.ListBillingCyclesResponse, error)
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
}

func (m *MockedClient) SubmitUsageReports(ctx context.Context, in *v1.SubmitUsageReportsRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockSubmitUsageReports != nil {
		return m.MockSubmitUsageReports(in)
	}

	return nil, trace.NotImplemented("SubmitUsageReports is not implemented")
}

// ListInvoices lists customer invoices
func (m *MockedClient) ListInvoices(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.ListInvoicesResponse, error) {
	if m.MockListInvoices != nil {
		return m.MockListInvoices()
	}

	return nil, trace.NotImplemented("ListInvoices is not implemented")
}

func (m *MockedClient) GetBillingInformation(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.GetBillingInformationResponse, error) {
	if m.MockGetBillingInformation != nil {
		return m.MockGetBillingInformation()
	}

	return nil, trace.NotImplemented("GetBillingInformation is not implemented")
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

func (m *MockedClient) UpdateAccount(ctx context.Context, in *v1.UpdateAccountRequest, opts ...grpc.CallOption) (*v1.EmptyResponse, error) {
	if m.MockUpdateAccount != nil {
		return m.MockUpdateAccount(in)
	}

	return nil, trace.NotImplemented("UpdateAccount is not implemented")
}

func (m *MockedClient) ListBillingCycles(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.ListBillingCyclesResponse, error) {
	if m.MockListBillingCycles != nil {
		return m.MockListBillingCycles()
	}

	return nil, trace.NotImplemented("ListBillingCycles is not implemented")
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
