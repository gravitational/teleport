/*
Copyright 2019 Gravitational, Inc.

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

// Example whitelist based access plugin.
//
// This plugin approves/denies access requests based on a simple whitelist
// of usernames. Requests from whitelisted users are approved, all others
// are denied.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/workflows"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"
)

func main() {
	ctx := context.Background()

	cfg, err := loadConfig("config.toml")
	if err != nil {
		log.Printf("ERROR: %s", err)
		os.Exit(1)
	}

	if err := run(ctx, cfg); err != nil {
		log.Printf("ERROR: %s", err)
		os.Exit(1)
	}
}

type Config struct {
	Addr         string   `toml:"addr"`
	IdentityFile string   `toml:"identity_file"`
	Whitelist    []string `toml:"whitelist"`
}

func loadConfig(filepath string) (*Config, error) {
	t, err := toml.LoadFile(filepath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conf := &Config{}
	if err := t.Unmarshal(conf); err != nil {
		return nil, trace.Wrap(err)
	}
	return conf, nil
}

func run(ctx context.Context, cfg *Config) error {
	// Create client for plugin to use.
	client, err := client.New(ctx, client.Config{
		Addrs: []string{cfg.Addr},
		Credentials: []client.Credentials{
			client.LoadIdentityFile(cfg.IdentityFile),
		},
		InsecureAddressDiscovery: true,
		DialTimeout:              time.Second * 5,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	// Create new plugin named "example".
	plugin := workflows.NewPlugin(ctx, "example", client)

	// Register a watcher for pending access requests.
	watcher, err := plugin.WatchRequests(ctx, types.AccessRequestFilter{
		State: types.RequestState_PENDING,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Wait for watcher to finish initializing, timing out after 5 seconds.
	cancelCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	err = watcher.WaitInit(cancelCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Printf("watcher initialized...")
	defer watcher.Close()

	for {
		select {
		case event := <-watcher.Events():
			switch event.Type {
			case types.OpPut:
				// OpPut indicates that a request has been created or updated. Since we specified
				// StatePending in our filter, only pending requests should appear here.
				log.Printf("Handling request: %+v", event.Request)
				whitelisted := false
			CheckWhitelist:
				for _, user := range cfg.Whitelist {
					if event.Request.GetUser() == user {
						whitelisted = true
						break CheckWhitelist
					}
				}
				params := types.AccessRequestUpdate{
					Annotations: map[string][]string{
						"strategy": {"whitelist"},
					},
				}
				if whitelisted {
					log.Printf("User %q in whitelist, approving request...", event.Request.GetUser())
					params.State = types.RequestState_APPROVED
					params.Reason = "user in whitelist"
				} else {
					log.Printf("User %q not in whitelist, denying request...", event.Request.GetUser())
					params.State = types.RequestState_DENIED
					params.Reason = "user not in whitelist"
				}
				if err := plugin.SetRequestState(ctx, event.Request.GetName(), "delegator", params); err != nil {
					return trace.Wrap(err)
				}
				log.Printf("Request state set: %v.", params.State)
			case types.OpDelete:
				// request has been removed (expired).
				// Due to some limitations in Teleport's event system, filters
				// don't really work with OpDelete events. As such, we may get
				// OpDelete events for requests that would not typically match
				// the filter argument we supplied above.
				log.Printf("Request %s has automatically expired.", event.Request.GetName())
			}
		case <-watcher.Done():
			return watcher.Error()
		}
	}
}
