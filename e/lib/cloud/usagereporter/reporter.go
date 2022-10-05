package usagereporter

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

// UsageReporter reports usage information of Teleport resources
type UsageReporter struct {
	Config
}

const buyTeleportAlertName = "upgrade-to-paid-plan"
const trialProductName = "Teleport 14 Day Trial"

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

	apps, err := r.ResourceGetter.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		r.Log.WithError(err).Error("Failed to report number of applications.")
	}

	kubeServers, err := r.ResourceGetter.GetKubeServices(ctx)
	if err != nil {
		r.Log.WithError(err).Error("Failed to report number of kube clusters.")
	}

	roles, err := r.ResourceGetter.GetRoles(ctx)
	if err != nil {
		r.Log.WithError(err).Error("Failed to report number of roles.")
	}

	authConnectorCount, err := r.getAuthConnectorCount(ctx)
	if err != nil {
		r.Log.WithError(err).Error("Failed to report number of auth connectors.")
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
				Quantity: int64(len(apps)),
			}, {
				Resource: cloudapi.KUBE_CLUSTER,
				Quantity: int64(len(kubeServers)),
			}, {
				Resource: cloudapi.ROLE,
				Quantity: int64(len(roles)),
			}, {
				Resource: cloudapi.AUTH_CONNECTOR,
				Quantity: int64(authConnectorCount),
			}},
		}},
	}

	if _, err = r.CloudClient.SubmitUsageReports(ctx, req); err != nil {
		r.Log.WithError(err).Error("Unable submit usage report.")
	} else {
		r.Log.Infof("Reported: nodes=%v, users=%v, databases=%v, k8s=%v, apps=%v, roles=%v, auth_connectors=%v",
			len(nodes),
			len(users),
			len(databases),
			len(kubeServers),
			len(apps),
			len(roles),
			authConnectorCount,
		)
	}

	userCreatedResource := len(apps) > 0 || len(nodes) > 0 || len(databases) > 0 || len(kubeServers) > 0
	if userCreatedResource {
		err := r.checkClusterAlert(ctx)
		if err != nil {
			r.Log.WithError(err).Error("Failed to check cluster alert.")
		}
	}
}

func (r *UsageReporter) checkClusterAlert(ctx context.Context) error {
	b, err := r.CloudClient.GetBillingInformation(ctx, &cloudapi.EmptyRequest{})
	if err != nil {
		return trace.Wrap(err)
	}

	alerts, err := r.ResourceGetter.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: buyTeleportAlertName,
	})
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if b.ProductName == trialProductName && len(alerts) == 0 {
		if err := r.tryCreateBuyTeleportAlert(ctx); err != nil {
			r.Log.WithError(err).Error("Failed to try/create cluster alert for trial.")
		}
	}

	if b.ProductName != trialProductName && len(alerts) != 0 {
		if err := r.tryRemoveBuyTeleportAlert(ctx); err != nil {
			r.Log.WithError(err).Error("Failed to try/remove cluster alert for trial.")
		}
	}

	return nil
}

func (r *UsageReporter) tryCreateBuyTeleportAlert(ctx context.Context) error {
	accessEventTypes := []string{
		events.SessionStartEvent,
		events.AppSessionStartEvent,
		events.DatabaseSessionStartEvent,
		events.KubeRequestEvent,
		events.WindowsDesktopSessionStartEvent,
	}
	accessEvents, _, err := r.ResourceGetter.SearchEvents(
		time.Now().Add(-2*r.Interval),
		time.Now(),
		defaults.Namespace,
		accessEventTypes,
		1,
		types.EventOrderAscending,
		"",
	)
	if err != nil {
		return trace.Wrap(err)
	}
	// if the user has not accessed a resource, do not create an alert
	if len(accessEvents) == 0 {
		return nil
	}

	alert, err := types.NewClusterAlert(
		buyTeleportAlertName,
		"Upgrade to a paid plan",
		types.WithAlertSeverity(types.AlertSeverity_LOW),
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
		types.WithAlertLabel(types.AlertPermitAll, "yes"),
		types.WithAlertLabel(types.AlertLink, "https://goteleport.com/signup/cloud?utm_campaign=cloud&utm_medium=product&utm_source=upgrade"),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(r.ResourceGetter.UpsertClusterAlert(ctx, alert))
}

func (r *UsageReporter) tryRemoveBuyTeleportAlert(ctx context.Context) error {
	err := r.ResourceGetter.DeleteClusterAlert(ctx, buyTeleportAlertName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return nil
}

func (r *UsageReporter) getAuthConnectorCount(ctx context.Context) (int, error) {
	ghConnectors, err := r.ResourceGetter.GetGithubConnectors(ctx, false)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	oidcConnectors, err := r.ResourceGetter.GetOIDCConnectors(ctx, false)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	samlConnectors, err := r.ResourceGetter.GetSAMLConnectors(ctx, false)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return len(ghConnectors) + len(oidcConnectors) + len(samlConnectors), nil
}

// acquireRecordingLock attempts to set a lock for recording new usage. Returns an isAlreadyExists error in case the
// lock already exists. The lock will expire over time.
func (r *UsageReporter) acquireReportingLock(ctx context.Context, ttl time.Duration) error {
	item := backend.Item{
		Key:     backend.Key(cloudPrefix, lockPrefix),
		Value:   []byte{1},
		Expires: r.BackendGetter.Clock().Now().UTC().Add(ttl),
	}

	// Check if the item exists before attempting to create it to reduce the amount of reported backend write failures. Errors in the backend will be caught by the following Create.
	if resp, err := r.BackendGetter.Get(ctx, item.Key); err == nil {
		if r.BackendGetter.Clock().Now().UTC().Before(resp.Expires) {
			return trace.AlreadyExists("%v already exists", item.Key)
		}
	}

	_, err := r.BackendGetter.Create(ctx, item)
	return trace.Wrap(err)
}

const (
	cloudPrefix = "cloud"
	lockPrefix  = "lock"
)
