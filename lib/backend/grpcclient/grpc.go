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
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"time"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	bpb "github.com/gravitational/teleport/api/backend/proto"
)

// Backend uses gRPC to send events to a gRPC server to be persisted.
type Backend struct {
	clock      clockwork.Clock
	client     bpb.BackendServiceClient
	clientConn *grpc.ClientConn
	buf        *backend.CircularBuffer
	log        *log.Entry
}

// Config represents JSON config for gRPC backend
type Config struct {
	// Server is the server providing the gRPC service
	Server string `json:"server,omitempty"`
	// BufferSize is a default buffer size
	// used to pull events
	BufferSize int `json:"buffer_size,omitempty"`
	// ServerCA is the CA used to sign the server side of the mTLS connection
	// TODO(michaelmcallister): Support online cert rotation
	ServerCA   string `json:"server_ca,omitempty"`
	ClientCert string `json:"client_cert,omitempty"`
	ClientKey  string `json:"client_key,omitempty"`
}

// GetName returns the name of the backend as it appears in 'storage/type' section
// in Teleport YAML file. This function is a part of backend API
func GetName() string {
	return "grpc"
}

// keep this here to test interface conformance
var _ backend.Backend = &Backend{}

// New returns new instance of gRPC-powered backend.
//
// The supplied context is used to cancel or expire the pending connection to
// the gRPC server. Once New is returned, the cancellation and expiration of ctx
// will be noop.
func New(ctx context.Context, params backend.Params) (*Backend, error) {
	l := log.WithFields(log.Fields{trace.Component: GetName()})

	l.Infof("Initializing backend %v.", GetName())
	defer l.Debug("Backend initialization complete.")

	var cfg *Config
	if err := utils.ObjectToStruct(params, &cfg); err != nil {
		return nil, trace.BadParameter("gRPC: configuration is invalid: %v", err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// CA + client certs for mTLS
	caCert, err := ioutil.ReadFile(cfg.ServerCA)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(michaelmcallister): There are some tunables that we may need to expose
	// or set such as:
	// - InitialConnWindowSize
	// - InitialWindowSize
	// - ReadBufferSize
	// - WriteBuffersize
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS13,
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
	})))
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		PermitWithoutStream: true,
	}))
	opts = append(opts, grpc.WithBlock())

	conn, err := grpc.DialContext(ctx, cfg.Server, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	//TODO(michaelmcallister): Connect to the gRPC server and emit events to the
	//buffer.
	buf := backend.NewCircularBuffer(backend.BufferCapacity(cfg.BufferSize))
	buf.SetInit()

	backend := &Backend{
		log:        l,
		clock:      clockwork.NewRealClock(),
		clientConn: conn,
		client:     bpb.NewBackendServiceClient(conn),
		buf:        buf,
	}
	return backend, nil
}

// CheckAndSetDefaults is a helper that returns an error if the supplied
// configuration is not enough to connect to the gRPC server.
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Server == "" {
		return trace.BadParameter("gRPC: server is not specified")
	}
	if cfg.ClientCert == "" {
		return trace.BadParameter("gRPC: client_cert is not specified")
	}
	if cfg.ClientKey == "" {
		return trace.BadParameter("gRPC: client_key is not specified")
	}
	if cfg.ServerCA == "" {
		return trace.BadParameter("gRPC: server_ca is not specified")
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferCapacity
	}
	return nil
}

// Clock returns wall clock
func (b *Backend) Clock() clockwork.Clock {
	return b.clock
}

// Close closes the underlying gRPC connection and releases associated
// resources.
func (b *Backend) Close() error {
	b.buf.Close()
	return b.clientConn.Close()
}

// CloseWatchers closes all the watchers without closing the backend.
func (b *Backend) CloseWatchers() {
	b.buf.Clear()
}

// NewWatcher returns a new event watcher
func (b *Backend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	return b.buf.NewWatcher(ctx, watch)
}

// GetRange returns range of elements
func (b *Backend) GetRange(ctx context.Context, startKey, endKey []byte, limit int) (*backend.GetResult, error) {
	result, err := b.client.GetRange(ctx, &bpb.GetRangeRequest{
		StartKey: startKey,
		EndKey:   endKey,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return &backend.GetResult{Items: grpcItemsToBackendItems(result.Items)}, nil
}

// Create creates item if it does not exist
func (b *Backend) Create(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	result, err := b.client.Create(ctx, backendItemToGrpcItem(item))
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return grpcLeaseToBackendLease(result), nil
}

// Update updates value in the backend
func (b *Backend) Update(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	result, err := b.client.Update(ctx, backendItemToGrpcItem(item))
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return grpcLeaseToBackendLease(result), nil
}

// CompareAndSwap compares item with existing item and replaces is with
// replaceWith item
func (b *Backend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	result, err := b.client.CompareAndSwap(ctx, &bpb.CompareAndSwapRequest{
		Expected:    backendItemToGrpcItem(expected),
		ReplaceWith: backendItemToGrpcItem(replaceWith),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return grpcLeaseToBackendLease(result), nil
}

// Put puts value into backend (creates if it does not exists,
// updates it otherwise)
func (b *Backend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	result, err := b.client.Put(ctx, backendItemToGrpcItem(item))
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return grpcLeaseToBackendLease(result), nil
}

// KeepAlive keeps objects from expiring, updating the lease on the existing
// objects
func (b *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	_, err := b.client.KeepAlive(ctx, &bpb.KeepAliveRequest{
		Lease: &bpb.Lease{
			Id:  lease.ID,
			Key: lease.Key,
		},
		Expires: expires,
	})
	return trail.FromGRPC(err)
}

// Get returns a single item or not found error
func (b *Backend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	result, err := b.client.Get(ctx, &bpb.GetRequest{Key: key})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return grpcItemToBackendItem(result), nil
}

// Delete deletes item by key
func (b *Backend) Delete(ctx context.Context, key []byte) error {
	_, err := b.client.Delete(ctx, &bpb.DeleteRequest{Key: key})
	return trail.FromGRPC(err)
}

// DeleteRange deletes range of items with keys between startKey and endKey
func (b *Backend) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	_, err := b.client.DeleteRange(ctx, &bpb.DeleteRangeRequest{
		StartKey: startKey,
		EndKey:   endKey,
	})
	return trail.FromGRPC(err)
}
