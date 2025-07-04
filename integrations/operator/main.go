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
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-logr/logr"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/integrations/lib/embeddedtbot"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	tbotconfig "github.com/gravitational/teleport/lib/tbot/config"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var (
	scheme = controllers.Scheme
)

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
	setupLog := logger.WithName("setup")

	config := &operatorConfig{}
	config.BindFlags(flag.CommandLine)
	botConfig := &embeddedtbot.BotConfig{}
	botConfig.AuthServerAddressMode = tbotconfig.AllowProxyAsAuthServer
	botConfig.BindFlags(flag.CommandLine)
	flag.Parse()

	// Now that we parsed the flags, we can tune the log level.
	var logLevel slog.Level
	if err := (&logLevel).UnmarshalText([]byte(config.logLevel)); err != nil {
		setupLog.Error(err, "Failed to parse log level", "level", config.logLevel)
		os.Exit(1)
	}
	slogLeveler.Set(logLevel)

	err = config.CheckAndSetDefaults()
	if err != nil {
		setupLog.Error(err, "invalid configuration")
		os.Exit(1)
	}

	bot, err := embeddedtbot.New(botConfig, slogLogger.With(teleport.ComponentLabel, "embedded-tbot"))
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
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: config.metricsAddr,
		},
		HealthProbeBindAddress:  config.probeAddr,
		LeaderElection:          true,
		LeaderElectionID:        config.leaderElectionID,
		LeaderElectionNamespace: config.namespace,
		PprofBindAddress:        config.pprofAddr,
		Cache: cache.Options{
			SyncPeriod: &config.syncPeriod,
			DefaultNamespaces: map[string]cache.Config{
				config.namespace: {},
			},
		},
		// All our controllers now use unstructured objects, we need to cache them.
		Client: ctrlclient.Options{Cache: &ctrlclient.CacheOptions{Unstructured: true}},
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err = mgr.Add(bot); err != nil {
		setupLog.Error(err, "unable to add tBot as a manager runnable")
		os.Exit(1)
	}

	if err = resources.SetupAllControllers(setupLog, mgr, client, pong.ServerFeatures); err != nil {
		setupLog.Error(err, "failed to setup controllers")
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
