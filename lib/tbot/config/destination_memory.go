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

package config

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

const DestinationMemoryType = "memory"

// DestinationMemory is a memory certificate Destination
type DestinationMemory struct {
	store map[string][]byte `yaml:"-"`
	// mutex protects store in case other routines want to read its content
	mutex sync.RWMutex
}

func (dm *DestinationMemory) UnmarshalYAML(node *yaml.Node) error {
	// Accept either a bool or a raw (in this case empty) struct
	//   memory: {}
	// or:
	//   memory: true

	var boolVal bool
	if err := node.Decode(&boolVal); err == nil {
		if !boolVal {
			return trace.BadParameter("memory must not be false (leave unset to disable)")
		}
		return nil
	}

	type rawMemory DestinationMemory
	return trace.Wrap(node.Decode((*rawMemory)(dm)))
}

func (dm *DestinationMemory) CheckAndSetDefaults() error {
	dm.store = make(map[string][]byte)

	return nil
}

func (dm *DestinationMemory) Init(_ context.Context, subdirs []string) error {
	// Nothing to do.
	return nil
}

func (dm *DestinationMemory) Verify(keys []string) error {
	// Nothing to do.
	return nil
}

func (dm *DestinationMemory) Write(ctx context.Context, name string, data []byte) error {
	_, span := tracer.Start(
		ctx,
		"DestinationMemory/Write",
		oteltrace.WithAttributes(attribute.String("name", name)),
	)
	defer span.End()

	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	dm.store[name] = data

	return nil
}

func (dm *DestinationMemory) Read(ctx context.Context, name string) ([]byte, error) {
	_, span := tracer.Start(
		ctx,
		"DestinationMemory/Read",
		oteltrace.WithAttributes(attribute.String("name", name)),
	)
	defer span.End()

	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	b, ok := dm.store[name]
	if !ok {
		return nil, trace.NotFound("not found: %s", name)
	}

	return b, nil
}

func (dm *DestinationMemory) String() string {
	return DestinationMemoryType
}

func (dm *DestinationMemory) TryLock() (func() error, error) {
	// As this is purely in-memory, no locking behavior is required for the
	// Destination.
	return func() error {
		return nil
	}, nil
}

func (dm *DestinationMemory) MarshalYAML() (any, error) {
	type raw DestinationMemory
	return withTypeHeader((*raw)(dm), DestinationMemoryType)
}

func (dm *DestinationMemory) IsPersistent() bool {
	return false
}
