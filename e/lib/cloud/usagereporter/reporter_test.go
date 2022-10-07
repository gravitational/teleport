package usagereporter

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestUsageReport(t *testing.T) {
	m, err := createReporterMocks()
	require.Nil(t, err)

	m.apiGetters.MockedGetNodes = func() ([]types.Server, error) {
		return []types.Server{&types.ServerV2{}}, nil
	}

	m.apiGetters.MockedGetUsers = func() ([]types.User, error) {
		return []types.User{&types.UserV2{}}, nil
	}

	m.apiGetters.MockedGetApplicationServers = func() ([]types.AppServer, error) {
		return []types.AppServer{&types.AppServerV3{}}, nil
	}

	m.apiGetters.MockedGetKubeServices = func() ([]types.Server, error) {
		return []types.Server{&types.ServerV2{}}, nil
	}

	m.apiGetters.MockedGetDatabaseServers = func() ([]types.DatabaseServer, error) {
		return []types.DatabaseServer{&types.DatabaseServerV3{}}, nil
	}

	m.apiGetters.MockedGetRoles = func() ([]types.Role, error) {
		return []types.Role{&types.RoleV5{}, &types.RoleV5{}}, nil
	}

	m.apiGetters.MockedGetGithubConnectors = func() ([]types.GithubConnector, error) {
		return []types.GithubConnector{&types.GithubConnectorV3{}}, nil
	}

	m.apiGetters.MockedGetOIDCConnectors = func() ([]types.OIDCConnector, error) {
		return []types.OIDCConnector{&types.OIDCConnectorV3{}, &types.OIDCConnectorV3{}}, nil
	}

	m.apiGetters.MockedGetSAMLConnectors = func() ([]types.SAMLConnector, error) {
		return []types.SAMLConnector{&types.SAMLConnectorV2{}, &types.SAMLConnectorV2{}, &types.SAMLConnectorV2{}}, nil
	}

	var obtained []*cloudapi.UsageReport
	m.client.MockSubmitUsageReports = func(request *cloudapi.SubmitUsageReportsRequest) (*cloudapi.EmptyResponse, error) {
		obtained = request.Reports
		return nil, nil
	}

	m.reporter.reportUsage(context.TODO())

	needed := []*cloudapi.UsageReport{{
		PeriodStart: 449884799,
		PeriodEnd:   449884800,
		Items: []*cloudapi.UsageReportItem{{
			Resource: cloudapi.USER,
			Quantity: 1,
		}, {
			Resource: cloudapi.SERVER,
			Quantity: 1,
		}, {
			Resource: cloudapi.DATABASE,
			Quantity: 1,
		}, {
			Resource: cloudapi.APPLICATION,
			Quantity: 1,
		}, {
			Resource: cloudapi.KUBE_CLUSTER,
			Quantity: 1,
		}, {
			Resource: cloudapi.ROLE,
			Quantity: 2,
		}, {
			Resource: cloudapi.AUTH_CONNECTOR,
			Quantity: 6,
		}},
	}}

	require.Equal(t, obtained, needed)
}

func TestErrors(t *testing.T) {
	m, err := createReporterMocks()
	require.Nil(t, err)

	m.apiGetters.MockedGetNodes = func() ([]types.Server, error) {
		return nil, trace.BadParameter("unable to return servers")
	}

	m.apiGetters.MockedGetUsers = func() ([]types.User, error) {
		return nil, trace.BadParameter("unable to return users")
	}

	m.apiGetters.MockedGetApplicationServers = func() ([]types.AppServer, error) {
		return nil, trace.BadParameter("unable to return apps")
	}

	m.apiGetters.MockedGetKubeServices = func() ([]types.Server, error) {
		return nil, trace.BadParameter("unable to return kube servers")
	}

	m.apiGetters.MockedGetDatabaseServers = func() ([]types.DatabaseServer, error) {
		return []types.DatabaseServer{&types.DatabaseServerV3{}}, nil
	}

	m.apiGetters.MockedGetGithubConnectors = func() ([]types.GithubConnector, error) {
		return nil, trace.BadParameter("unable to return github auth connectors")
	}

	var obtained []*cloudapi.UsageReport
	m.client.MockSubmitUsageReports = func(request *cloudapi.SubmitUsageReportsRequest) (*cloudapi.EmptyResponse, error) {
		obtained = request.Reports
		return nil, nil
	}

	m.reporter.reportUsage(context.TODO())

	needed := []*cloudapi.UsageReport{{
		PeriodStart: 449884799,
		PeriodEnd:   449884800,
		Items: []*cloudapi.UsageReportItem{{
			Resource: cloudapi.USER,
			Quantity: 0,
		}, {
			Resource: cloudapi.SERVER,
			Quantity: 0,
		}, {
			Resource: cloudapi.DATABASE,
			Quantity: 1,
		}, {
			Resource: cloudapi.APPLICATION,
			Quantity: 0,
		}, {
			Resource: cloudapi.KUBE_CLUSTER,
			Quantity: 0,
		}, {
			Resource: cloudapi.ROLE,
			Quantity: 0,
		}, {
			Resource: cloudapi.AUTH_CONNECTOR,
			Quantity: 0,
		}},
	}}

	require.Equal(t, needed, obtained)
}

func TestCheckClusterAlertTryCreateBuyTeleportAlert(t *testing.T) {
	ch := make(chan types.ClusterAlert, 1)
	trialBillingInfo := &cloudapi.GetBillingInformationResponse{
		Trial:        true,
		SelfEnrolled: true,
	}

	for _, test := range []struct {
		name              string
		shouldCreateAlert bool
		setupMocks        func(*reporterMocks)
	}{
		{
			name:              "creates alert if criteria is met",
			shouldCreateAlert: true,
			setupMocks: func(m *reporterMocks) {
				m.client.MockGetBillingInformation = func() (*cloudapi.GetBillingInformationResponse, error) {
					return trialBillingInfo, nil
				}

				m.apiGetters.MockedGetClusterAlerts = func() ([]types.ClusterAlert, error) {
					return nil, trace.NotFound("not-found")
				}

				m.apiGetters.MockedSearchEvents = func() ([]apievents.AuditEvent, string, error) {
					return []apievents.AuditEvent{
						&apievents.SessionStart{},
					}, "", nil
				}

				m.apiGetters.MockedUpsertClusterAlert = func(ctx context.Context, alert types.ClusterAlert) error {
					ch <- alert
					return nil
				}
			},
		},
		{
			name:              "does not create alert if not self enrolled",
			shouldCreateAlert: true,
			setupMocks: func(m *reporterMocks) {
				m.client.MockGetBillingInformation = func() (*cloudapi.GetBillingInformationResponse, error) {
					return &cloudapi.GetBillingInformationResponse{Trial: true, SelfEnrolled: false}, nil
				}

				m.apiGetters.MockedGetClusterAlerts = func() ([]types.ClusterAlert, error) {
					return nil, trace.NotFound("not-found")
				}

				m.apiGetters.MockedSearchEvents = func() ([]apievents.AuditEvent, string, error) {
					return []apievents.AuditEvent{
						&apievents.SessionStart{},
					}, "", nil
				}

				m.apiGetters.MockedUpsertClusterAlert = func(ctx context.Context, alert types.ClusterAlert) error {
					ch <- alert
					return nil
				}
			},
		},
		{
			name:              "does not create alert for non-trial",
			shouldCreateAlert: false,
			setupMocks: func(m *reporterMocks) {
				m.client.MockGetBillingInformation = func() (*cloudapi.GetBillingInformationResponse, error) {
					return &cloudapi.GetBillingInformationResponse{Trial: false, SelfEnrolled: true}, nil
				}
				m.apiGetters.MockedGetClusterAlerts = func() ([]types.ClusterAlert, error) {
					return nil, trace.NotFound("not-found")
				}
			},
		},
		{
			name:              "does not create alert if alert exists",
			shouldCreateAlert: false,
			setupMocks: func(m *reporterMocks) {
				m.client.MockGetBillingInformation = func() (*cloudapi.GetBillingInformationResponse, error) {
					return trialBillingInfo, nil
				}

				m.apiGetters.MockedGetClusterAlerts = func() ([]types.ClusterAlert, error) {
					return []types.ClusterAlert{
						{
							ResourceHeader: types.ResourceHeader{
								Metadata: types.Metadata{
									Name: buyTeleportAlertName,
								},
							},
						},
					}, nil
				}
			},
		},
		{
			name:              "does not create alert if no resources are accessed",
			shouldCreateAlert: false,
			setupMocks: func(m *reporterMocks) {
				m.client.MockGetBillingInformation = func() (*cloudapi.GetBillingInformationResponse, error) {
					return trialBillingInfo, nil
				}

				m.apiGetters.MockedGetClusterAlerts = func() ([]types.ClusterAlert, error) {
					return nil, trace.NotFound("not-found")
				}

				m.apiGetters.MockedSearchEvents = func() ([]apievents.AuditEvent, string, error) {
					return []apievents.AuditEvent{}, "", nil
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, err := createReporterMocks()
			require.NoError(t, err)

			test.setupMocks(&m)
			err = m.reporter.checkClusterAlert(context.Background())
			require.NoError(t, err)

			var insertedAlert types.ClusterAlert
			select {
			case a := <-ch:
				insertedAlert = a
			default:
			}
			if test.shouldCreateAlert {
				require.NotNil(t, insertedAlert)
				require.Equal(t, buyTeleportAlertName, insertedAlert.Metadata.Name)
			}
		})
	}
}

func TestCheckClusterAlertTryRemoveBuyTeleportAlert(t *testing.T) {
	ch := make(chan string, 1)
	trialBillingInfo := &cloudapi.GetBillingInformationResponse{
		Trial:        true,
		SelfEnrolled: true,
	}

	for _, test := range []struct {
		name              string
		shouldRemoveAlert bool
		setupMocks        func(*reporterMocks)
	}{
		{
			name:              "removes alert if criteria is met",
			shouldRemoveAlert: true,
			setupMocks: func(m *reporterMocks) {
				m.client.MockGetBillingInformation = func() (*cloudapi.GetBillingInformationResponse, error) {
					return &cloudapi.GetBillingInformationResponse{Trial: false, SelfEnrolled: true}, nil
				}

				m.apiGetters.MockedGetClusterAlerts = func() ([]types.ClusterAlert, error) {
					return []types.ClusterAlert{
						{
							ResourceHeader: types.ResourceHeader{
								Metadata: types.Metadata{
									Name: buyTeleportAlertName,
								},
							},
						},
					}, nil
				}

				m.apiGetters.MockedDeleteClusterAlert = func(ctx context.Context, alertID string) error {
					ch <- alertID
					return nil
				}
			},
		},
		{
			name:              "does not remove if not self-enrolled",
			shouldRemoveAlert: false,
			setupMocks: func(m *reporterMocks) {
				m.client.MockGetBillingInformation = func() (*cloudapi.GetBillingInformationResponse, error) {
					return &cloudapi.GetBillingInformationResponse{Trial: false, SelfEnrolled: false}, nil
				}

				m.apiGetters.MockedGetClusterAlerts = func() ([]types.ClusterAlert, error) {
					return []types.ClusterAlert{
						{
							ResourceHeader: types.ResourceHeader{
								Metadata: types.Metadata{
									Name: buyTeleportAlertName,
								},
							},
						},
					}, nil
				}

				m.apiGetters.MockedDeleteClusterAlert = func(ctx context.Context, alertID string) error {
					ch <- alertID
					return nil
				}
			},
		},
		{
			name:              "does not remove if trial account",
			shouldRemoveAlert: false,
			setupMocks: func(m *reporterMocks) {
				m.client.MockGetBillingInformation = func() (*cloudapi.GetBillingInformationResponse, error) {
					return trialBillingInfo, nil
				}

				m.apiGetters.MockedGetClusterAlerts = func() ([]types.ClusterAlert, error) {
					return []types.ClusterAlert{
						{
							ResourceHeader: types.ResourceHeader{
								Metadata: types.Metadata{
									Name: buyTeleportAlertName,
								},
							},
						},
					}, nil
				}
			},
		},
		{
			name:              "does not remove if does not exist",
			shouldRemoveAlert: false,
			setupMocks: func(m *reporterMocks) {
				m.client.MockGetBillingInformation = func() (*cloudapi.GetBillingInformationResponse, error) {
					return &cloudapi.GetBillingInformationResponse{Trial: false, SelfEnrolled: true}, nil
				}

				m.apiGetters.MockedGetClusterAlerts = func() ([]types.ClusterAlert, error) {
					return nil, nil
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, err := createReporterMocks()
			require.NoError(t, err)

			test.setupMocks(&m)
			err = m.reporter.checkClusterAlert(context.Background())
			require.NoError(t, err)

			var removedAlert string
			select {
			case a := <-ch:
				removedAlert = a
			default:
			}
			if test.shouldRemoveAlert {
				require.NotNil(t, removedAlert)
				require.Equal(t, buyTeleportAlertName, removedAlert)
			}
		})
	}
}

func createReporterMocks() (reporterMocks, error) {
	apiGetters := &MockedResourceGetter{}
	client := &cloud.MockedClient{}
	reporter, err := New(Config{
		Clock:          clockwork.NewFakeClock(),
		CloudClient:    client,
		ResourceGetter: apiGetters,
		BackendGetter:  NewMockedBackendGetter(),
		Interval:       1 * time.Microsecond,
	})

	return reporterMocks{
		apiGetters: apiGetters,
		client:     client,
		reporter:   reporter,
	}, err
}

type reporterMocks struct {
	apiGetters *MockedResourceGetter
	reporter   *UsageReporter
	client     *cloud.MockedClient
}
