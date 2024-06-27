/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

// LoadtestCommand implements the `tctl loadtest` family of commands.
type LoadtestCommand struct {
	config *servicecfg.Config

	nodeHeartbeats *kingpin.CmdClause

	watch *kingpin.CmdClause

	count       int
	churn       int
	labels      int
	interval    time.Duration
	ttl         time.Duration
	concurrency int

	kind string
}

// Initialize allows LoadtestCommand to plug itself into the CLI parser
func (c *LoadtestCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.config = config
	loadtest := app.Command("loadtest", "Tools for generating artificial load").Hidden()

	c.nodeHeartbeats = loadtest.Command("node-heartbeats", "Generate artificial node heartbeats").Hidden()
	c.nodeHeartbeats.Flag("count", "Number of unique nodes to heartbeat").Default("10000").IntVar(&c.count)
	c.nodeHeartbeats.Flag("churn", "Number of nodes to churn each round").Default("0").IntVar(&c.churn)
	c.nodeHeartbeats.Flag("labels", "Number of labels to generate per node.").Default("1").IntVar(&c.labels)
	c.nodeHeartbeats.Flag("interval", "Node heartbeat interval").Default("1m").DurationVar(&c.interval)
	c.nodeHeartbeats.Flag("ttl", "TTL of heartbeated nodes").Default("10m").DurationVar(&c.ttl)
	c.nodeHeartbeats.Flag("concurrency", "Max concurrent requests").Default(
		strconv.Itoa(runtime.NumCPU() * 16),
	).IntVar(&c.concurrency)

	c.watch = loadtest.Command("watch", "Monitor event stream").Hidden()
	c.watch.Flag("kind", "Resource kind(s) to watch, e.g. --kind=node,user,role").StringVar(&c.kind)
}

// TryRun takes the CLI command as an argument (like "loadtest node-heartbeats") and executes it.
func (c *LoadtestCommand) TryRun(ctx context.Context, cmd string, client *authclient.Client) (match bool, err error) {
	switch cmd {
	case c.nodeHeartbeats.FullCommand():
		err = c.NodeHeartbeats(ctx, client)
	case c.watch.FullCommand():
		err = c.Watch(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

func (c *LoadtestCommand) NodeHeartbeats(ctx context.Context, client *authclient.Client) error {
	infof := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "[i] "+format+"\n", args...)
	}
	warnf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "[!] "+format+"\n", args...)
	}

	infof("Setting up node hb load generation. count=%d, churn=%d, labels=%d, interval=%s, ttl=%s, concurrency=%d",
		c.count,
		c.churn,
		c.labels,
		c.interval,
		c.ttl,
		c.concurrency,
	)

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	node_ids := make([]string, 0, c.count)

	for i := 0; i < c.count; i++ {
		node_ids = append(node_ids, uuid.New().String())
	}

	labels := make(map[string]string, c.labels)
	for i := 0; i < c.labels; i++ {
		labels[uuid.New().String()] = uuid.New().String()
	}

	// allocate twice the expected count so that we have sufficient room to support up
	// to one full round worth of backlog.
	workch := make(chan string, c.count*2)
	defer close(workch)

	var errct atomic.Uint64
	errch := make(chan error, 1)

	mknode := func(id string) types.Server {
		node, err := types.NewServer(id, types.KindNode, types.ServerSpecV2{Hostname: uuid.New().String()})
		if err != nil {
			panic(err)
		}
		node.SetExpiry(time.Now().Add(c.ttl))
		node.SetStaticLabels(labels)

		return node
	}

	sampleNode := mknode(uuid.New().String())

	sn, err := services.MarshalServer(sampleNode)
	if err != nil {
		return trace.Wrap(err)
	}

	infof("Estimated serialized node size: %d (bytes)", len(sn))

	for i := 0; i < c.concurrency; i++ {
		go func() {
			for id := range workch {
				_, err = client.UpsertNode(ctx, mknode(id))
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					log.Debugf("Failed to upsert node: %v", err)
					select {
					case errch <- err:
					default:
					}
					errct.Add(1)
				}
			}
		}()
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	var generation uint64
	for {
		if backlog := len(workch); backlog != 0 {
			warnf("Backlog in heartbeat emission. size=%d", backlog)
		}

		select {
		case err := <-errch:
			warnf("Error during last round of heartbeats: %v", err)
		default:
		}

		for i := 0; i < c.churn; i++ {
			node_ids = append(node_ids, uuid.New().String())
		}

		node_ids = node_ids[c.churn:]

		for _, id := range node_ids {
			select {
			case workch <- id:
			default:
				panic("too much backlog!")
			}
		}

		generation++

		infof("Queued heartbeat batch for emission. generation=%d, errors=%d", generation, errct.Load())

		select {
		case <-ticker.C:
		case <-ctx.Done():
			fmt.Println("")
			return nil
		}
	}
}

func (c *LoadtestCommand) Watch(ctx context.Context, client *authclient.Client) error {
	var kinds []types.WatchKind
	for _, kind := range strings.Split(c.kind, ",") {
		kind = strings.TrimSpace(kind)
		if kind == "" {
			continue
		}

		kinds = append(kinds, types.WatchKind{
			Kind: kind,
		})
	}

	var allowPartialSuccess bool
	if len(kinds) == 0 {
		// use auth watch kinds by default
		ccfg := cache.ForAuth(cache.Config{})
		kinds = ccfg.Watches
		allowPartialSuccess = true
	}

	watcher, err := client.NewWatcher(ctx, types.Watch{
		Name:                "tctl-watch",
		Kinds:               kinds,
		AllowPartialSuccess: allowPartialSuccess,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	defer watcher.Close()

	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}

		var skinds []string
		for _, k := range event.Resource.(types.WatchStatus).GetKinds() {
			skinds = append(skinds, k.Kind)
		}

		fmt.Printf("INIT: %v\n", skinds)
	case <-watcher.Done():
		return trace.Errorf("failed to get init event: %v", watcher.Error())
	}

	for {
		select {
		case event := <-watcher.Events():
			switch event.Type {
			case types.OpPut:
				printEvent("PUT", event.Resource)
			case types.OpDelete:
				printEvent("DEL", event.Resource)
			default:
				return trace.BadParameter("expected put or del event, got %v instead", event.Type)
			}
		case <-watcher.Done():
			if ctx.Err() != nil {
				// canceled by caller
				return nil
			}
			return trace.Errorf("watcher exited unexpectedly: %v", watcher.Error())
		}
	}
}

func printEvent(ekind string, rsc types.Resource) {
	if sk := rsc.GetSubKind(); sk != "" {
		fmt.Printf("%s: %s/%s/%s\n", ekind, rsc.GetKind(), sk, rsc.GetName())
	} else {
		fmt.Printf("%s: %s/%s\n", ekind, rsc.GetKind(), rsc.GetName())
	}
}
