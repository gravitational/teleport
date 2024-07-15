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
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/gravitational/trace"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kubeversionupdater "github.com/gravitational/teleport/integrations/kube-agent-updater"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/controller"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/img"
	podmaintenance "github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/maintenance"
	"github.com/gravitational/teleport/lib/automaticupgrades/maintenance"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
}

func main() {
	ctx := ctrl.SetupSignalHandler()

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

	flag.StringVar(&agentName, "agent-name", "", "The name of the agent that should be updated. This is mandatory.")
	flag.StringVar(&agentNamespace, "agent-namespace", "", "The namespace of the agent that should be updated. This is mandatory.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "healthz-addr", ":8081", "The address the probe endpoint binds to.")
	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Hour, "Operator sync period (format: https://pkg.go.dev/time#ParseDuration)")
	flag.BoolVar(&insecureNoVerify, "insecure-no-verify-image", false, "Disable image signature verification. The image tag is still resolved and image must exist.")
	flag.BoolVar(&insecureNoResolve, "insecure-no-resolve-image", false, "Disable image signature verification AND resolution. The updater can update to non-existing images.")
	flag.BoolVar(&disableLeaderElection, "disable-leader-election", false, "Disable leader election, used when running the kube-agent-updater outside of Kubernetes.")
	flag.StringVar(&versionServer, "version-server", "https://updates.releases.teleport.dev/v1/", "URL of the HTTP server advertising target version and critical maintenances. Trailing slash is optional.")
	flag.StringVar(&versionChannel, "version-channel", "stable/cloud", "Version channel to get updates from.")
	flag.StringVar(&baseImageName, "base-image", "public.ecr.aws/gravitational/teleport", "Image reference containing registry and repository.")
	flag.StringVar(&credSource, "pull-credentials", img.NoCredentialSource,
		fmt.Sprintf("Where to get registry pull credentials, values are '%s', '%s', '%s', '%s'.",
			img.DockerCredentialSource, img.GoogleCredentialSource, img.AmazonCredentialSource, img.NoCredentialSource,
		),
	)

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if agentName == "" {
		ctrl.Log.Error(trace.BadParameter("--agent-name empty"), "agent-name must be provided")
		os.Exit(1)
	}
	if agentNamespace == "" {
		ctrl.Log.Error(trace.BadParameter("--agent-namespace empty"), "agent-namespace must be provided")
		os.Exit(1)
	}

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

	versionServerURL, err := url.Parse(strings.TrimRight(versionServer, "/") + "/" + versionChannel)
	if err != nil {
		ctrl.Log.Error(err, "failed to parse version server URL, exiting")
		os.Exit(1)
	}
	versionGetter := version.NewBasicHTTPVersionGetter(versionServerURL)
	maintenanceTriggers := maintenance.Triggers{
		maintenance.NewBasicHTTPMaintenanceTrigger("critical update", versionServerURL),
		podmaintenance.NewUnhealthyWorkloadTrigger("unhealthy pods", mgr.GetClient()),
		podmaintenance.NewWindowTrigger("maintenance window", mgr.GetClient()),
	}

	var imageValidators img.Validators
	switch {
	case insecureNoResolve:
		ctrl.Log.Info("INSECURE: Image validation and resolution disabled")
		imageValidators = append(imageValidators, img.NewNopValidator("insecure no resolution"))
	case insecureNoVerify:
		ctrl.Log.Info("INSECURE: Image validation disabled")
		imageValidators = append(imageValidators, img.NewInsecureValidator("insecure always verified"))
	default:
		kc, err := img.GetKeychain(credSource)
		if err != nil {
			ctrl.Log.Error(err, "failed to get keychain for registry auth")
		}
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

	versionUpdater := controller.NewVersionUpdater(versionGetter, imageValidators, maintenanceTriggers, baseImage)

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

	ctrl.Log.Info("starting the updater", "version", kubeversionupdater.Version, "url", versionServerURL.String())

	if err := mgr.Start(ctx); err != nil {
		ctrl.Log.Error(err, "failed to start manager, exiting")
		os.Exit(1)
	}
}
