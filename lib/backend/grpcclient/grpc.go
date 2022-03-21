// Copyright 2022 Gravitational, Inc
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

package grpcclient

import (
	"context"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	log "github.com/sirupsen/logrus"
)

type GrpcBackend struct {
	clock clockwork.Clock
}

// Config represents JSON config for grpc backend
type Config struct {
	// TODO(michaelmcallister): implementation
}

// GetName returns the name of etcd backend as it appears in 'storage/type' section
// in Teleport YAML file. This function is a part of backend API
func GetName() string {
	return "grpc"
}

// keep this here to test interface conformance
var _ backend.Backend = &GrpcBackend{}

// New returns new instance of grpc-powered backend
func New(ctx context.Context, params backend.Params) (*GrpcBackend, error) {
	l := log.WithFields(log.Fields{trace.Component: GetName()})

	l.Infof("Initializing backend %v.", GetName())
	defer l.Debug("Backend initialization complete.")

	return &GrpcBackend{}, nil
}

func (cfg *Config) CheckAndSetDefaults() error {
	// TODO(michaelmcallister): implementation
	return nil
}

func (b *GrpcBackend) Clock() clockwork.Clock {
	return b.clock
}

func (b *GrpcBackend) Close() error {
	// TODO(michaelmcallister): implementation
	return nil
}

func (b *GrpcBackend) CloseWatchers() {
	// TODO(michaelmcallister): implementation
}

// NewWatcher returns a new event watcher
func (b *GrpcBackend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	// TODO(michaelmcallister): implementation
	return nil, nil
}

func (b *GrpcBackend) GetRange(ctx context.Context, startKey, endKey []byte, limit int) (*backend.GetResult, error) {
	// TODO(michaelmcallister): implementation
	return nil, trace.NotImplemented("GetRange not implemented")
}

func (b *GrpcBackend) Create(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	// TODO(michaelmcallister): implementation
	return nil, trace.NotImplemented("Create not implemented")
}

func (b *GrpcBackend) Update(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	// TODO(michaelmcallister): implementation
	return nil, trace.NotImplemented("Update not implemented")
}

func (b *GrpcBackend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	// TODO(michaelmcallister): implementation
	return nil, trace.NotImplemented("CompareAndSwap not implemented")
}

func (b *GrpcBackend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	// TODO(michaelmcallister): implementation
	return nil, trace.NotImplemented("Put not implemented")
}

func (b *GrpcBackend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	// TODO(michaelmcallister): implementation
	return trace.NotImplemented("KeepAlive not implemented")
}

func (b *GrpcBackend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	// TODO(michaelmcallister): implementation
	return nil, trace.NotImplemented("Get not implemented")
}

func (b *GrpcBackend) Delete(ctx context.Context, key []byte) error {
	// TODO(michaelmcallister): implementation
	return trace.NotImplemented("Delete not implemented")
}

func (b *GrpcBackend) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	// TODO(michaelmcallister): implementation
	return trace.NotImplemented("DeleteRange not implemented")
}
