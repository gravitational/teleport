/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package adaptor

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	"github.com/gravitational/teleport/lib/backend/kubernetes"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/versioncontrol/endpoint"
	uw "github.com/gravitational/teleport/lib/versioncontrol/upgradewindow"
)

// Updater is the adaptor for the local updater.
// Once created, it allows the Teleport process to get information
// about its updater status, and communicate with the updater.
type Updater struct {
	kind                   string
	external               string
	version                *semver.Version
	helloUpdaterInfoGetter func(ctx context.Context) (*types.UpdaterV2Info, error)

	config *Config
	// kubeBackend is only set when the updater kind is "kube"
	kubeBackend *kubernetes.Backend

	// lock protects client
	lock sync.Mutex
	// client is populated only once we get the instance connector. This can only happen after
	// RegisterRoutines is called. If you need to wait for the client,
	// use config.ClientGetter instead.
	client UpgradeWindowsClient
}

// Kind returns the detected updater kind.
// Empty if not updater.
func (u *Updater) Kind() string {
	return u.kind
}

// External returns the external updater kind.
// Empty if no updater.
// Note(hugoShaka): I'm not sure what the difference is between updater and external updater.
// It looks like it has something to do with integrations and agents updated by non-local updaters.
// However, this should just be another value in Kind.
// This is kept here for backward compatibility, I don't want to break anything.
func (u *Updater) External() string {
	return u.external
}

// Version returns the detected updater version.
// Can return nil if no updater is detected or if we cannot find the updater version.
func (u *Updater) Version() *semver.Version {
	return u.version
}

// Info returns the updater info that should be sent upstream as part of the Hello.
// This should be evaluated just before sending the hello as the information might change
// and Hellos are sent again when the instance is reconnecting.
// This returns trace.ErrNotImplemented if the updater does not support reporting information (Managed Updates v1).
func (u *Updater) Info(ctx context.Context) (*types.UpdaterV2Info, error) {
	if u.helloUpdaterInfoGetter == nil {
		return nil, trace.NotImplemented("updater kind %q does not support reporting updater info", u.kind)
	}
	return u.helloUpdaterInfoGetter(ctx)
}

// RegisterRoutines registers the updater-related routines against the supervisor.
// This function is separate from DetectAndConfigureUpdater to avoid a circular dependency.
// The inventory needs Updater.Info() to be created, and the updater routines need the
// inventory to detect if the auth connection is healthy.
func (u *Updater) RegisterRoutines(ctx context.Context, supervisor ProcessSupervisor, clientGetter func() (UpgradeWindowsClient, error), sentinel <-chan inventory.DownstreamSender) error {
	go u.waitForClient(ctx, clientGetter)

	switch u.kind {
	case types.UpgraderKindSystemdUnit:
		// For managed updates v1 unit updaters, we don't need to collect info
		// But we must export the endpoint and maintenance schedule.
		supervisor.RegisterFunc("autoupdates.endpoint.export", func() error {
			// This call will block until we have a real client or the process exit context is cancelled.
			clt, err := clientGetter()
			if err != nil {
				return trace.Wrap(err)
			}
			if clt == nil {
				return trace.BadParameter("process exiting and Instance connector never became available")
			}

			resp, err := clt.Ping(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			if !resp.GetServerFeatures().GetCloud() {
				return nil
			}

			if err := endpoint.Export(ctx, u.config.ResolverAddr.String()); err != nil {
				u.config.Log.WarnContext(ctx,
					"Failed to export and validate autoupdates endpoint.",
					"addr", u.config.ResolverAddr.String(),
					"error", err)
				return trace.Wrap(err)
			}
			u.config.Log.InfoContext(ctx, "Exported autoupdates endpoint.", "addr", u.config.ResolverAddr.String())
			return nil
		})

		// Export updater schedule
		if err := u.runUpdaterExporter(ctx, supervisor, sentinel); err != nil {
			return trace.Wrap(err)
		}
	case types.UpgraderKindKubeController:
		// Export updater schedule
		if err := u.runUpdaterExporter(ctx, supervisor, sentinel); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// DetectAndConfigureUpdater detects if the current Teleport instance is managed by an updater,
// configures the updater if needed (managed updates v1 maintenance window export, or managed updates v2 updater uuid)
// and returns an updater adaptor describing the updater and allowing to interact with it.
func DetectAndConfigureUpdater(ctx context.Context, cfg *Config) (*Updater, error) {

	if err := cfg.Check(ctx); err != nil {
		return nil, trace.Wrap(err, "checking updater detector configuration")
	}
	// Detect the kind of updater we use and its version
	upgraderKind, externalUpgrader, upgraderVersion := detectUpdater(ctx, cfg.Log)

	updater := &Updater{
		kind:     upgraderKind,
		external: externalUpgrader,
		version:  upgraderVersion,
		config:   cfg,
	}

	switch updater.kind {
	case types.UpgraderKindKubeController:
		kubeSharedBackend, err := kubernetes.NewShared()
		if err != nil {
			return updater, trace.Wrap(err, "building the shared kubernetes backend used to communicate with the updater")
		}
		updater.kubeBackend = kubeSharedBackend
	case types.UpgraderKindTeleportUpdate:
		// Exports are not required for teleport-update, we only need to collect infos
		updater.helloUpdaterInfoGetter = func(ctx context.Context) (*types.UpdaterV2Info, error) {
			return autoupdate.ReadHelloUpdaterInfo(ctx, updater.config.Log, updater.config.HostUUID)
		}
	}
	return updater, nil
}

// detectUpdater returns metadata about auto-upgraders that may be active.
// Note that kind and externalName are usually the same.
// However, some unregistered upgraders like the AWS ODIC upgrader are not valid kinds.
// For these upgraders, kind is empty and externalName is set to a non-kind value.
func detectUpdater(ctx context.Context, log *slog.Logger) (kind, externalName string, version *semver.Version) {
	// Check if the deprecated teleport-upgrader script is being used.
	kind = os.Getenv(automaticupgrades.EnvUpgrader)
	version = automaticupgrades.GetUpgraderVersion(ctx)
	if version == nil {
		kind = ""
	}

	// If the installation is managed by teleport-update, it supersedes the teleport-upgrader script.
	ok, err := autoupdate.IsManagedByUpdater()
	if err != nil {
		log.WarnContext(ctx, "Failed to determine if auto-updates are enabled.", "error", err)
	} else if ok {
		// If this is a teleport-update managed installation, the version
		// managed by the timer will always match the installed version of teleport.
		kind = types.UpgraderKindTeleportUpdate
		version = teleport.SemVer()
	}

	// Instances deployed using the AWS OIDC integration are automatically updated
	// by the proxy. The instance heartbeat should properly reflect that.
	externalName = kind
	if externalName == "" && os.Getenv(types.InstallMethodAWSOIDCDeployServiceEnvVar) == "true" {
		externalName = types.OriginIntegrationAWSOIDC
	}
	return kind, externalName, version
}

// runUpdaterExporter configures the window exporter for upgraders that export windows.
// This is only used for Managed Updates v1 (unit and old kube updaters).
func (u *Updater) runUpdaterExporter(ctx context.Context, supervisor ProcessSupervisor, sentinel <-chan inventory.DownstreamSender) error {
	if u.kubeBackend == nil {
		return trace.BadParameter("kubeBackend is not set, this is a bug")
	}
	driver, err := uw.NewDriver(u.kind, u.kubeBackend)
	if err != nil {
		return trace.Wrap(err)
	}

	exporter, err := uw.NewExporter(uw.ExporterConfig[inventory.DownstreamSender]{
		Driver:                   driver,
		ExportFunc:               u.getWindow,
		AuthConnectivitySentinel: sentinel,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	supervisor.RegisterCriticalFunc("upgradeewindow.export", exporter.Run)
	supervisor.OnExit("upgradewindow.export.stop", func(_ any) {
		exporter.Close()
	})

	u.config.Log.InfoContext(ctx, "Configured upgrade window exporter for external upgrader.", "kind", u.kind)
	return nil
}

// waitForClient calls a clientGetter and waits to obtain a client. The client is then stored
// in Updater.client.
func (u *Updater) waitForClient(ctx context.Context, clientGetter func() (UpgradeWindowsClient, error)) {
	clt, err := clientGetter()
	if err != nil {
		u.config.Log.WarnContext(ctx, "Unable to get client for updater.", "err", err)
		return
	}
	u.lock.Lock()
	defer u.lock.Unlock()
	u.client = clt
}

func (u *Updater) getWindow(ctx context.Context, req proto.ExportUpgradeWindowsRequest) (proto.ExportUpgradeWindowsResponse, error) {
	u.lock.Lock()
	defer u.lock.Unlock()
	// The client might not be ready yet (e.g. the instance is still connecting)
	// We try getting it, and if it works, we use it.
	if u.client == nil {
		return proto.ExportUpgradeWindowsResponse{}, trace.Errorf("client not yet initialized")
	}
	return u.client.ExportUpgradeWindows(ctx, req)
}
