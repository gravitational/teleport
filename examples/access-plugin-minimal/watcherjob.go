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

	"github.com/gravitational/teleport-plugins/lib"
	"github.com/gravitational/teleport-plugins/lib/watcherjob"
	"github.com/gravitational/teleport/api/types"
)

func (g *googleSheetsPlugin) handleEvent(ctx context.Context, event types.Event) error {
	if event.Resource == nil {
		return nil
	}

	r := event.Resource.(types.AccessRequest)

	if r.GetState() == types.RequestState_PENDING {
		return g.createRow(r)
	}

	return g.updateSpreadsheet(r)
}

func (g *googleSheetsPlugin) run() error {
	ctx := context.Background()
	proc := lib.NewProcess(ctx)
	watcherJob := watcherjob.NewJob(
		g.teleportClient,
		watcherjob.Config{
			Watch: types.Watch{Kinds: []types.WatchKind{types.WatchKind{Kind: types.KindAccessRequest}}},
		},
		g.handleEvent,
	)

	proc.SpawnCriticalJob(watcherJob)

	fmt.Println("Started the watcher job")

	<-watcherJob.Done()

	fmt.Println("The watcher job is finished")

	return nil
}
