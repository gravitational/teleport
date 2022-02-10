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
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gravitational/trace"
)

type clusterClient struct {
	redis.ClusterClient
}

func (c *clusterClient) Process(ctx context.Context, inCmd redis.Cmder) error {
	cmd, ok := inCmd.(*redis.Cmd)
	if !ok {
		return trace.BadParameter("failed to cast Redis command")
	}

	switch cmdName := strings.ToLower(cmd.Name()); cmdName {
	case "multi", "exec", "watch":
		return trace.NotImplemented("%s is not supported in the cluster mode", cmdName)
	case "dbsize":
		res := c.DBSize(ctx)
		cmd.SetVal(res.Val())
		cmd.SetErr(res.Err())

		return nil
	case "scan":
		var resultsKeys []string
		var mtx sync.Mutex

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

		cmd := redis.NewCmd(ctx, cmd.Args())
		cmd.SetVal(resultsKeys)

		return nil
	case "keys":
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
		// TODO(jakub) mset may work, but without consistency guaranty
	case "mget":
		if len(cmd.Args()) == 1 {
			return trace.BadParameter("wrong number of arguments for 'script' command")
		}

		var resultsKeys []interface{}
		var mtx sync.Mutex

		keys := cmd.Args()[1:]
		for _, k := range keys {
			result := c.Get(ctx, k.(string))
			if result.Err() == redis.Nil {
				mtx.Lock()
				resultsKeys = append(resultsKeys, redis.Nil)
				mtx.Unlock()
				continue
			}

			if result.Err() != nil {
				cmd.SetErr(result.Err())
				return trace.Wrap(result.Err())
			}

			mtx.Lock()
			resultsKeys = append(resultsKeys, result.Val())
			mtx.Unlock()
		}

		cmd.SetVal(resultsKeys)

		return nil
	case "flushall", "flushdb":
		if err := c.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			singleCmd := redis.NewCmd(ctx, cmd.Args()...)
			err := client.Process(ctx, singleCmd)
			if err != nil {
				return trace.Wrap(err)
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
