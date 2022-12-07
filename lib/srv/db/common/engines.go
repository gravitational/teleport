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

package common

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cloud"
)

var (
	// engines is a global database engines registry.
	engines map[string]EngineFn
	// enginesMu protects access to the global engines registry map.
	enginesMu sync.RWMutex
)

// EngineFn defines a database engine constructor function.
type EngineFn func(EngineConfig) Engine

// RegisterEngine registers a new engine constructor.
func RegisterEngine(fn EngineFn, names ...string) {
	enginesMu.Lock()
	defer enginesMu.Unlock()
	if engines == nil {
		engines = make(map[string]EngineFn)
	}
	for _, name := range names {
		engines[name] = fn
	}
}

// GetEngine returns a new engine for the provided configuration.
func GetEngine(name string, conf EngineConfig) (Engine, error) {
	if err := conf.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	enginesMu.RLock()
	engineFn := engines[name]
	enginesMu.RUnlock()
	if engineFn == nil {
		return nil, trace.NotFound("database engine %q is not registered", name)
	}
	return engineFn(conf), nil
}

// EngineConfig is the common configuration every database engine uses.
type EngineConfig struct {
	// Auth handles database access authentication.
	Auth Auth
	// Audit emits database access audit events.
	Audit Audit
	// AuthClient is the cluster auth server client.
	AuthClient *auth.Client
	// CloudClients provides access to cloud API clients.
	CloudClients cloud.Clients
	// Context is the database server close context.
	Context context.Context
	// Clock is the clock interface.
	Clock clockwork.Clock
	// Log is used for logging.
	Log logrus.FieldLogger
	// Users handles database users.
	Users Users
	// DataDir is the Teleport data directory
	DataDir string
}

// CheckAndSetDefaults validates the config and sets default values.
func (c *EngineConfig) CheckAndSetDefaults() error {
	if c.Auth == nil {
		return trace.BadParameter("engine config Auth is missing")
	}
	if c.Audit == nil {
		return trace.BadParameter("engine config Audit is missing")
	}
	if c.AuthClient == nil {
		return trace.BadParameter("engine config AuthClient is missing")
	}
	if c.CloudClients == nil {
		return trace.BadParameter("engine config CloudClients are missing")
	}
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Log == nil {
		c.Log = logrus.StandardLogger()
	}
	return nil
}
