package usagereporter

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/v7/types"
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

	m.apiGetters.MockedGetAppServers = func() ([]types.Server, error) {
		return []types.Server{&types.ServerV2{
			Spec: types.ServerSpecV2{
				Apps: []*types.App{{}},
			},
		}}, nil
	}

	m.apiGetters.MockedGetKubeServices = func() ([]types.Server, error) {
		return []types.Server{&types.ServerV2{}}, nil
	}

	m.apiGetters.MockedGetDatabaseServers = func() ([]types.DatabaseServer, error) {
		return []types.DatabaseServer{&types.DatabaseServerV3{}}, nil
	}

	var obtained []*cloudapi.UsageReport
	m.client.MockSubmitUsageReports = func(request *cloudapi.SubmitUsageReportsRequest) (*cloudapi.EmptyResponse, error) {
		obtained = request.Reports
		return nil, nil
	}

	m.reporter.reportUsage(context.TODO())

	needed := []*cloudapi.UsageReport{{PeriodStart: 449884799,
		PeriodEnd: 449884800,
		Items: []*cloudapi.UsageReportItem{{Resource: cloudapi.USER,
			Quantity: 1,
		}, {Resource: cloudapi.SERVER,
			Quantity: 1,
		}, {Resource: cloudapi.DATABASE,
			Quantity: 1,
		}, {Resource: cloudapi.APPLICATION,
			Quantity: 1,
		}, {Resource: cloudapi.KUBE_CLUSTER,
			Quantity: 1,
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

	m.apiGetters.MockedGetAppServers = func() ([]types.Server, error) {
		return nil, trace.BadParameter("unable to return apps")
	}

	m.apiGetters.MockedGetKubeServices = func() ([]types.Server, error) {
		return nil, trace.BadParameter("unable to return kube servers")
	}

	m.apiGetters.MockedGetDatabaseServers = func() ([]types.DatabaseServer, error) {
		return []types.DatabaseServer{&types.DatabaseServerV3{}}, nil
	}

	var obtained []*cloudapi.UsageReport
	m.client.MockSubmitUsageReports = func(request *cloudapi.SubmitUsageReportsRequest) (*cloudapi.EmptyResponse, error) {
		obtained = request.Reports
		return nil, nil
	}

	m.reporter.reportUsage(context.TODO())

	needed := []*cloudapi.UsageReport{{PeriodStart: 449884799,
		PeriodEnd: 449884800,
		Items: []*cloudapi.UsageReportItem{{Resource: cloudapi.USER,
			Quantity: 0,
		}, {Resource: cloudapi.SERVER,
			Quantity: 0,
		}, {Resource: cloudapi.DATABASE,
			Quantity: 1,
		}, {Resource: cloudapi.APPLICATION,
			Quantity: 0,
		}, {Resource: cloudapi.KUBE_CLUSTER,
			Quantity: 0,
		}},
	}}

	require.Equal(t, obtained, needed)
}

func createReporterMocks() (reporterMocks, error) {
	apiGetters := &MockedResourceGetter{}
	client := &cloud.MockedClient{}
	reporter, err := New(Config{
		Clock:          clockwork.NewFakeClock(),
		CloudClient:    client,
		ResourceGetter: apiGetters,
		BackendGetter:  &MockedBackendGetter{},
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
