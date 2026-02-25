/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package auth

import (
	"sort"
	"sync"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
)

// GeneratedGRPCServiceRegistrar registers a generated gRPC service.
type GeneratedGRPCServiceRegistrar func(server *grpc.Server, cfg GRPCServerConfig) error

var generatedGRPCServiceRegistrars struct {
	mu sync.RWMutex
	m  map[string]GeneratedGRPCServiceRegistrar
}

// RegisterGeneratedGRPCService registers a generated gRPC service registrar.
//
// This function is intended to be called from generated files using init().
func RegisterGeneratedGRPCService(name string, registrar GeneratedGRPCServiceRegistrar) {
	if name == "" {
		panic("auth: generated gRPC service name is required")
	}
	if registrar == nil {
		panic("auth: generated gRPC service registrar is nil")
	}

	generatedGRPCServiceRegistrars.mu.Lock()
	defer generatedGRPCServiceRegistrars.mu.Unlock()

	if generatedGRPCServiceRegistrars.m == nil {
		generatedGRPCServiceRegistrars.m = make(map[string]GeneratedGRPCServiceRegistrar)
	}
	if _, exists := generatedGRPCServiceRegistrars.m[name]; exists {
		panic("auth: duplicate generated gRPC service registrar: " + name)
	}
	generatedGRPCServiceRegistrars.m[name] = registrar
}

func runGeneratedGRPCServiceRegistrars(server *grpc.Server, cfg GRPCServerConfig) error {
	generatedGRPCServiceRegistrars.mu.RLock()
	defer generatedGRPCServiceRegistrars.mu.RUnlock()

	if len(generatedGRPCServiceRegistrars.m) == 0 {
		return nil
	}

	names := make([]string, 0, len(generatedGRPCServiceRegistrars.m))
	for name := range generatedGRPCServiceRegistrars.m {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if err := generatedGRPCServiceRegistrars.m[name](server, cfg); err != nil {
			return trace.Wrap(err, "registering generated gRPC service %q", name)
		}
	}
	return nil
}
