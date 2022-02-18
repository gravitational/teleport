/*

 Copyright 2022 Gravitational, Inc.

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

package redis

import (
	"context"
	"crypto/tls"
	"strconv"
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gravitational/trace"
)

// Commands with additional processing in Teleport when using cluster mode.
const (
	multiCmd    = "multi"
	execCmd     = "exec"
	watchCmd    = "watch"
	dbsizeCmd   = "dbsize"
	scanCmd     = "scan"
	keysCmd     = "keys"
	mgetCmd     = "mget"
	flushallCmd = "flushall"
	flushdbCmd  = "flushdb"
)

// clusterClient is a wrapper around redis.ClusterClient
type clusterClient struct {
	redis.ClusterClient
}

// newClient creates a new Redis client based on given ConnectionMode. If connection mode is not supported
// an error is returned.
func newClient(ctx context.Context, mode ConnectionMode, addr string, tlsConfig *tls.Config) (redis.UniversalClient, error) {
	// TODO(jakub): Use system CA bundle if connecting to AWS.
	// TODO(jakub): Investigate Redis Sentinel.
	switch mode {
	case Standalone:
		return redis.NewClient(&redis.Options{
			Addr:      addr,
			TLSConfig: tlsConfig,
		}), nil
	case Cluster:
		client := &clusterClient{
			ClusterClient: *redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:     []string{addr},
				TLSConfig: tlsConfig,
			}),
		}
		// Load cluster information.
		client.ReloadState(ctx)

		return client, nil
	default:
		// We've checked that while validating the config, but checking again can help with regression.
		return nil, trace.BadParameter("incorrect connection mode %s", mode)
	}
}

// Process add supports for additional cluster commands. Our Redis implementation passes most commands to
// go-redis `Process()` function which doesn't handel all Cluster commands like for ex. DBSIZE, FLUSHDB, etc.
// This "override" adds
func (c *clusterClient) Process(ctx context.Context, inCmd redis.Cmder) error {
	cmd, ok := inCmd.(*redis.Cmd)
	if !ok {
		return trace.BadParameter("failed to cast Redis command type: %T", cmd)
	}

	switch cmdName := strings.ToLower(cmd.Name()); cmdName {
	case multiCmd, execCmd, watchCmd:
		// do not allow transaction commands as they always fail on exec.
		return trace.NotImplemented("%s is not supported in the cluster mode", cmdName)
	case dbsizeCmd:
		// use go-redis dbsize implementation. It returns size of all keys in the whole cluster instead of
		// just currently connected instance.
		res := c.DBSize(ctx)
		cmd.SetVal(res.Val())
		cmd.SetErr(res.Err())

		return nil
	case scanCmd:
		var resultsKeys []string
		var mtx sync.Mutex

		// ForEachMaster is called in parallel, so locking inside is needed.
		if err := c.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			scanCmd := redis.NewScanCmd(ctx, client.Process, cmd.Args()...)
			err := client.Process(ctx, scanCmd)
			if err != nil {
				return trace.Wrap(err)
			}

			keys, _, err := scanCmd.Result()
			if err != nil {
				return trace.Wrap(err)
			}

			mtx.Lock()
			resultsKeys = append(resultsKeys, keys...)
			mtx.Unlock()

			return nil
		}); err != nil {
			return trace.Wrap(err)
		}

		// TODO(jakule): rethink iterator issue.
		cmd.SetVal([]interface{}{strconv.FormatInt(0, 10), resultsKeys})

		return nil
	case keysCmd:
		var resultsKeys []string
		var mtx sync.Mutex

		if err := c.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			scanCmd := redis.NewStringSliceCmd(ctx, cmd.Args()...)
			err := client.Process(ctx, scanCmd)
			if err != nil {
				return trace.Wrap(err)
			}

			keys, err := scanCmd.Result()
			if err != nil {
				return trace.Wrap(err)
			}

			mtx.Lock()
			resultsKeys = append(resultsKeys, keys...)
			mtx.Unlock()

			return nil
		}); err != nil {
			return trace.Wrap(err)
		}

		cmd.SetVal(resultsKeys)

		return nil
	case mgetCmd:
		if len(cmd.Args()) == 1 {
			return trace.BadParameter("wrong number of arguments for 'mget' command")
		}

		var resultsKeys []interface{}

		keys := cmd.Args()[1:]
		for _, k := range keys {
			result := c.Get(ctx, k.(string))
			if result.Err() == redis.Nil {
				resultsKeys = append(resultsKeys, redis.Nil)
				continue
			}

			if result.Err() != nil {
				cmd.SetErr(result.Err())
				return trace.Wrap(result.Err())
			}

			resultsKeys = append(resultsKeys, result.Val())
		}

		cmd.SetVal(resultsKeys)

		return nil
	case flushallCmd, flushdbCmd:
		var mtx sync.Mutex

		if err := c.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			singleCmd := redis.NewCmd(ctx, cmd.Args()...)
			err := client.Process(ctx, singleCmd)
			if err != nil {
				return trace.Wrap(err)
			}

			mtx.Lock()
			defer mtx.Unlock()

			if cmd.Err() != nil {
				// If other call have already set error do not erase it.
				// It should be returned to the caller.
				return nil
			}

			cmd.SetVal(singleCmd.Val())
			cmd.SetErr(singleCmd.Err())

			return nil
		}); err != nil {
			return trace.Wrap(err)
		}

		return nil
	default:
		return c.ClusterClient.Process(ctx, cmd)
	}
}
