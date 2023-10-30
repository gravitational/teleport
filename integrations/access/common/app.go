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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "6.1.0-beta.1"
	// grpcBackoffMaxDelay is a maximum time gRPC client waits before reconnection attempt.
	grpcBackoffMaxDelay = time.Second * 2
	// InitTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
)

type AppCreator[M MessagingBot] func() *App[M]

// BaseApp is responsible for handling the common features for a plugin.
// It will start a Teleport client, listen for events and treat them.
// It also handles signals and watches its thread.
// To instantiate a new BaseApp, use NewApp()
type BaseApp[M MessagingBot] struct {
	PluginName string
	APIClient  teleport.Client
	Conf       PluginConfiguration[M]
	Bot        M

	apps    []App[M]
	mainJob lib.ServiceJob

	*lib.Process
}

// NewApp creates a new BaseApp and initialize its main job
func NewApp[M MessagingBot](conf PluginConfiguration[M], pluginName string) *BaseApp[M] {
	baseApp := BaseApp[M]{
		PluginName: pluginName,
		Conf:       conf,
	}
	baseApp.mainJob = lib.NewServiceJob(baseApp.run)
	return &baseApp
}

func (a *BaseApp[M]) AddApp(app App[M]) *BaseApp[M] {
	a.apps = append(a.apps, app)
	return a
}

// Run initializes and runs a watcher and a callback server
func (a *BaseApp[_]) Run(ctx context.Context) error {
	// Initialize the process.
	a.Process = lib.NewProcess(ctx)
	a.SpawnCriticalJob(a.mainJob)
	<-a.Process.Done()
	return a.Err()
}

// Err returns the error app finished with.
func (a *BaseApp[_]) Err() error {
	return trace.Wrap(a.mainJob.Err())
}

// WaitReady waits for http and watcher service to start up.
func (a *BaseApp[_]) WaitReady(ctx context.Context) (bool, error) {
	return a.mainJob.WaitReady(ctx)
}

func (a *BaseApp[_]) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
	log := logger.Get(ctx)
	log.Debug("Checking Teleport server version")

	pong, err := a.APIClient.Ping(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			return pong, trace.Wrap(err, "server version must be at least %s", minServerVersion)
		}
		return pong, trace.Wrap(err, "Unable to get Teleport server version")
	}
	err = lib.AssertServerVersion(pong, minServerVersion)
	return pong, trace.Wrap(err)
}

// initTeleport creates a Teleport client and validates Teleport connectivity.
func (a *BaseApp[M]) initTeleport(ctx context.Context, conf PluginConfiguration[M]) (clusterName, webProxyAddr string, err error) {
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

type App[M MessagingBot] interface {
	Init(baseApp *BaseApp[M]) error
	Start(ctx context.Context, process *lib.Process) error
	WaitReady(ctx context.Context) (bool, error)
	WaitForDone()
	Err() error
}

// run starts the event watcher job and blocks utils it stops
func (a *BaseApp[_]) run(ctx context.Context) error {
	log := logger.Get(ctx)

	if err := a.init(ctx); err != nil {
		return trace.Wrap(err)
	}

	for _, app := range a.apps {
		if err := app.Start(ctx, a.Process); err != nil {
			return trace.Wrap(err)
		}
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

func (a *BaseApp[_]) init(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	log := logger.Get(ctx)

	clusterName, webProxyAddr, err := a.initTeleport(ctx, a.Conf)
	if err != nil {
		return trace.Wrap(err)
	}

	a.Bot, err = a.Conf.NewBot(clusterName, webProxyAddr)
	if err != nil {
		return trace.Wrap(err)
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
