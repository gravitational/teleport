/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/trace"
)

type BenchCommand struct {
	benchDesktop *kingpin.CmdClause
}

func (b *BenchCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	bench := app.Command("bench", "Benchmarking tool for Teleport").Alias("benchmark").Hidden()

	benchDesktop := bench.Command("desktop", "Run benchmark for creating and deleting desktop resources").Hidden()
	b.benchDesktop = benchDesktop
}

func (b *BenchCommand) TryRun(ctx context.Context, cmd string, client *authclient.Client) (match bool, err error) {
	switch cmd {
	case b.benchDesktop.FullCommand():
		err = b.RunDesktopBench(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

func (b *BenchCommand) RunDesktopBench(ctx context.Context, client *authclient.Client) error {
	// Get a wds to benchmark against
	wdss, err := client.GetWindowsDesktopServices(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(wdss) == 0 {
		return trace.NotFound("no Windows Desktop Services found")
	}
	wds := wdss[0]
	hostID := wds.GetName()

	fmt.Printf("Benchmarking desktop creation against w_d_s: %s\n", hostID)

	const numDesktops = 1000
	const numRuns = 10

	var totalCreationTime time.Duration

	for run := 0; run < numRuns; run++ {
		creationTime, err := b.benchmarkDesktopUpsert(ctx, client, hostID, numDesktops)
		if err != nil {
			return trace.Wrap(err)
		}
		totalCreationTime += creationTime
	}

	avgCreationTime := totalCreationTime / numRuns

	fmt.Printf("Average creation time for %d desktops over %d runs: %v\n", numDesktops, numRuns, avgCreationTime)

	return nil
}

func (b *BenchCommand) benchmarkDesktopUpsert(ctx context.Context, client *authclient.Client, hostID string, numDesktops int) (time.Duration, error) {
	createdDesktops := make([]*types.WindowsDesktopV3, 0, numDesktops)
	defer func() {
		time.Sleep(3 * time.Second) // Give the created desktops a chance to propagate
		for _, desktop := range createdDesktops {
			err := client.DeleteWindowsDesktop(ctx, desktop.GetHostID(), desktop.GetName())
			if err != nil {
				fmt.Printf("Failed to delete desktop %s: %v\n", desktop.GetName(), err)
			}
		}
		fmt.Printf("Cleanup completed\n")
	}()

	startCreation := time.Now()
	for i := 0; i < numDesktops; i++ {
		// Create desktop resource
		desktopName := fmt.Sprintf("desktop-%d", i)
		desktop, err := types.NewWindowsDesktopV3(desktopName, nil /* labels */, types.WindowsDesktopSpecV3{
			Addr:   "test-addr",
			Domain: "test-domain",
			HostID: hostID,
			NonAD:  true,
		})
		if err != nil {
			return 0, trace.Wrap(err)
		}
		err = client.UpsertWindowsDesktop(ctx, desktop)
		if err != nil {
			return 0, trace.Wrap(err)
		}
		createdDesktops = append(createdDesktops, desktop)
	}
	creationTime := time.Since(startCreation)
	fmt.Printf("Creation completed in %v\n", creationTime)

	return creationTime, nil
}
