/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "6.1.0-beta.1"
	// InitTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
)

// BaseApp is responsible for handling the common features for a plugin.
// It will start a Teleport client, listen for events and treat them.
// It also handles signals and watches its thread.
// To instantiate a new BaseApp, use NewApp()
type BaseApp struct {
	PluginName string
	APIClient  teleport.Client
	Conf       PluginConfiguration
	Bot        MessagingBot
	Clock      clockwork.Clock

	apps    []App
	mainJob lib.ServiceJob

	*lib.Process
}

// NewApp creates a new BaseApp and initialize its main job
func NewApp(conf PluginConfiguration, pluginName string) *BaseApp {
	baseApp := BaseApp{
		PluginName: pluginName,
		Conf:       conf,
	}
	baseApp.mainJob = lib.NewServiceJob(baseApp.run)

	return &baseApp
}

// Run initializes and runs a watcher and a callback server
func (a *BaseApp) Run(ctx context.Context) error {
	// Initialize the process.
	a.Process = lib.NewProcess(ctx)
	a.SpawnCriticalJob(a.mainJob)
	<-a.Process.Done()
	return a.Err()
}

// Err returns the error app finished with.
func (a *BaseApp) Err() error {
	return trace.Wrap(a.mainJob.Err())
}

// WaitReady waits for http and watcher service to start up.
func (a *BaseApp) WaitReady(ctx context.Context) (bool, error) {
	return a.mainJob.WaitReady(ctx)
}

func (a *BaseApp) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
	log := logger.Get(ctx)
	log.Debug("Checking Teleport server version")

	pong, err := a.APIClient.Ping(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			return pong, trace.Wrap(err, "server version must be at least %s", minServerVersion)
		}
		return pong, trace.Wrap(err, "Unable to get Teleport server version")
	}
	err = utils.CheckVersion(pong.ServerVersion, minServerVersion)
	return pong, trace.Wrap(err)
}

// initTeleport creates a Teleport client and validates Teleport connectivity.
func (a *BaseApp) initTeleport(ctx context.Context, conf PluginConfiguration) (clusterName, webProxyAddr string, err error) {
	clt, err := conf.GetTeleportClient(ctx)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	a.APIClient = clt
	pong, err := a.checkTeleportVersion(ctx)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	if pong.ServerFeatures.AdvancedAccessWorkflows {
		webProxyAddr = pong.ProxyPublicAddr
	}

	return pong.ClusterName, webProxyAddr, nil
}

type App interface {
	Init(baseApp *BaseApp) error
	Start(process *lib.Process)
	WaitReady(ctx context.Context) (bool, error)
	WaitForDone()
	Err() error
}

// run starts the event watcher job and blocks utils it stops
func (a *BaseApp) run(ctx context.Context) error {
	log := logger.Get(ctx)

	if err := a.init(ctx); err != nil {
		return trace.Wrap(err)
	}

	for _, app := range a.apps {
		app.Start(a.Process)
	}

	allOK := true
	for _, app := range a.apps {
		ok, err := app.WaitReady(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		allOK = allOK && ok
		if !allOK {
			break
		}
	}

	a.mainJob.SetReady(allOK)
	if allOK {
		log.Info("Plugin is ready")
	} else {
		log.Error("Plugin is not ready")
	}

	for _, app := range a.apps {
		app.WaitForDone()
	}

	var allErrs []error
	for _, app := range a.apps {
		allErrs = append(allErrs, app.Err())
	}
	return trace.NewAggregate(allErrs...)
}

func (a *BaseApp) init(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	log := logger.Get(ctx)

	if a.Clock == nil {
		a.Clock = clockwork.NewRealClock()
	}

	clusterName, webProxyAddr, err := a.initTeleport(ctx, a.Conf)
	if err != nil {
		return trace.Wrap(err)
	}

	a.Bot, err = a.Conf.NewBot(clusterName, webProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	a.apps = a.Bot.SupportedApps()

	if len(a.apps) == 0 {
		return trace.BadParameter("no apps supported for this plugin")
	}

	for _, app := range a.apps {
		if err := app.Init(a); err != nil {
			return trace.Wrap(err)
		}
	}

	log.Debug("Starting API health check...")
	if err = a.Bot.CheckHealth(ctx); err != nil {
		return trace.Wrap(err, "API health check failed")
	}

	log.Debug("API health check finished ok")
	return nil
}
