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

package services

import (
	"os"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
)

// InitOkta will initialize and start the Okta service.
func InitOkta(process *service.TeleportProcess) {
	process.RegisterWithAuthServer(types.RoleOkta, OktaIdentityEvent)
	process.RegisterCriticalFunc("okta.init", func() error {
		return initOktaService(process)
	})
}

func initOktaService(process *service.TeleportProcess) error {
	log := process.Config.Log.WithField(trace.Component, teleport.Component(
		eteleport.ComponentOkta, process.GetID()))

	conn, err := process.WaitForConnector(OktaIdentityEvent, log)
	if conn == nil {
		return trace.Wrap(err)
	}

	tokenBytes, err := os.ReadFile(process.Config.Okta.APITokenPath)
	if err != nil {
		return trace.Wrap(err)
	}

	// Remove any leading and trailing whitespace from the token.
	token := strings.TrimSpace(string(tokenBytes))

	accessPoint, err := newLocalCacheForOkta(process, conn.Client, []string{eteleport.ComponentOkta})
	if err != nil {
		return trace.Wrap(err)
	}

	// asyncEmitter makes sure that sessions do not block
	// in case if connections are slow
	asyncEmitter, err := process.NewAsyncEmitter(conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx := process.ExitContext()

	oktaService, err := okta.New(ctx, okta.Config{
		Log:             log,
		Emitter:         asyncEmitter,
		AccessPoint:     accessPoint,
		OktaAPIEndpoint: process.Config.Okta.APIEndpoint,
		OktaAPIToken:    token,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.OnExit("okta.stop", func(payload interface{}) {
		log.Info("Shutting down.")
		if oktaService != nil {
			oktaService.Stop()
		}
		if asyncEmitter != nil {
			if err := asyncEmitter.Close(); err != nil {
				log.Warnf("Error while closing emitter: %v", err)
			}
		}
		if err := conn.Close(); err != nil {
			log.Warnf("Error while closing connection: %v", err)
		}
		log.Info("Exited.")
	})

	process.BroadcastEvent(service.Event{Name: OktaReady, Payload: nil})

	if err := oktaService.Start(ctx); err != nil {
		return trace.Wrap(err)
	}
	log.Info("Okta service has successfully started")

	if err := oktaService.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// combinedOktaClient is an auth.Client client with services.Okta added to it.
type combinedOktaClient struct {
	auth.ClientI
	services.Okta
}

// newLocalCacheForOkta returns a new instance of access point for an Okta service.
func newLocalCacheForOkta(process *service.TeleportProcess, clt auth.ClientI, cacheName []string) (auth.OktaAccessPoint, error) {
	oktaClient := clt.OktaClient()
	client := combinedOktaClient{clt, oktaClient}
	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return client, nil
	}
	cache, err := process.NewLocalCache(clt, cache.ForOkta, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return auth.NewOktaWrapper(client, cache), nil
}
