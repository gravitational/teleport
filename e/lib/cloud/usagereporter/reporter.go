package usagereporter

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
)

// UsageReporter reports usage information of Teleport resources
type UsageReporter struct {
	Config
}

const buyTeleportAlertName = "upgrade-to-paid-plan"

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
	r.Logger.InfoContext(ctx, "Usage Reporter has started", "reporting_interval", r.Interval)
	ticker := r.Clock.NewTicker(r.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			r.reportUsage(ctx)
		case <-ctx.Done():
			r.Logger.InfoContext(ctx, "Usage Reporter has stopped")
			return
		}
	}
}

func (r *UsageReporter) reportUsage(ctx context.Context) {
	if err := r.acquireReportingLock(ctx, r.Interval-r.Interval/2); err != nil {
		if !trace.IsAlreadyExists(err) {
			r.Logger.ErrorContext(ctx, "Failed to set recording lock", "error", err)
		} else {
			r.Logger.InfoContext(ctx, "Encountered active usage recording lock")
		}
		return
	}

	nodes, err := r.ResourceGetter.GetNodes(ctx, defaults.Namespace)
	if err != nil {
		r.Logger.ErrorContext(ctx, "Failed to report number of nodes", "error", err)
	}

	databases, err := r.ResourceGetter.GetDatabaseServers(ctx, defaults.Namespace)
	if err != nil {
		r.Logger.ErrorContext(ctx, "Failed to report number of databases", "error", err)
	}

	users, err := r.ResourceGetter.GetUsers(ctx, false)
	if err != nil {
		r.Logger.ErrorContext(ctx, "Failed to report number of users", "error", err)
	}

	apps, err := r.ResourceGetter.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		r.Logger.ErrorContext(ctx, "Failed to report number of applications", "error", err)
	}

	kubeServers, err := r.ResourceGetter.GetKubernetesServers(ctx)
	if err != nil {
		r.Logger.ErrorContext(ctx, "Failed to report number of kube clusters", "error", err)
	}

	roles, err := r.ResourceGetter.GetRoles(ctx)
	if err != nil {
		r.Logger.ErrorContext(ctx, "Failed to report number of roles", "error", err)
	}

	authConnectorCount, err := r.getAuthConnectorCount(ctx)
	if err != nil {
		r.Logger.ErrorContext(ctx, "Failed to report number of auth connectors", "error", err)
	}

	reportingTime := r.Clock.Now().UTC()
	req := &cloudapi.SubmitUsageReportsRequest{
		Reports: []*cloudapi.UsageReport{{
			PeriodStart: reportingTime.Add(-r.Interval).Unix(),
			PeriodEnd:   reportingTime.Unix(),
			Items: []*cloudapi.UsageReportItem{{
				Resource: cloudapi.UsageResourceType_USER,
				Quantity: int64(len(users)),
			}, {
				Resource: cloudapi.UsageResourceType_SERVER,
				Quantity: int64(len(nodes)),
			}, {
				Resource: cloudapi.UsageResourceType_DATABASE,
				Quantity: int64(len(databases)),
			}, {
				Resource: cloudapi.UsageResourceType_APPLICATION,
				Quantity: int64(len(apps)),
			}, {
				Resource: cloudapi.UsageResourceType_KUBE_CLUSTER,
				Quantity: int64(len(kubeServers)),
			}, {
				Resource: cloudapi.UsageResourceType_ROLE,
				Quantity: int64(len(roles)),
			}, {
				Resource: cloudapi.UsageResourceType_AUTH_CONNECTOR,
				Quantity: int64(authConnectorCount),
			}},
		}},
	}

	if _, err = r.CloudClient.SubmitUsageReports(ctx, req); err != nil {
		r.Logger.ErrorContext(ctx, "Unable submit usage report", "error", err)
	} else {
		r.Logger.InfoContext(ctx, "Successfully submitted usage report",
			"nodes", len(nodes),
			"users", len(users),
			"databases", len(databases),
			"k8s", len(kubeServers),
			"apps", len(apps),
			"roles", len(roles),
			"auth_connectors", authConnectorCount,
		)
	}

	userCreatedResource := len(apps) > 0 || len(nodes) > 0 || len(databases) > 0 || len(kubeServers) > 0
	if userCreatedResource {
		err := r.checkClusterAlert(ctx)
		if err != nil {
			r.Logger.ErrorContext(ctx, "Failed to check cluster alert", "error", err)
		}
	}
}

func (r *UsageReporter) checkClusterAlert(ctx context.Context) error {
	b, err := r.CloudClient.GetBillingInformation(ctx, &cloudapi.EmptyRequest{})
	if err != nil {
		return trace.Wrap(err)
	}

	if !b.SelfEnrolled {
		// if user did not sign themselves up for their trial account, do not create/clear alerts
		return nil
	}

	alerts, err := r.ResourceGetter.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: buyTeleportAlertName,
	})
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if b.Trial && b.UpsellAlert && len(alerts) == 0 {
		if err := r.tryCreateBuyTeleportAlert(ctx); err != nil {
			r.Logger.ErrorContext(ctx, "Failed to try/create cluster alert for trial", "error", err)
		}
	}

	if (!b.Trial || !b.UpsellAlert) && len(alerts) != 0 {
		if err := r.tryRemoveBuyTeleportAlert(ctx); err != nil {
			r.Logger.ErrorContext(ctx, "Failed to try/remove cluster alert for trial", "error", err)
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
	accessEvents, _, err := r.ResourceGetter.SearchEvents(ctx, events.SearchEventsRequest{
		From:       time.Now().Add(-2 * r.Interval),
		To:         time.Now(),
		EventTypes: accessEventTypes,
		Limit:      1,
		Order:      types.EventOrderAscending,
	})
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
		types.WithAlertLabel(types.AlertLinkText, "Upgrade Plan"),
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
		Key:     backend.NewKey(cloudPrefix, lockPrefix),
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
