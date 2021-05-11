package workflows_test

// this package adds godoc examples for several Client types and functions
// See https://pkg.go.dev/github.com/fluhus/godoc-tricks#Examples

import (
	"context"
	"log"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/workflows"
)

var ctx = context.Background()
var plugin *workflows.Plugin
var watcher workflows.RequestWatcher

func ExampleNewPlugin() {
	// Create a client with an open connection to a Teleport Auth server.
	// documentation on createing a Teleport client can be found at
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
	watcher, err := plugin.WatchRequests(ctx, workflows.Filter{
		State: workflows.StatePending,
	})
	if err != nil {
		log.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// you can wait for the watcher to init to catch initialization errors.
	if err := watcher.WaitInit(ctx, time.Second); err != nil {
		log.Fatalf("watcher failed to init: %v", err)
	}

	for {
		select {
		case event := <-watcher.Events():
			log.Printf("type: %v, request: %v", event.Request, event.Type)
			// handle request event ...
		case <-watcher.Done():
			err := watcher.Error()
			log.Fatalf("watcher closed: %v", err)
		}
	}
}

func ExampleRequestWatcher() {
	// you can wait for the watcher to init to catch initialization errors.
	if err := watcher.WaitInit(ctx, time.Second); err != nil {
		log.Fatalf("watcher failed to init: %v", err)
	}
	defer watcher.Close()

	// loop over events until the watcher is done.
	for {
		select {
		case event := <-watcher.Events():
			log.Printf("type: %v, request: %v", event.Request, event.Type)
			// handle request event ...
		case <-watcher.Done():
			err := watcher.Error()
			log.Fatalf("watcher closed: %v", err)
		}
	}
}

func ExampleNewRequestWatcher() {
	// Create a client with an open connection to a Teleport Auth server.
	// documentation on createing a Teleport client can be found at
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

	// Register a watcher for pending access requests.
	watcher, err := workflows.NewRequestWatcher(ctx, client, workflows.Filter{
		State: workflows.StatePending,
	})
	if err != nil {
		log.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// you can wait for the watcher to init to catch initialization errors.
	if err := watcher.WaitInit(ctx, time.Second); err != nil {
		log.Fatalf("watcher failed to init: %v", err)
	}

	for {
		select {
		case event := <-watcher.Events():
			log.Printf("type: %v, request: %v", event.Request, event.Type)
			// handle request event ...
		case <-watcher.Done():
			err := watcher.Error()
			log.Fatalf("watcher closed: %v", err)
		}
	}
}
