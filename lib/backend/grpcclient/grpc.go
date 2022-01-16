/*
Copyright 2021 Gravitational, Inc.

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

package grpcclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	grpcbackend "github.com/gravitational/teleport/api/backend/grpc"
	"github.com/gravitational/teleport/api/utils"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

type GrpcBackend struct {
	backend.NoMigrations
	clock clockwork.Clock

	clientConn *grpc.ClientConn
	grpc       grpcbackend.BackendServiceClient

	muWatchers sync.Mutex

	// buf uses the same watch / fan out architecture as other backends
	// like etcd/dynamo for proof of concept. But it might be better to
	// write through and cache in the gRPC service than the processes.
	buf *backend.CircularBuffer

	log *logrus.Entry
}

// Config represents JSON config for grpc backend
type Config struct {
	// Server is the server providing the grpc service
	Server string `json:"server,omitempty"`

	// BufferSize is a default buffer size
	// used to pull events
	BufferSize int `json:"buffer_size,omitempty"`

	// ServerCA is the CA used to sign the server side of the mTLS connection
	// TODO: Support online cert rotation
	ServerCA   string `json:"server_ca,omitempty"`
	ClientCert string `json:"client_cert,omitempty"`
	ClientKey  string `json:"client_key,omitempty"`
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

	var cfg *Config
	err := utils.ObjectToStruct(params, &cfg)
	if err != nil {
		return nil, trace.BadParameter("grpc configuration is invalid: %v", err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Server CA
	caCert, err := ioutil.ReadFile(cfg.ServerCA)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Client certs for mTLS
	// Read the key pair to create certificate
	cert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
	if err != nil {
		log.Fatal(err)
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS13,
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
	})))
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{ // TODO: configurable
		PermitWithoutStream: true,
	}))
	opts = append(opts, grpc.WithBlock())              //TODO: we may not want this to block, but for now block startup until connected to backend
	opts = append(opts, grpc.WithTimeout(time.Minute)) //TODO: configurable

	//TODO tuning WriteBuffersize / ReadBufferSize / InitialWindowSize / InitialConnWindowSize
	//TODO use balancer package

	conn, err := grpc.Dial(cfg.Server, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	backend := &GrpcBackend{
		log:        l,
		clock:      clockwork.NewRealClock(),
		clientConn: conn,
		grpc:       grpcbackend.NewBackendServiceClient(conn),
		buf: backend.NewCircularBuffer(
			backend.BufferCapacity(cfg.BufferSize),
		),
	}

	// TODO: clean close of background routine
	go backend.run(context.TODO())

	return backend, nil
}

func (cfg *Config) CheckAndSetDefaults() error {
	// TODO implementation
	return nil
}

func (b *GrpcBackend) Clock() clockwork.Clock {
	return b.clock
}

func (b *GrpcBackend) Close() error {
	//return trace.Wrap(b.buf.Close())
	//TODO need implementation
	return nil
}

func (b *GrpcBackend) CloseWatchers() {
	//b.buf.Clear()
	//TODO: needs implementation
}

// NewWatcher returns a new event watcher
func (b *GrpcBackend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	return b.buf.NewWatcher(ctx, watch)

	/*
		watchCtx, cancel := context.WithCancel(context.Background())

		res := watcher{
			events: make(chan backend.Event, watch.QueueSize),
			done:   make(chan struct{}, 1),
			cancel: cancel,
		}

		stream, err := b.grpc.NewWatcher(watchCtx, &grpcbackend.Watch{
			Name:            watch.Name,
			Prefixes:        watch.Prefixes,
			QueueSize:       int32(watch.QueueSize),
			MetricComponent: watch.MetricComponent,
		})
		if err != nil {
			return nil, trail.FromGRPC(err)
		}

		go func() {
			for {
				ev, err := stream.Recv()
				if err != nil {
					//TODO: this should be made to be resumable on failure
					//TODO: silent failure
					res.shutdown()
					return
				}

				res.events <- ConvertGrpcEventBackendEvent(ev)
			}
		}()

		return res, nil
	*/
}

// run connects to the gRPC server and sets up a watch for all events, and feeds the fanout buffer, similar
// to the etcd/dynamo backends
func (b *GrpcBackend) run(ctx context.Context) {
	// rate limit the connection attempts
	// TODO: should backoff
	// TODO: configurable
	ticker := time.NewTicker(time.Second)

	var lastSeenId int64
	for {
		<-ticker.C

		stream, err := b.grpc.NewWatcher(ctx, &grpcbackend.Watch{ResumeLastId: lastSeenId})
		if err != nil {
			b.log.Warn("Error setting up stream: ", err)
			continue
		}

		lastSeenId, err = b.processStream(stream)
		if err != nil {
			b.log.Warn("Error processing stream: ", err)
		}
	}
}

func (b *GrpcBackend) processStream(stream grpcbackend.BackendService_NewWatcherClient) (int64, error) {
	var lastSeenId int64

	b.buf.SetInit()
	defer b.buf.Reset()

	for {
		ev, err := stream.Recv()
		if err != nil {
			return lastSeenId, err
		}

		lastSeenId = ev.Item.Id

		b.buf.Emit(ConvertGrpcEventBackendEvent(ev))
	}
}

func (b *GrpcBackend) GetRange(ctx context.Context, startKey, endKey []byte, limit int) (*backend.GetResult, error) {
	result, err := b.grpc.GetRange(ctx, &grpcbackend.GetRangeRequest{
		StartKey: startKey,
		EndKey:   endKey,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return &backend.GetResult{
		Items: ConvertGrpcItemsBackendItems(result.Items),
	}, nil
}

func (b *GrpcBackend) Create(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	result, err := b.grpc.Create(ctx, ConvertBackendItemGrpcItem(item))
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return ConvertGrpcLeaseBackendLease(result), nil
}

func (b *GrpcBackend) Update(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	result, err := b.grpc.Update(ctx, ConvertBackendItemGrpcItem(item))
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return ConvertGrpcLeaseBackendLease(result), nil
}

func (b *GrpcBackend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	result, err := b.grpc.CompareAndSwap(ctx, &grpcbackend.CompareAndSwapRequest{
		Expected:    ConvertBackendItemGrpcItem(expected),
		ReplaceWith: ConvertBackendItemGrpcItem(replaceWith),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return ConvertGrpcLeaseBackendLease(result), nil
}

func (b *GrpcBackend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	result, err := b.grpc.Put(ctx, ConvertBackendItemGrpcItem(item))
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return ConvertGrpcLeaseBackendLease(result), nil
}

func (b *GrpcBackend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	_, err := b.grpc.KeepAlive(ctx, &grpcbackend.KeepAliveRequest{
		Lease:   ConvertBackendLeaseGrpcLease(lease),
		Expires: expires,
	})

	return trail.FromGRPC(err)

}

func (b *GrpcBackend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	result, err := b.grpc.Get(ctx, &grpcbackend.GetRequest{
		Key: key,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return ConvertGrpcItemBackendItem(result), nil
}

func (b *GrpcBackend) Delete(ctx context.Context, key []byte) error {
	_, err := b.grpc.Delete(ctx, &grpcbackend.DeleteRequest{
		Key: key,
	})

	return trail.FromGRPC(err)

}

func (b *GrpcBackend) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	_, err := b.grpc.DeleteRange(ctx, &grpcbackend.DeleteRangeRequest{
		StartKey: startKey,
		EndKey:   endKey,
	})

	return trail.FromGRPC(err)
}
