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

//nolint:goimports,gci // goimports disagree with gci on blank imports. Remove when GCI is fixed upstream https://github.com/daixiang0/gci/issues/135
package main

import (
	"flag"
	"os"
	"time"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	resourcesv5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/embeddedtbot"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(resourcesv5.AddToScheme(scheme))
	utilruntime.Must(resourcesv3.AddToScheme(scheme))
	utilruntime.Must(resourcesv2.AddToScheme(scheme))
	utilruntime.Must(resourcesv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	utilruntime.Must(apiextv1.AddToScheme(scheme))
}

func main() {
	ctx := ctrl.SetupSignalHandler()

	config := &operatorConfig{}
	config.BindFlags(flag.CommandLine)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	botConfig := &embeddedtbot.BotConfig{}
	botConfig.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	err := config.CheckAndSetDefaults()
	if err != nil {
		setupLog.Error(err, "invalid configuration")
		os.Exit(1)
	}

	bot, err := embeddedtbot.New(botConfig)
	if err != nil {
		setupLog.Error(err, "unable to build tbot")
		os.Exit(1)
	}

	pong, err := bot.Preflight(ctx)
	if err != nil {
		setupLog.Error(err, "tbot preflight checks failed")
		os.Exit(1)
	}

	client, err := bot.StartAndWaitForClient(ctx, 15*time.Second)
	if err != nil {
		setupLog.Error(err, "error waiting the teleport client")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      config.metricsAddr,
		HealthProbeBindAddress:  config.probeAddr,
		LeaderElection:          true,
		LeaderElectionID:        config.leaderElectionID,
		LeaderElectionNamespace: config.namespace,
		PprofBindAddress:        config.pprofAddr,
		Cache: cache.Options{
			SyncPeriod: &config.syncPeriod,
			Namespaces: []string{config.namespace},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = mgr.Add(bot); err != nil {
		setupLog.Error(err, "unable to add tBot as a manager runnable")
		os.Exit(1)
	}

	if err = (&resources.RoleReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		TeleportClient: client,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TeleportRole")
		os.Exit(1)
	}

	if err = resources.NewUserReconciler(mgr.GetClient(), client).
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TeleportUser")
		os.Exit(1)
	}

	if err = resources.NewGithubConnectorReconciler(mgr.GetClient(), client).
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TeleportGithubConnector")
		os.Exit(1)
	}

	if pong.ServerFeatures.OIDC {
		if err = resources.NewOIDCConnectorReconciler(mgr.GetClient(), client).
			SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "TeleportOIDCConnector")
			os.Exit(1)
		}
	} else {
		setupLog.Info("OIDC connectors are only available in Teleport Enterprise edition. TeleportOIDCConnector resources won't be reconciled")
	}

	if pong.ServerFeatures.SAML {
		if err = resources.NewSAMLConnectorReconciler(mgr.GetClient(), client).
			SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "TeleportSAMLConnector")
			os.Exit(1)
		}
	} else {
		setupLog.Info("SAML connectors are only available in Teleport Enterprise edition. TeleportSAMLConnector resources won't be reconciled")
	}

	// Login Rules are enterprise-only but there is no specific feature flag for them.
	if pong.ServerFeatures.OIDC || pong.ServerFeatures.SAML {
		if err := resources.NewLoginRuleReconciler(mgr.GetClient(), client).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "TeleportLoginRule")
			os.Exit(1)
		}
	} else {
		setupLog.Info("Login Rules are only available in Teleport Enterprise edition. TeleportLoginRule resources won't be reconciled")
	}

	if err = resources.NewProvisionTokenReconciler(mgr.GetClient(), client).
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TeleportProvisionToken")
		os.Exit(1)
	}

	if err = resources.NewOktaImportRuleReconciler(mgr.GetClient(), client).
		SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TeleportOktaImportRule")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
