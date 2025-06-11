package notification

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/lib/accessmonitoring"
	"github.com/gravitational/trace"
)

const (
	handlerName = "test-handler"
)

func TestHandleAccessRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	const (
		testRole          = "test-role"
		testRequesterName = "requester@goteleport.com"
		testRuleName      = "test-rule"
		testRecipient     = "reviewer@goteleport.com"

		withSecretsFalse = false
	)

	testReqID := uuid.New().String()

	// Setup test rule
	cache := accessmonitoring.NewCache()
	rule := newNotificationRule(testRuleName,
		fmt.Sprintf(`contains_all(set("%s"), access_request.spec.roles)`, testRole),
		[]string{testRecipient},
	)
	cache.Put([]*accessmonitoringrulesv1.AccessMonitoringRule{rule})

	// Setup test user
	testRequester, err := types.NewUser(testRequesterName)
	require.NoError(t, err)

	tests := []struct {
		description       string
		requester         string
		requestedRole     string
		setupMockClient   func(m *mockClient)
		setupMockNotifier func(m *mockNotifier)
		assertErr         require.ErrorAssertionFunc
	}{
		{
			description:   "test role",
			requester:     testRequesterName,
			requestedRole: testRole,
			setupMockClient: func(m *mockClient) {
				m.On("GetUser", mock.Anything, testRequesterName, withSecretsFalse).
					Return(testRequester, nil)
				m.On("FetchRecipient", mock.Anything, testRecipient).
					Return(&common.Recipient{}, nil)
			},
			assertErr: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			client := &mockClient{}
			if test.setupMockClient != nil {
				test.setupMockClient(client)
			}

			notifier := &mockNotifier{}
			if test.setupMockNotifier != nil {
				test.setupMockNotifier(notifier)
			}

			handler, err := NewHandler(Config{
				Client:   client,
				Cache:    cache,
				Notifier: notifier,
			})
			require.NoError(t, err)

			req, err := types.NewAccessRequest(testReqID, test.requester, test.requestedRole)
			require.NoError(t, err)

			test.assertErr(t, handler.HandleAccessRequest(ctx, types.Event{
				Type:     types.OpPut,
				Resource: req,
			}))

			client.AssertExpectations(t)
		})
	}
}

func TestGetRecipients(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	const (
		testRuleName           = "test-rule"
		testRequestedRoleName  = "requested-role"
		testRequesterName      = "requester@goteleport.com"
		testReviewerName       = "reviewer@goteleport.com"
		testStaticReviewerName = "static-reviewer@goteleport.com"

		withSecretsFalse = false
	)

	testReqID := uuid.New().String()

	// Setup access monitoring rule recipients
	cache := accessmonitoring.NewCache()
	rule := newNotificationRule(testRuleName,
		fmt.Sprintf(`contains_all(set("%s"), access_request.spec.roles)`, testRequestedRoleName),
		[]string{testReviewerName},
	)
	cache.Put([]*accessmonitoringrulesv1.AccessMonitoringRule{rule})

	// Setup static recipients
	staticRecipients := common.RawRecipientsMap{
		"*": []string{testStaticReviewerName},
	}

	// Setup mock Teleport client
	client := &mockClient{}

	// Setup requester user
	testRequester, err := types.NewUser(testRequesterName)
	require.NoError(t, err)

	client.On("GetUser", mock.Anything, testRequesterName, withSecretsFalse).
		Return(testRequester, nil)

		// Setup requested role
	testRole, err := types.NewRole(testRequestedRoleName, types.RoleSpecV6{})
	require.NoError(t, err)

	client.On("GetRole", mock.Anything, testRequestedRoleName).
		Return(testRole, nil)

	handler := &Handler{
		Config: Config{
			Client:           client,
			StaticRecipients: staticRecipients,
		},
		rules: cache,
	}

	tests := []struct {
		description        string
		requester          string
		requestedRole      string
		expectedRecipients []string
	}{
		{
			description:        "access monitoring rule recipients",
			requester:          testRequesterName,
			requestedRole:      testRequestedRoleName,
			expectedRecipients: []string{testRequesterName, testReviewerName},
		},
		{
			description:        "static recipients",
			requester:          testRequesterName,
			requestedRole:      "non-rule-matching-role",
			expectedRecipients: []string{testRequesterName, testStaticReviewerName},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			req, err := types.NewAccessRequest(testReqID, test.requester, test.requestedRole)
			require.NoError(t, err)

			recipients, err := handler.getRecipients(ctx, req)
			require.NoError(t, err)
			require.ElementsMatch(t, test.expectedRecipients, recipients)
		})
	}
}

func TestGetResourceNames(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	const (
		testClusterName = "test-cluster"
		testNodeName    = "test-node"
		testAppName     = "test-app"
	)

	tests := []struct {
		description       string
		requestedResource types.ResourceID
		expectedResource  string
		setupMockClient   func(m *mockClient)
	}{
		{
			description:      "node resource",
			expectedResource: "node/test-node",
			requestedResource: types.ResourceID{
				ClusterName: testClusterName,
				Kind:        types.KindNode,
				Name:        testNodeName,
			},
			setupMockClient: func(m *mockClient) {
				node, err := types.NewNode(
					testNodeName,
					types.SubKindTeleportNode,
					types.ServerSpecV2{Hostname: testNodeName},
					nil,
				)
				require.NoError(t, err)

				m.On("ListResources", mock.Anything, mock.Anything).
					Return(
						&types.ListResourcesResponse{
							Resources:  []types.ResourceWithLabels{node},
							TotalCount: 1,
						},
						nil,
					)
			},
		},
		{
			description:      "app resource",
			expectedResource: "/test-cluster/app/test-app",
			requestedResource: types.ResourceID{
				ClusterName: testClusterName,
				Kind:        types.KindApp,
				Name:        testAppName,
			},
			setupMockClient: func(m *mockClient) {
				app, err := types.NewAppV3(
					types.Metadata{Name: testAppName},
					types.AppSpecV3{URI: "tcp://localhost"},
				)
				require.NoError(t, err)

				m.On("ListResources", mock.Anything, mock.Anything).
					Return(
						&types.ListResourcesResponse{
							Resources:  []types.ResourceWithLabels{app},
							TotalCount: 1,
						},
						nil,
					)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			client := &mockClient{}
			if test.setupMockClient != nil {
				test.setupMockClient(client)
			}

			handler := &Handler{
				Config: Config{
					Client: client,
				},
			}

			testReqID := uuid.New().String()
			requestedResources := []types.ResourceID{test.requestedResource}
			req, err := types.NewAccessRequestWithResources(testReqID, "test-user", nil, requestedResources)
			require.NoError(t, err)

			resources, err := handler.getResourceNames(ctx, req)
			require.NoError(t, err)
			require.ElementsMatch(t, []string{test.expectedResource}, resources)
		})
	}
}

func TestGetLoginsByRole(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	const (
		testUserName   = "test-user"
		testAdminRole  = "admin"
		testAccessRole = "access"
		testDevRole    = "dev"
	)

	tests := []struct {
		description          string
		requestedRoles       []string
		expectedLoginsByRole map[string][]string
		setupMockClient      func(m *mockClient)
	}{
		{
			description:    "apply traits",
			requestedRoles: []string{testAdminRole},
			expectedLoginsByRole: map[string][]string{
				testAdminRole: {"root", "foo", "bar", "buz"},
			},
			setupMockClient: func(m *mockClient) {
				user, err := types.NewUser(testUserName)
				require.NoError(t, err)
				user.SetTraits(map[string][]string{"logins": {"buz"}})

				m.On("GetUser", mock.Anything, testUserName, mock.Anything).
					Return(user, nil)

				role, err := types.NewRole(testAdminRole, types.RoleSpecV6{})
				require.NoError(t, err)
				role.SetLogins(types.Allow, []string{"root", "foo", "bar", "{{internal.logins}}"})

				m.On("GetRole", mock.Anything, testAdminRole).
					Return(role, nil)
			},
		},
		{
			description:    "multiple roles",
			requestedRoles: []string{testAccessRole, testDevRole},
			expectedLoginsByRole: map[string][]string{
				testAccessRole: {"{{internal.logins}}"},
				testDevRole:    {"foo", "bar"},
			},
			setupMockClient: func(m *mockClient) {
				user, err := types.NewUser(testUserName)
				require.NoError(t, err)

				m.On("GetUser", mock.Anything, testUserName, mock.Anything).
					Return(user, nil)

				// Mock access role request
				accessRole, err := types.NewRole(testAccessRole, types.RoleSpecV6{})
				require.NoError(t, err)
				accessRole.SetLogins(types.Allow, []string{"{{internal.logins}}"})

				m.On("GetRole", mock.Anything, testAccessRole).
					Return(accessRole, nil)

				// Mock dev role request
				devRole, err := types.NewRole(testDevRole, types.RoleSpecV6{})
				require.NoError(t, err)
				devRole.SetLogins(types.Allow, []string{"foo", "bar"})

				m.On("GetRole", mock.Anything, testDevRole).
					Return(devRole, nil)
			},
		},
		{
			description:    "empty logins",
			requestedRoles: []string{testDevRole},
			expectedLoginsByRole: map[string][]string{
				testDevRole: {},
			},
			setupMockClient: func(m *mockClient) {
				user, err := types.NewUser(testUserName)
				require.NoError(t, err)

				m.On("GetUser", mock.Anything, testUserName, mock.Anything).
					Return(user, nil)

				devRole, err := types.NewRole(testDevRole, types.RoleSpecV6{})
				require.NoError(t, err)

				m.On("GetRole", mock.Anything, testDevRole).
					Return(devRole, nil)
			},
		},
		{
			description:    "missing user.read permissions",
			requestedRoles: []string{testAdminRole},
			expectedLoginsByRole: map[string][]string{
				testAdminRole: {"root", "foo", "bar", "{{internal.logins}}"},
			},
			setupMockClient: func(m *mockClient) {
				m.On("GetUser", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, trace.AccessDenied("missing user.read"))

				role, err := types.NewRole(testAdminRole, types.RoleSpecV6{})
				require.NoError(t, err)
				role.SetLogins(types.Allow, []string{"root", "foo", "bar", "{{internal.logins}}"})

				m.On("GetRole", mock.Anything, testAdminRole).
					Return(role, nil)
			},
		},
		{
			description:          "missing role.read permissions",
			requestedRoles:       []string{testAdminRole},
			expectedLoginsByRole: map[string][]string{},
			setupMockClient: func(m *mockClient) {
				m.On("GetUser", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, trace.AccessDenied("missing user.read"))

				m.On("GetRole", mock.Anything, mock.Anything).
					Return(nil, trace.AccessDenied("missing role.read"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			client := &mockClient{}
			if test.setupMockClient != nil {
				test.setupMockClient(client)
			}

			handler := &Handler{
				Config: Config{
					Logger: slog.Default(),
					Client: client,
				},
			}

			testReqID := uuid.New().String()
			req, err := types.NewAccessRequest(testReqID, testUserName, test.requestedRoles...)
			require.NoError(t, err)

			loginsByRole, err := handler.getLoginsByRole(ctx, req)
			require.NoError(t, err)
			require.Equal(t, test.expectedLoginsByRole, loginsByRole)
		})
	}
}

func newNotificationRule(name, condition string, recipients []string) *accessmonitoringrulesv1.AccessMonitoringRule {
	return &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind: types.KindAccessMonitoringRule,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:  []string{types.KindAccessRequest},
			Condition: condition,
			Notification: &accessmonitoringrulesv1.Notification{
				Name:       handlerName,
				Recipients: recipients,
			},
		},
	}
}

type mockMessage struct {
	mock.Mock
}

func (m *mockMessage) ID() string {
	args := m.Called()
	return args.String(1)
}

type mockNotifier struct {
	mock.Mock
}

func (m *mockNotifier) CheckHealth(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockNotifier) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	args := m.Called(ctx, recipient)
	fetched, ok := args.Get(0).(*common.Recipient)
	if ok {
		return fetched, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockNotifier) NotifyReviewer(ctx context.Context, recipient *common.Recipient, ar pd.AccessRequestData) (Message, error) {
	args := m.Called(ctx, recipient, ar)
	sent, ok := args.Get(0).(*mockMessage)
	if ok {
		return sent, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockNotifier) NotifyRequester(ctx context.Context, recipient *common.Recipient, ar pd.AccessRequestData) (Message, error) {
	args := m.Called(ctx, recipient, ar)
	sent, ok := args.Get(0).(*mockMessage)
	if ok {
		return sent, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockNotifier) UpdateReviewer(ctx context.Context, original Message, ar pd.AccessRequestData) (Message, error) {
	args := m.Called(ctx, original, ar)
	sent, ok := args.Get(0).(*mockMessage)
	if ok {
		return sent, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockNotifier) UpdateRequester(ctx context.Context, original Message, ar pd.AccessRequestData) (Message, error) {
	args := m.Called(ctx, original, ar)
	sent, ok := args.Get(0).(*mockMessage)
	if ok {
		return sent, args.Error(1)
	}
	return nil, args.Error(1)
}

// mockClient is a mock implementation of the Teleport API client.
type mockClient struct {
	mock.Mock
}

func (m *mockClient) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	args := m.Called(ctx, name, withSecrets)
	user, ok := args.Get(0).(types.User)
	if ok {
		return user, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	args := m.Called(ctx, name)
	role, ok := args.Get(0).(types.Role)
	if ok {
		return role, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockClient) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	args := m.Called(ctx, req)
	response, ok := args.Get(0).(*types.ListResourcesResponse)
	if ok {
		return response, args.Error(1)
	}
	return nil, args.Error(1)
}
