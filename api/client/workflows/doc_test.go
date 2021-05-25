/*
Copyright 2021 Gravitational, Inc.

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

package workflows_test

// This test file adds godoc examples for workflows package
// See https://pkg.go.dev/github.com/fluhus/godoc-tricks#Examples

import (
	"context"
	"log"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/workflows"
	"github.com/gravitational/teleport/api/types"
)

var (
	ctx = context.Background()
	err error
	// predefined plugin
	plugin *workflows.Plugin
	// predefined watcher
	watcher *workflows.RequestWatcher
	// predefined request
	req types.AccessRequest
)

func ExamplePlugin() {
	// The plugin can be used to create, get, and update the state of requests.
	req, err := plugin.CreateRequest(ctx, "user", "admin")
	if err != nil {
		log.Fatalf("failed to create request: %v", err)
	}

	// PluginData can be stored directly on a request and retrieved later
	// in order for the plugin to maintain its own data while remaining stateless.
	pluginData := workflows.PluginDataMap{"data": "to track"}
	if err := plugin.UpdatePluginData(ctx, req.GetName(), pluginData, nil); err != nil {
		log.Fatalf("failed to update plugin data: %v", err)
	}

	// The plugin can also be used to watch for new Request events.
	watcher, err := plugin.WatchRequests(ctx, types.AccessRequestFilter{})
	if err != nil {
		log.Fatalf("failed to create watcher: %v", err)
	}

	for event := range watcher.Events() {
		log.Printf("type: %v, request: %v", event.Request, event.Type)
	}

}

func ExampleNewPlugin() {
	// Create a client with an open connection to a Teleport Auth server.
	// More documentation on creating a Teleport client can be found at
	// pkg.go.dev/github.com/gravitational/teleport/api/client#New.
	client, err := client.New(ctx, client.Config{
		Addrs: []string{"proxy.example.com:3080"},
		Credentials: []client.Credentials{
			client.LoadIdentityFile("path/to/identity_file"),
		},
		InsecureAddressDiscovery: false,
	})
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	plugin = workflows.NewPlugin(ctx, "plugin-name", client)
}

func ExamplePlugin_WatchRequests() {
	// Register a watcher for pending access requests.
	watcher, err = plugin.WatchRequests(ctx, types.AccessRequestFilter{
		State: types.RequestState_PENDING,
	})
	if err != nil {
		log.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()
}
func ExamplePlugin_CreateRequest() {
	req, err := plugin.CreateRequest(ctx, "alice", "admin-role1", "admin-role2")
	if err != nil {
		log.Fatalf("failed to create request: %v", err)
	}
	log.Printf("request created: %v", req)
}

func ExamplePlugin_GetRequest() {
	req, err := plugin.GetRequest(ctx, "reqID")
	if err != nil {
		log.Fatalf("failed to get request: %v", err)
	}
	log.Printf("request retrieved: %v", req)
}
func ExamplePlugin_GetRequests() {
	reqs, err := plugin.GetRequests(ctx, types.AccessRequestFilter{
		User:  "Alice",
		State: types.RequestState_PENDING,
	})
	if err != nil {
		log.Fatalf("failed to get requests: %v", err)
	}
	log.Printf("requests retrieved %v", reqs)
}
func ExamplePlugin_SetRequestState() {
	err := plugin.SetRequestState(ctx, req.GetName(), "manager1", types.AccessRequestUpdate{
		State:  types.RequestState_APPROVED,
		Roles:  []string{"admin-role2"},
		Reason: "Reason for request resolution.",
		Annotations: map[string][]string{
			"associatedTicketID": {"72"},
		},
	})
	if err != nil {
		log.Fatalf("failed to set request state: %v", err)
	}
}
func ExamplePlugin_GetPluginData() {
	data, err := plugin.GetPluginData(ctx, req.GetName())
	if err != nil {
		log.Fatalf("failed to get plugin data: %v", err)
	}
	log.Printf("retrieved plugin data: %v", data)
}
func ExamplePlugin_UpdatePluginData() {
	// Optionally retrieve expected plugin data to perform the update with
	// comparison. This will avoid data races from concurrent updates.
	expect, err := plugin.GetPluginData(ctx, req.GetName())
	if err != nil {
		log.Fatalf("failed to get plugin data: %v", err)
	}

	set := workflows.PluginDataMap{
		"data": "to track",
	}
	if err = plugin.UpdatePluginData(ctx, req.GetName(), set, expect); err != nil {
		log.Fatalf("failed to update plugin data: %v", err)
	}
}

func ExampleRequestWatcher() {
	// Use WaitInit to wait for the watcher to successfully initialize.
	if err := watcher.WaitInit(ctx); err != nil {
		log.Fatalf("watcher failed to initialize: %v", err)
	}

	// Loop over incoming events until the event channel is closed.
	for event := range watcher.Events() {
		log.Printf("type: %v, request: %v", event.Request, event.Type)
		switch event.Type {
		case types.OpPut:
			// OpPut indicates that the access request has been created or updated.
		case types.OpDelete:
			// OpDelete indicates that the access request has been removed.
		}
	}

	// After the watcher is closed, it should have an error.
	log.Fatalf("watcher closed: %v", watcher.Error())
}

func ExampleNewRequestWatcher() {
	// Create a client with an open connection to a Teleport Auth server.
	// More documentation on creating a Teleport client can be found at
	// pkg.go.dev/github.com/gravitational/teleport/api/client#New.
	client, err := client.New(ctx, client.Config{
		Addrs: []string{"proxy.example.com:3080"},
		Credentials: []client.Credentials{
			client.LoadIdentityFile("path/to/identity_file"),
		},
		InsecureAddressDiscovery: false,
	})
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	watcher, err = workflows.NewRequestWatcher(ctx, client, types.AccessRequestFilter{})
	if err != nil {
		log.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()
}

func ExampleRequestWatcher_WaitInit() {
	if err := watcher.WaitInit(ctx); err != nil {
		log.Fatalf("watcher failed to init: %v", err)
	}
}

func ExampleRequestWatcher_Events() {
	// Loop over incoming events until the event channel is closed.
	for event := range watcher.Events() {
		log.Printf("type: %v, request: %v", event.Request, event.Type)
		switch event.Type {
		case types.OpPut:
			// OpPut indicates that the access request has been created or updated.
		case types.OpDelete:
			// OpDelete indicates that the access request has been removed.
		}
	}
}
