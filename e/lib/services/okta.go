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
	"context"
	"os"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
)

// oktaDependEvents is a list of events that the Okta service depends on.
var oktaDependEvents = []string{
	service.AuthTLSReady,
	service.AuthIdentityEvent,
	service.ProxySSHReady,
	service.ProxyWebServerReady,
	service.ProxyReverseTunnelReady,
}

// InitOkta will initialize and start the Okta service.
func InitOkta(process *service.TeleportProcess) {
	process.RegisterWithAuthServer(types.RoleOkta, OktaIdentityEvent)
	process.RegisterCriticalFunc("okta.init", func() error {
		return initOktaService(process)
	})
}

func initOktaService(process *service.TeleportProcess) error {
	ctx := process.ExitContext()

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

	resp, err := conn.Client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// If this process connected through the web proxy, it will discover the
	// reverse tunnel address correctly and store it in the connector.
	//
	// If it was not, it is running in single process mode which is used for
	// development and demos. In that case, wait until all dependencies (like
	// auth and reverse tunnel server) are ready before starting.
	tunnelAddrResolver := conn.TunnelProxyResolver()
	if tunnelAddrResolver == nil {
		tunnelAddrResolver = process.SingleProcessModeResolver(resp.GetProxyListenerMode())

		// run the resolver. this will check configuration for errors.
		_, _, err := tunnelAddrResolver(process.ExitContext())
		if err != nil {
			return trace.Wrap(err)
		}

		// Block and wait for all dependencies to start before starting.
		log.Debugf("Waiting for Okta service dependencies to start.")
		for _, event := range oktaDependEvents {
			_, err := process.WaitForEvent(process.ExitContext(), event)
			if err != nil {
				log.Debugf("Process is exiting.")
				break
			}
		}
		log.Debugf("Okta service dependencies have started, continuing.")
	}

	// asyncEmitter makes sure that sessions do not block
	// in case if connections are slow
	asyncEmitter, err := process.NewAsyncEmitter(conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyGetter := reversetunnel.NewConnectedProxyGetter()

	oktaService, err := okta.New(ctx, okta.Config{
		Log:             log,
		Hostname:        process.Config.Hostname,
		HostID:          process.Config.HostUUID,
		RotationGetter:  process.GetRotation,
		Emitter:         asyncEmitter,
		AccessPoint:     accessPoint,
		OnHeartbeat:     process.OnHeartbeat(teleport.Okta),
		OktaAPIEndpoint: process.Config.Okta.APIEndpoint,
		OktaAPIToken:    token,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.OnExit("okta.stop", func(payload interface{}) {
		log.Info("Shutting down.")
		var ctx context.Context
		if payload != nil {
			payloadCtx, ok := payload.(context.Context)
			if ok {
				ctx = payloadCtx
			}
		}

		if oktaService != nil {
			if err := oktaService.Shutdown(); err != nil {
				log.Errorf("Error shutting down Okta service: %v", err)
			}

			if ctx != nil {
				if err := oktaService.Close(ctx); err != nil {
					log.Errorf("Error closing Okta service: %v", err)
				}
			}
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

	clusterName := conn.ServerIdentity.ClusterName

	// Create and start an agent pool.
	agentPool, err := reversetunnel.NewAgentPool(
		process.ExitContext(),
		reversetunnel.AgentPoolConfig{
			Component:            eteleport.ComponentOkta,
			HostUUID:             conn.ServerIdentity.ID.HostUUID,
			Resolver:             tunnelAddrResolver,
			Client:               conn.Client,
			Server:               oktaService,
			AccessPoint:          accessPoint,
			HostSigner:           conn.ServerIdentity.KeySigner,
			Cluster:              clusterName,
			FIPS:                 process.Config.FIPS,
			ConnectedProxyGetter: proxyGetter,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	err = agentPool.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	oktaService.Wait(ctx)
	agentPool.Stop()
	agentPool.Wait()

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
