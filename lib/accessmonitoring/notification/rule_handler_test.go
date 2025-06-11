package notification

import (
	"context"
	"testing"
	"time"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/accessmonitoring"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pb "google.golang.org/protobuf/proto"
)

func TestInitializeCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	mockClient := &mockRuleHandlerClient{}
	cache := accessmonitoring.NewCache()
	handler, err := NewRuleHandler(RuleHandlerConfig{
		HandlerName: handlerName,
		Client:      mockClient,
		Cache:       cache,
	})
	require.NoError(t, err)

	mockReq := &accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest{
		Subjects:         []string{types.KindAccessRequest},
		NotificationName: handler.HandlerName,
	}

	mockResp := []*accessmonitoringrulesv1.AccessMonitoringRule{
		newNotificationRule("test-rule", "condition", []string{"test-recipient"}),
	}

	mockClient.On("ListAccessMonitoringRulesWithFilter", mock.Anything, mockReq).
		Return(mockResp, "", nil)

	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type: types.OpInit,
		Resource: types.NewWatchStatus([]types.WatchKind{
			{Kind: types.KindAccessMonitoringRule},
		}),
	}))
	require.Len(t, cache.Get(), 1)
	require.ElementsMatch(t, mockResp, cache.Get())
}

func TestHandleAccessMonitoringRule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	mockClient := &mockRuleHandlerClient{}
	cache := accessmonitoring.NewCache()
	handler, err := NewRuleHandler(RuleHandlerConfig{
		HandlerName: handlerName,
		Client:      mockClient,
		Cache:       cache,
	})
	require.NoError(t, err)

	// Test add rule.
	rule := newNotificationRule("test-rule", "condition", []string{"test-recipient"})
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Len(t, cache.Get(), 1)
	require.True(t, pb.Equal(rule, cache.Get()[0]))

	// Test update rule.
	rule = newNotificationRule("test-rule", "condition-updated", []string{"test-recipient"})
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Len(t, cache.Get(), 1)
	require.True(t, pb.Equal(rule, cache.Get()[0]))

	// Test delete rule.
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpDelete,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Empty(t, cache.Get())

	// Test rule does not apply with invalid automatic approval name.
	rule = newNotificationRule("test-rule", "condition", []string{"test-recipient"})
	rule.Spec.Notification.Name = "invalid"
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Empty(t, cache.Get())

	// Test rule does not apply with invalid subject.
	rule = newNotificationRule("test-rule", "condition", []string{"test-recipient"})
	rule.Spec.Subjects = []string{"invalid"}
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Empty(t, cache.Get())
}

// mockClient is a mock implementation of the Teleport API client.
type mockRuleHandlerClient struct {
	mock.Mock
}

func (m *mockRuleHandlerClient) ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) (
	[]*accessmonitoringrulesv1.AccessMonitoringRule,
	string,
	error,
) {
	// Expect zero value page size for testing.
	req.PageSize = 0

	args := m.Called(ctx, req)
	rules, ok := args.Get(0).([]*accessmonitoringrulesv1.AccessMonitoringRule)
	if ok {
		return rules, args.String(1), args.Error(2)
	}
	return nil, args.String(1), args.Error(2)
}
