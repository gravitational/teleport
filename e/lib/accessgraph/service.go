/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package accessgraph

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// RegisterAccessGraphService registers the access graph service.
func RegisterAccessGraphService(cfg *servicecfg.Config, process *service.TeleportProcess, license *licensefile.LicenseFile) error {
	if !cfg.AccessGraph.Enabled {
		return nil
	}
	cfg.Log.Debug("Access Graph integration enabled")

	process.RegisterFunc("access-graph-service", func() error {
		cfg.Log.Info("Starting access graph service")

		accessGraphAddr := cfg.AccessGraph.Addr
		if accessGraphAddr == "" {
			cfg.Log.Error("access graph endpoint not configured")
			return trace.NotFound("access graph endpoint not configured")
		}
		const accessGraphRetryPeriod = 5 * time.Second

		// TODO(jakule): Very excessive retrying, but we need to make sure that
		// the access graph is initialized before we start serving requests.
		for {
			if err := initializeAndWatchAccessGraph(process.GracefulExitContext(),
				cfg.Log.WithField("Addr", accessGraphAddr),
				ServiceClientConfig{
					Addr:     accessGraphAddr,
					CA:       cfg.AccessGraph.CA,
					License:  license,
					Insecure: cfg.AccessGraph.Insecure,
				},
				process.GetAuthServer(), process.GetBackend()); err != nil {
				cfg.Log.Errorf("Failed to initialize access graph: %v", err)
				select {
				case <-process.GracefulExitContext().Done():
					return trace.Wrap(err)
				case <-time.After(accessGraphRetryPeriod):
					continue
				}
			}
		}
	})

	return nil
}
