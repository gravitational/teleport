/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/gravitational/trace"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/controller"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/img"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/maintenance"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/version"
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
	var disableLeaderElection bool

	flag.StringVar(&agentName, "agent-name", "", "The name of the agent that should be updated. This is mandatory.")
	flag.StringVar(&agentNamespace, "agent-namespace", "", "The namespace of the agent that should be updated. This is mandatory.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "healthz-addr", ":8081", "The address the probe endpoint binds to.")
	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Hour, "Operator sync period (format: https://pkg.go.dev/time#ParseDuration)")
	flag.BoolVar(&insecureNoVerify, "insecure-no-verify-image", false, "Disable image signature verification.")
	flag.BoolVar(&disableLeaderElection, "disable-leader-election", false, "Disable leader election, used when running the kube-agent-updater outside of Kubernetes.")
	flag.StringVar(&versionServer, "version-server", "https://updates.releases.teleport.dev/v1/", "URL of the HTTP server advertising target version and critical maintenances. Trailing slash is optional.")
	flag.StringVar(&versionChannel, "version-channel", "cloud/stable", "Version channel to get updates from.")
	flag.StringVar(&baseImageName, "base-image", "public.ecr.aws/gravitational/teleport", "Image reference containing registry and repository.")

	opts := zap.Options{
		Development: true,
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
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         !disableLeaderElection,
		LeaderElectionID:       agentName,
		Namespace:              agentNamespace,
		SyncPeriod:             &syncPeriod,
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: cache.SelectorsByObject{
				&appsv1.Deployment{}: {
					Field: fields.SelectorFromSet(fields.Set{"metadata.name": agentName}),
				},
				&appsv1.StatefulSet{}: {
					Field: fields.SelectorFromSet(fields.Set{"metadata.name": agentName}),
				},
			},
		}),
	})
	if err != nil {
		ctrl.Log.Error(err, "failed to create new manager, exiting")
		os.Exit(1)
	}

	versionServerURL, err := url.Parse(strings.TrimRight(versionServer, "/") + "/" + versionChannel)
	if err != nil {
		ctrl.Log.Error(err, "failed to pasre version server URL, exiting")
		os.Exit(1)
	}
	versionGetter := version.NewBasicHTTPVersionGetter(versionServerURL)
	maintenanceTriggers := maintenance.Triggers{
		maintenance.NewBasicHTTPMaintenanceTrigger("critical update", versionServerURL),
		maintenance.NewUnhealthyWorkloadTrigger("unhealthy pods", mgr.GetClient()),
		maintenance.NewWindowTrigger("maintenance window", mgr.GetClient()),
	}

	var imageValidators img.Validators
	if insecureNoVerify {
		ctrl.Log.Info("INSECURE: Image validation disabled")
		imageValidators = append(imageValidators, img.NewInsecureValidator("insecure always verify"))
	} else {
		validator, err := img.NewCosignSingleKeyValidator(teleportProdOCIPubKey, "cosign signature validator")
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

	if err := mgr.Start(ctx); err != nil {
		ctrl.Log.Error(err, "failed to start manager, exiting")
		os.Exit(1)
	}
}
