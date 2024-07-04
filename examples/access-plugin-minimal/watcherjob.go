// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func (g *googleSheetsClient) HandleEvent(ctx context.Context, event types.Event) error {
	if event.Resource == nil {
		return nil
	}

	if _, ok := event.Resource.(*types.WatchStatusV1); ok {
		fmt.Println("Successfully started listening for Access Requests...")
		return nil
	}

	r, ok := event.Resource.(types.AccessRequest)
	if !ok {
		fmt.Printf("Unknown (%T) event received, skipping.\n", event.Resource)
		return nil
	}

	if r.GetState() == types.RequestState_PENDING {
		if err := g.createRow(r); err != nil {
			return err
		}
		fmt.Println("Successfully created a row")
		return nil
	}

	if err := g.updateSpreadsheet(r); err != nil {
		return err
	}
	fmt.Println("Successfully updated a spreadsheet row")
	return nil
}

func (p *AccessRequestPlugin) Run() error {
	ctx := context.Background()

	watch, err := p.TeleportClient.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			types.WatchKind{Kind: types.KindAccessRequest},
		},
	})

	if err != nil {
		return trace.Wrap(err)
	}
	defer watch.Close()

	fmt.Println("Starting the watcher job")

	for {
		select {
		case e := <-watch.Events():
			if err := p.EventHandler.HandleEvent(ctx, e); err != nil {
				return trace.Wrap(err)
			}
		case <-watch.Done():
			fmt.Println("The watcher job is finished")
			return nil
		}
	}
}
