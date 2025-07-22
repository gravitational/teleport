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

package destination

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const MemoryType = "memory"

// NewMemory returns an initialized and ready to use memory destination.
func NewMemory() *Memory {
	mem := &Memory{}
	mem.CheckAndSetDefaults()
	return mem
}

// Memory is a memory certificate Destination
type Memory struct {
	store map[string][]byte `yaml:"-"`
	// mutex protects store in case other routines want to read its content
	mutex sync.RWMutex
}

func (dm *Memory) UnmarshalYAML(node *yaml.Node) error {
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

	type rawMemory Memory
	return trace.Wrap(node.Decode((*rawMemory)(dm)))
}

func (dm *Memory) CheckAndSetDefaults() error {
	// Initialize the store but only if it is nil. This allows the memory
	// destination to persist across multiple "Starts" of a bot.
	if dm.store == nil {
		dm.store = make(map[string][]byte)
	}

	return nil
}

func (dm *Memory) Init(_ context.Context, subdirs []string) error {
	// Nothing to do.
	return nil
}

func (dm *Memory) Verify(keys []string) error {
	// Nothing to do.
	return nil
}

func (dm *Memory) Write(ctx context.Context, name string, data []byte) error {
	_, span := tracer.Start(
		ctx,
		"Memory/Write",
		oteltrace.WithAttributes(attribute.String("name", name)),
	)
	defer span.End()

	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	dm.store[name] = data

	return nil
}

func (dm *Memory) Read(ctx context.Context, name string) ([]byte, error) {
	_, span := tracer.Start(
		ctx,
		"Memory/Read",
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

func (dm *Memory) String() string {
	return MemoryType
}

func (dm *Memory) TryLock() (func() error, error) {
	// As this is purely in-memory, no locking behavior is required for the
	// Destination.
	return func() error {
		return nil
	}, nil
}

func (dm *Memory) MarshalYAML() (any, error) {
	type raw Memory
	return encoding.WithTypeHeader((*raw)(dm), MemoryType)
}

func (dm *Memory) IsPersistent() bool {
	return false
}
