package usagereporter

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/v7/defaults"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/backend"

	"github.com/gravitational/trace"
)

// UsageReporter reports usage information of Teleport resources
type UsageReporter struct {
	Config
}

// New instantiates a new pro/enterprise teleport process
func New(config Config) (*UsageReporter, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &UsageReporter{
		Config: config,
	}, nil
}

// Run starts the usage reporting loop
func (r *UsageReporter) Run(ctx context.Context) {
	r.Log.Infof("Usage Reporter has started and will report at %v interval.", r.Interval)
	ticker := r.Clock.NewTicker(r.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			r.reportUsage(ctx)
		case <-ctx.Done():
			r.Log.Info("Usage Reporter has stopped.")
			return
		}
	}
}

func (r *UsageReporter) reportUsage(ctx context.Context) {
	if err := r.acquireReportingLock(ctx, r.Interval-r.Interval/2); err != nil {
		if !trace.IsAlreadyExists(err) {
			r.Log.WithError(err).Error("Failed to set recording lock.")
		} else {
			r.Log.Info("Encountered active usage recording lock.")
		}
		return
	}

	nodes, err := r.ResourceGetter.GetNodes(ctx, defaults.Namespace)
	if err != nil {
		r.Log.WithError(err).Error("Failed to report number of nodes.")
	}

	databases, err := r.ResourceGetter.GetDatabaseServers(ctx, defaults.Namespace)
	if err != nil {
		r.Log.WithError(err).Error("Failed to report number of databases.")
	}

	users, err := r.ResourceGetter.GetUsers(false)
	if err != nil {
		r.Log.WithError(err).Error("Failed to report number of users.")
	}

	appCount, err := r.getAppsCount(ctx)
	if err != nil {
		r.Log.WithError(err).Error("Failed to report number of applications.")
	}

	kubeServers, err := r.ResourceGetter.GetKubeServices(ctx)
	if err != nil {
		r.Log.WithError(err).Error("Failed to report number of kube clusters.")
	}

	reportingTime := r.Clock.Now().UTC()
	req := &cloudapi.SubmitUsageReportsRequest{
		Reports: []*cloudapi.UsageReport{{
			PeriodStart: reportingTime.Add(-r.Interval).Unix(),
			PeriodEnd:   reportingTime.Unix(),
			Items: []*cloudapi.UsageReportItem{{
				Resource: cloudapi.USER,
				Quantity: int64(len(users)),
			}, {
				Resource: cloudapi.SERVER,
				Quantity: int64(len(nodes)),
			}, {
				Resource: cloudapi.DATABASE,
				Quantity: int64(len(databases)),
			}, {
				Resource: cloudapi.APPLICATION,
				Quantity: int64(appCount),
			}, {
				Resource: cloudapi.KUBE_CLUSTER,
				Quantity: int64(len(kubeServers)),
			}},
		}},
	}

	if _, err = r.CloudClient.SubmitUsageReports(ctx, req); err != nil {
		r.Log.WithError(err).Error("Unable submit usage report.")
	} else {
		r.Log.Infof("Reported: nodes=%v, users=%v, databases=%v, k8s=%v, apps=%v",
			len(nodes),
			len(users),
			len(databases),
			len(kubeServers),
			appCount,
		)
	}
}

func (r *UsageReporter) getAppsCount(ctx context.Context) (int, error) {
	servers, err := r.ResourceGetter.GetAppServers(ctx, defaults.Namespace)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	count := 0
	for _, server := range servers {
		count = count + len(server.GetApps())
	}

	return count, nil
}

// acquireRecordingLock attempts to set a lock for recording new usage. Returns an isAlreadyExists error in case the
// lock already exists. The lock will expire over time.
func (r *UsageReporter) acquireReportingLock(ctx context.Context, ttl time.Duration) error {
	item := backend.Item{
		Key:     backend.Key(cloudPrefix, lockPrefix),
		Value:   []byte{1},
		Expires: r.BackendGetter.Clock().Now().UTC().Add(ttl),
	}

	_, err := r.BackendGetter.Create(ctx, item)
	return trace.Wrap(err)
}

const (
	cloudPrefix = "cloud"
	lockPrefix  = "lock"
)
