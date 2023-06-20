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

	r := event.Resource.(types.AccessRequest)

	if r.GetState() == types.RequestState_PENDING {
		return g.createRow(r)
	}

	return g.updateSpreadsheet(r)
}

func (p *AccessRequestPlugin) Run() error {
	ctx := context.Background()

	watch, err := p.TeleportClient.NewWatcher(ctx, types.Watch{
		Name: "Access Requests",
		Kinds: []types.WatchKind{
			types.WatchKind{Kind: types.KindAccessRequest},
		},
	})

	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Starting the watcher job")

	for {
		select {
		case e := <-watch.Events():
			p.EventHandler.HandleEvent(ctx, e)
		case <-watch.Done():
			fmt.Println("The watcher job is finished")
			return nil
		}
	}
}
