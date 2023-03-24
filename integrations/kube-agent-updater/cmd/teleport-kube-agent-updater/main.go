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
	"os"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/gravitational/trace"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	runtimescheme "sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/controller"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/img"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/maintenance"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/version"
)

var (
	SchemeBuilder = &runtimescheme.Builder{GroupVersion: appsv1.SchemeGroupVersion}
	scheme        = runtime.NewScheme()
)

func init() {
	SchemeBuilder.Register(
		&appsv1.Deployment{},
		&appsv1.DeploymentList{},
		&appsv1.StatefulSet{},
		&appsv1.StatefulSetList{},
	)
	utilruntime.Must(SchemeBuilder.AddToScheme(scheme))
}

func main() {
	ctx := ctrl.SetupSignalHandler()

	var agentName string
	var agentNamespace string
	var metricsAddr string
	var probeAddr string
	var syncPeriod time.Duration

	flag.StringVar(&agentName, "agent-name", "", "The name of the agent that should be updated. This is mandatory.")
	flag.StringVar(&agentNamespace, "agent-namespace", "", "The namespace of the agent that should be updated. This is mandatory.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "healthz-addr", ":8081", "The address the probe endpoint binds to.")
	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Hour, "Operator sync period (format: https://pkg.go.dev/time#ParseDuration)")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if agentName == "" {
		ctrl.Log.Error(trace.BadParameter("--agent-name empty"), "agent-name must be provided")
	}
	if agentNamespace == "" {
		ctrl.Log.Error(trace.BadParameter("--agent-namespace empty"), "agent-namespace must be provided")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         true,
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

	// TODO: replace those mocks by the real thing
	versionGetter := version.NewGetterMock("12.0.3", nil)
	imageValidators := []img.Validator{
		img.NewImageValidatorMock("mock", true, img.NewImageRef("", "", "", "")),
	}
	maintenanceTriggers := []maintenance.Trigger{
		maintenance.NewMaintenanceTriggerMock("never", false),
	}
	baseImage, _ := reference.ParseNamed("public.ecr.aws/trent-playground/gravitational/teleport")
	// End of mocks

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

	if err := mgr.Start(ctx); err != nil {
		ctrl.Log.Error(err, "failed to start manager, exiting")
		os.Exit(1)
	}
}
