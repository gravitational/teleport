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
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	teleport "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
)

const (
	desktopExpiry     = 10 * time.Minute
	heartbeatInterval = 3 * time.Minute
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Create a Teleport API client, loading credentials
	// from an existing tsh profile on disk.
	//
	// The user that these creds are bound to must have a role
	// granting create and update permissions on the windows_desktop
	// resource.
	clt, err := teleport.New(ctx, teleport.Config{
		Credentials: []teleport.Credentials{teleport.LoadProfile("", "")},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer clt.Close()

	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := heartbeatDesktops(ctx, clt); err != nil {
				// In this example, we just log the error and will try again
				// on the next tick.
				log.Println("[ERROR]:", err)
			}
		}
	}
}

// desktops is the list of desktops we're going to register.
// They are hardcoded here for simplicity, but could be sourced
// from some other system of record.
var desktops = map[string]string{
	"desktop-1": "192.168.1.104",
	"desktop-2": "192.168.1.105",
}

// getDesktopServices returns the name (ID) of all registered
// Windows Desktop services in the cluster.
func getDesktopServices(ctx context.Context, clt *teleport.Client) ([]string, error) {
	// Note: in this example, we fetch the set of desktop services every few minutes.
	// This approach is suitable for smaller clusters, with a handful of desktop services.
	// In larger environments, you may choose to use a watcher to maintain the set of
	// active desktop services rather than repeatedly fetching them.
	services, err := clt.GetWindowsDesktopServices(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(services))
	for _, svc := range services {
		ids = append(ids, svc.GetName())
	}

	return ids, nil
}

func heartbeatDesktops(ctx context.Context, clt *teleport.Client) error {
	// Before we heartbeat, get the list of Windows Desktop Services.
	// We'll tell Teleport that all the desktops we register can be reached by
	// all of the services that exist in the cluster.
	svcs, err := getDesktopServices(ctx, clt)
	if err != nil {
		return fmt.Errorf("couldn't get desktop services: %w", err)
	}

	for _, svc := range svcs {
		for name, addr := range desktops {
			desktop, err := types.NewWindowsDesktopV3(
				name,
				nil, // no labels
				types.WindowsDesktopSpecV3{
					Addr: addr,

					// HostID identifies the windows_desktop_service(s)
					// that are able to proxy traffic to this desktop.
					// In this example, we assume all services have access
					// to all desktops, so this registration is repeated
					// for every HostID.
					HostID: svc,

					// in this example, we're assuming Active Directory
					// is not in use, so we leave the Domain empty and
					// set NonAD
					Domain: "",
					NonAD:  true,
				},
			)
			if err != nil {
				return fmt.Errorf("couldn't create desktop %v: %w", name, err)
			}

			// Set an expiration of 10 minutes. This means the desktop will
			// disappear from Teleport in 10 minutes unless we heartbeat again
			// before then.
			desktop.SetExpiry(time.Now().Add(desktopExpiry))

			if err := clt.UpsertWindowsDesktop(ctx, desktop); err != nil {
				return fmt.Errorf("couldn't heartbeat desktop %v: %w", name, err)
			}
		}
	}

	return nil
}
