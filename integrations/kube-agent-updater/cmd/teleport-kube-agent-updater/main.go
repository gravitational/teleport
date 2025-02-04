/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/go-logr/logr"
	"github.com/gravitational/trace"
	"golang.org/x/mod/semver"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gravitational/teleport/api/client/webclient"
	kubeversionupdater "github.com/gravitational/teleport/integrations/kube-agent-updater"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/controller"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/img"
	podmaintenance "github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/maintenance"
	"github.com/gravitational/teleport/lib/automaticupgrades/maintenance"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
}

var extraFields = []string{logutils.LevelField, logutils.ComponentField, logutils.TimestampField}

func main() {
	ctx := ctrl.SetupSignalHandler()

	// Setup early logger, using INFO level by default.
	slogLogger, slogLeveler, err := logutils.Initialize(logutils.Config{
		Severity:    slog.LevelInfo.String(),
		Format:      "json",
		ExtraFields: extraFields,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logs: %v\n", err)
		os.Exit(1)
	}

	logger := logr.FromSlogHandler(slogLogger.Handler())
	ctrl.SetLogger(logger)

	var agentName string
	var agentNamespace string
	var metricsAddr string
	var probeAddr string
	var syncPeriod time.Duration
	var baseImageName string
	var versionServer string
	var versionChannel string
	var insecureNoVerify bool
	var insecureNoResolve bool
	var disableLeaderElection bool
	var credSource string
	var logLevel string
	var proxyAddress string
	var updateGroup string

	flag.StringVar(&agentName, "agent-name", "", "The name of the agent that should be updated. This is mandatory.")
	flag.StringVar(&agentNamespace, "agent-namespace", "", "The namespace of the agent that should be updated. This is mandatory.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "healthz-addr", ":8081", "The address the probe endpoint binds to.")
	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Hour, "Operator sync period (format: https://pkg.go.dev/time#ParseDuration)")
	flag.BoolVar(&insecureNoVerify, "insecure-no-verify-image", false, "Disable image signature verification. The image tag is still resolved and image must exist.")
	flag.BoolVar(&insecureNoResolve, "insecure-no-resolve-image", false, "Disable image signature verification AND resolution. The updater can update to non-existing images.")
	flag.BoolVar(&disableLeaderElection, "disable-leader-election", false, "Disable leader election, used when running the kube-agent-updater outside of Kubernetes.")
	flag.StringVar(&proxyAddress, "proxy-address", "", "The proxy address of the teleport cluster. When set, the updater will try to get update via the /find proxy endpoint.")
	flag.StringVar(&updateGroup, "update-group", "", "The agent update group, as defined in the `autoupdate_config` resource. When unset or set to an unknown value, agent will update with the default group.")
	flag.StringVar(&versionServer, "version-server", "https://updates.releases.teleport.dev/v1/", "URL of the HTTP server advertising target version and critical maintenances. Trailing slash is optional.")
	flag.StringVar(&versionChannel, "version-channel", "stable/cloud", "Version channel to get updates from.")
	flag.StringVar(&baseImageName, "base-image", "public.ecr.aws/gravitational/teleport", "Image reference containing registry and repository.")
	flag.StringVar(&credSource, "pull-credentials", img.NoCredentialSource,
		fmt.Sprintf("Where to get registry pull credentials, values are '%s', '%s', '%s', '%s'.",
			img.DockerCredentialSource, img.GoogleCredentialSource, img.AmazonCredentialSource, img.NoCredentialSource,
		),
	)
	flag.StringVar(&logLevel, "log-level", "INFO", "Log level (DEBUG, INFO, WARN, ERROR).")
	flag.Parse()

	// Now that we parsed the flags, we can tune the log level.
	var lvl slog.Level
	if err := (&lvl).UnmarshalText([]byte(logLevel)); err != nil {
		ctrl.Log.Error(err, "Failed to parse log level", "level", logLevel)
		os.Exit(1)
	}
	slogLeveler.Set(lvl)

	// Validate configuration.
	if agentName == "" {
		ctrl.Log.Error(trace.BadParameter("--agent-name empty"), "agent-name must be provided")
		os.Exit(1)
	}
	if agentNamespace == "" {
		ctrl.Log.Error(trace.BadParameter("--agent-namespace empty"), "agent-namespace must be provided")
		os.Exit(1)
	}
	if versionServer == "" && proxyAddress == "" {
		ctrl.Log.Error(
			trace.BadParameter("at least one of --proxy-address or --version-server must be provided"),
			"the updater has no upstream configured, it cannot retrieve the version and check when to update",
		)
		os.Exit(1)
	}

	// Build a new controller manager. We need to do this early as some trigger
	// need a Kubernetes client and the manager is the one providing it.
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         !disableLeaderElection,
		LeaderElectionID:       agentName,
		Cache: cache.Options{
			// Create a cache scoped to the agentNamespace
			DefaultNamespaces: map[string]cache.Config{
				agentNamespace: {},
			},
			SyncPeriod: &syncPeriod,
			ByObject: map[kclient.Object]cache.ByObject{
				&appsv1.Deployment{}:  {Field: fields.SelectorFromSet(fields.Set{"metadata.name": agentName})},
				&appsv1.StatefulSet{}: {Field: fields.SelectorFromSet(fields.Set{"metadata.name": agentName})},
			},
		},
	})
	if err != nil {
		ctrl.Log.Error(err, "failed to create new manager, exiting")
		os.Exit(1)
	}

	// Craft the version getter and update triggers based on the configuration (use RFD-109 APIs, RFD-184, or both).
	var criticalUpdateTriggers []maintenance.Trigger
	var plannedMaintenanceTriggers []maintenance.Trigger
	var versionGetters []version.Getter

	// If the proxy server is specified, we enabled RFD-184 updates
	// See https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md#updater-apis
	if proxyAddress != "" {
		ctrl.Log.Info("fetching versions from the proxy /find endpoint", "proxy_server_url", proxyAddress, "update_group", updateGroup)

		proxyClt, err := webclient.NewReusableClient(&webclient.Config{
			Context:     ctx,
			ProxyAddr:   proxyAddress,
			UpdateGroup: updateGroup,
		})
		if err != nil {
			ctrl.Log.Error(err, "failed to create proxy client, exiting")
			os.Exit(1)
		}

		// We do a preflight check before starting to know if the proxy is correctly configured and reachable.
		ctrl.Log.Info("preflight check: ping the proxy server", "proxy_server_url", proxyAddress)
		pong, err := proxyClt.Ping()
		if err != nil {
			ctrl.Log.Error(err, "failed to ping proxy, either the proxy address is wrong, or the network blocks connections to the proxy",
				"proxy_address", proxyAddress,
			)
			os.Exit(1)
		}
		ctrl.Log.Info("proxy server successfully pinged",
			"proxy_server_url", proxyAddress,
			"proxy_cluster_name", pong.ClusterName,
			"proxy_version", pong.ServerVersion,
		)

		versionGetters = append(versionGetters, version.NewProxyVersionGetter("proxy update protocol", proxyClt))

		// In RFD 184, the server is driving the update, so both regular maintenances and
		// critical ones are fetched from the proxy. Using the same trigger ensures we hit the cache if both triggers
		// are evaluated and don't actually make 2 calls.
		proxyTrigger := maintenance.NewProxyMaintenanceTrigger("proxy update protocol", proxyClt)
		criticalUpdateTriggers = append(criticalUpdateTriggers, proxyTrigger)
		plannedMaintenanceTriggers = append(plannedMaintenanceTriggers, proxyTrigger)
	}

	// If the version server is specified, we enable RFD-109 updates
	// See https://github.com/gravitational/teleport/blob/master/rfd/0109-cloud-agent-upgrades.md#kubernetes-model
	if versionServer != "" {
		rawUrl := strings.TrimRight(versionServer, "/") + "/" + versionChannel
		versionServerURL, err := url.Parse(rawUrl)
		if err != nil {
			ctrl.Log.Error(err, "failed to parse version server URL, exiting", "url", rawUrl)
			os.Exit(1)
		}
		ctrl.Log.Info("fetching versions from the version server", "version_server_url", versionServerURL.String())

		versionGetters = append(versionGetters, version.NewBasicHTTPVersionGetter(versionServerURL))
		// critical updates are advertised by the version channel
		criticalUpdateTriggers = append(criticalUpdateTriggers, maintenance.NewBasicHTTPMaintenanceTrigger("critical update", versionServerURL))
		// planned maintenance windows are exported by the pods
		plannedMaintenanceTriggers = append(plannedMaintenanceTriggers, podmaintenance.NewWindowTrigger("maintenance window", mgr.GetClient()))
	}

	maintenanceTriggers := maintenance.Triggers{
		// We check if the update is critical.
		maintenance.FailoverTrigger(criticalUpdateTriggers),
		// We check if the agent in unhealthy.
		podmaintenance.NewUnhealthyWorkloadTrigger("unhealthy pods", mgr.GetClient()),
		// We check if we're in a maintenance window.
		maintenance.FailoverTrigger(plannedMaintenanceTriggers),
	}

	kc, err := img.GetKeychain(credSource)
	if err != nil {
		ctrl.Log.Error(err, "failed to get keychain for registry auth")
	}

	var imageValidators img.Validators
	switch {
	case insecureNoResolve:
		ctrl.Log.Info("INSECURE: Image validation and resolution disabled")
		imageValidators = append(imageValidators, img.NewNopValidator("insecure no resolution"))
	case insecureNoVerify:
		ctrl.Log.Info("INSECURE: Image validation disabled")
		imageValidators = append(imageValidators, img.NewInsecureValidator("insecure always verified", kc))
	case semver.Prerelease("v"+kubeversionupdater.Version) != "":
		ctrl.Log.Info("This is a pre-release updater version, the key usied to sign dev and pre-release builds of Teleport will be trusted.")
		validator, err := img.NewCosignSingleKeyValidator(teleportStageOCIPubKey, "staging cosign signature validator", kc)
		if err != nil {
			ctrl.Log.Error(err, "failed to build pre-release image validator, exiting")
			os.Exit(1)
		}
		imageValidators = append(imageValidators, validator)
		fallthrough
	default:
		validator, err := img.NewCosignSingleKeyValidator(teleportProdOCIPubKey, "cosign signature validator", kc)
		if err != nil {
			ctrl.Log.Error(err, "failed to build image validator, exiting")
			os.Exit(1)
		}
		imageValidators = append(imageValidators, validator)
	}

	baseImage, err := reference.ParseNamed(baseImageName)
	if err != nil {
		ctrl.Log.Error(err, "failed to parse base image reference, exiting")
		os.Exit(1)
	}

	versionUpdater := controller.NewVersionUpdater(
		version.FailoverGetter(versionGetters),
		imageValidators,
		maintenanceTriggers,
		baseImage,
	)

	// Controller registration
	deploymentController := controller.DeploymentVersionUpdater{
		VersionUpdater: versionUpdater,
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
	}

	if err := deploymentController.SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "failed to setup deployment controller, exiting")
		os.Exit(1)
	}

	statefulsetController := controller.StatefulSetVersionUpdater{
		VersionUpdater: versionUpdater,
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
	}

	if err := statefulsetController.SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "failed to setup statefulset controller, exiting")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	ctrl.Log.Info("starting the updater", "version", kubeversionupdater.Version)

	if err := mgr.Start(ctx); err != nil {
		ctrl.Log.Error(err, "failed to start manager, exiting")
		os.Exit(1)
	}
}
