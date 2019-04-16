/*
Copyright 2015-2018 Gravitational, Inc.

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

// Package etcdbk implements Etcd powered backend
package etcdbk

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"io/ioutil"
	"sort"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	writeRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "etcd_backend_write_requests",
			Help: "Number of wrtie requests to the database",
		},
	)
	readRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "etcd_backend_read_requests",
			Help: "Number of read requests to the database",
		},
	)
	batchReadRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "etcd_backend_batch_read_requests",
			Help: "Number of read requests to the database",
		},
	)
	txRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "etcd_backend_tx_requests",
			Help: "Number of transaction requests to the database",
		},
	)
	writeLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "etcd_backend_write_seconds",
			Help: "Latency for etcd write operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
	txLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "etcd_backend_tx_seconds",
			Help: "Latency for etcd transaction operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
	batchReadLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "etcd_backend_batch_read_seconds",
			Help: "Latency for etcd read operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
	readLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "etcd_backend_read_seconds",
			Help: "Latency for etcd read operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(writeLatencies)
	prometheus.MustRegister(txLatencies)
	prometheus.MustRegister(batchReadLatencies)
	prometheus.MustRegister(readLatencies)
	prometheus.MustRegister(writeRequests)
	prometheus.MustRegister(txRequests)
	prometheus.MustRegister(batchReadRequests)
	prometheus.MustRegister(readRequests)
}

const (
	// keyPrefix is a prefix that is added to every etcd key
	// for backwards compatibility
	keyPrefix = "/teleport"
)

type EtcdBackend struct {
	nodes []string
	*log.Entry
	cfg              *Config
	etcdKey          string
	client           *clientv3.Client
	cancelC          chan bool
	stopC            chan bool
	clock            clockwork.Clock
	buf              *backend.CircularBuffer
	ctx              context.Context
	cancel           context.CancelFunc
	watchStarted     context.Context
	signalWatchStart context.CancelFunc
}

// Config represents JSON config for etcd backend
type Config struct {
	Nodes       []string `json:"peers,omitempty"`
	Key         string   `json:"prefix,omitempty"`
	TLSKeyFile  string   `json:"tls_key_file,omitempty"`
	TLSCertFile string   `json:"tls_cert_file,omitempty"`
	TLSCAFile   string   `json:"tls_ca_file,omitempty"`
	Insecure    bool     `json:"insecure,omitempty"`
	// BufferSize is a default buffer size
	// used to pull events
	BufferSize int `json:"buffer_size,omitempty"`
	// DialTimeout specifies dial timeout
	DialTimeout time.Duration `json:"dial_timeout,omitempty"`
}

// GetName returns the name of etcd backend as it appears in 'storage/type' section
// in Teleport YAML file. This function is a part of backend API
func GetName() string {
	return "etcd"
}

// keep this here to test interface conformance
var _ backend.Backend = &EtcdBackend{}

// New returns new instance of Etcd-powered backend
func New(ctx context.Context, params backend.Params) (*EtcdBackend, error) {
	var err error
	if params == nil {
		return nil, trace.BadParameter("missing etcd configuration")
	}

	// convert generic backend parameters structure to etcd config:
	var cfg *Config
	if err = utils.ObjectToStruct(params, &cfg); err != nil {
		return nil, trace.BadParameter("invalid etcd configuration: %v", err)
	}
	if err = cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	buf, err := backend.NewCircularBuffer(ctx, cfg.BufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	closeCtx, cancel := context.WithCancel(ctx)
	watchStarted, signalWatchStart := context.WithCancel(ctx)
	b := &EtcdBackend{
		Entry:            log.WithFields(log.Fields{trace.Component: GetName()}),
		cfg:              cfg,
		nodes:            cfg.Nodes,
		cancelC:          make(chan bool, 1),
		stopC:            make(chan bool, 1),
		clock:            clockwork.NewRealClock(),
		cancel:           cancel,
		ctx:              closeCtx,
		watchStarted:     watchStarted,
		signalWatchStart: signalWatchStart,
		buf:              buf,
	}
	if err = b.reconnect(); err != nil {
		return nil, trace.Wrap(err)
	}
	// Wrap backend in a input sanitizer and return it.
	return b, nil
}

// Validate checks if all the parameters are present/valid
func (cfg *Config) Validate() error {
	if len(cfg.Key) == 0 {
		return trace.BadParameter(`etcd: missing "prefix" setting`)
	}
	if len(cfg.Nodes) == 0 {
		return trace.BadParameter(`etcd: missing "peers" setting`)
	}
	if cfg.Insecure == false {
		if cfg.TLSKeyFile == "" {
			return trace.BadParameter(`etcd: missing "tls_key_file" setting`)
		}
		if cfg.TLSCertFile == "" {
			return trace.BadParameter(`etcd: missing "tls_cert_file" setting`)
		}
		if cfg.TLSCAFile == "" {
			return trace.BadParameter(`etcd: missing "tls_ca_file" setting`)
		}
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferSize
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = defaults.DefaultDialTimeout
	}
	return nil
}

func (b *EtcdBackend) Clock() clockwork.Clock {
	return b.clock
}

func (b *EtcdBackend) Close() error {
	b.cancel()
	b.buf.Close()
	return b.client.Close()
}

func (b *EtcdBackend) reconnect() error {
	clientCertPEM, err := ioutil.ReadFile(b.cfg.TLSCertFile)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	clientKeyPEM, err := ioutil.ReadFile(b.cfg.TLSKeyFile)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	caCertPEM, err := ioutil.ReadFile(b.cfg.TLSCAFile)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	tlsConfig := utils.TLSConfig(nil)
	tlsCert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	if err != nil {
		return trace.BadParameter("failed to parse private key: %v", err)
	}
	certPool := x509.NewCertPool()
	parsedCert, err := tlsca.ParseCertificatePEM(caCertPEM)
	if err != nil {
		return trace.Wrap(err, "failed to parse CA certificate")
	}
	certPool.AddCert(parsedCert)

	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.RootCAs = certPool
	tlsConfig.ClientCAs = certPool

	clt, err := clientv3.New(clientv3.Config{
		Endpoints:   b.nodes,
		TLS:         tlsConfig,
		DialTimeout: b.cfg.DialTimeout,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	b.client = clt
	go b.asyncWatch()
	return nil
}

func (b *EtcdBackend) asyncWatch() {
	err := b.watchEvents()
	b.Debugf("Watch exited: %v.", err)
}

func (b *EtcdBackend) watchEvents() error {
start:
	eventsC := b.client.Watch(b.ctx, keyPrefix, clientv3.WithPrefix())
	b.signalWatchStart()
	for {
		select {
		case e, ok := <-eventsC:
			if e.Canceled || !ok {
				b.Debugf("Watch channel has closed.")
				goto start
			}
			out := make([]backend.Event, 0, len(e.Events))
			for i := range e.Events {
				event, err := b.fromEvent(b.ctx, *e.Events[i])
				if err != nil {
					b.Errorf("Failed to unmarshal event: %v %v.", err, *e.Events[i])
				} else {
					out = append(out, *event)
				}
			}
			b.buf.PushBatch(out)
		case <-b.ctx.Done():
			return trace.ConnectionProblem(b.ctx.Err(), "context is closing")
		}
	}
}

// NewWatcher returns a new event watcher
func (b *EtcdBackend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	select {
	case <-b.watchStarted.Done():
	case <-ctx.Done():
		return nil, trace.ConnectionProblem(ctx.Err(), "context is closing")
	}
	return b.buf.NewWatcher(ctx, watch)
}

// GetRange returns query range
func (b *EtcdBackend) GetRange(ctx context.Context, startKey, endKey []byte, limit int) (*backend.GetResult, error) {
	if len(startKey) == 0 {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	opts := []clientv3.OpOption{clientv3.WithSerializable(), clientv3.WithRange(prependPrefix(endKey))}
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(int64(limit)))
	}
	start := b.clock.Now()
	re, err := b.client.Get(ctx, prependPrefix(startKey), opts...)
	batchReadLatencies.Observe(time.Since(start).Seconds())
	batchReadRequests.Inc()
	if err := convertErr(err); err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]backend.Item, 0, len(re.Kvs))
	for _, kv := range re.Kvs {
		value, err := unmarshal(kv.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items = append(items, backend.Item{
			Key:     trimPrefix(kv.Key),
			Value:   value,
			ID:      kv.ModRevision,
			LeaseID: kv.Lease,
		})
	}
	sort.Sort(backend.Items(items))
	return &backend.GetResult{Items: items}, nil
}

// Create creates item if it does not exist
func (b *EtcdBackend) Create(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	var opts []clientv3.OpOption
	var lease backend.Lease
	if !item.Expires.IsZero() {
		if err := b.setupLease(ctx, item, &lease, &opts); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	start := b.clock.Now()
	re, err := b.client.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(prependPrefix(item.Key)), "=", 0)).
		Then(clientv3.OpPut(prependPrefix(item.Key), base64.StdEncoding.EncodeToString(item.Value), opts...)).
		Commit()
	txLatencies.Observe(time.Since(start).Seconds())
	txRequests.Inc()
	if err != nil {
		return nil, trace.Wrap(convertErr(err))
	}
	if !re.Succeeded {
		return nil, trace.AlreadyExists("%q already exists", string(item.Key))
	}
	return &lease, nil
}

// Update updates value in the backend
func (b *EtcdBackend) Update(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	var opts []clientv3.OpOption
	var lease backend.Lease
	if !item.Expires.IsZero() {
		if err := b.setupLease(ctx, item, &lease, &opts); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	start := b.clock.Now()
	re, err := b.client.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(prependPrefix(item.Key)), "!=", 0)).
		Then(clientv3.OpPut(prependPrefix(item.Key), base64.StdEncoding.EncodeToString(item.Value), opts...)).
		Commit()
	txLatencies.Observe(time.Since(start).Seconds())
	txRequests.Inc()
	if err != nil {
		return nil, trace.Wrap(convertErr(err))
	}
	if !re.Succeeded {
		return nil, trace.NotFound("%q is not found", string(item.Key))
	}
	return &lease, nil
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (b *EtcdBackend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	if len(expected.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if len(replaceWith.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if bytes.Compare(expected.Key, replaceWith.Key) != 0 {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}
	var opts []clientv3.OpOption
	var lease backend.Lease
	if !replaceWith.Expires.IsZero() {
		if err := b.setupLease(ctx, replaceWith, &lease, &opts); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	encodedPrev := base64.StdEncoding.EncodeToString(expected.Value)
	start := b.clock.Now()
	re, err := b.client.Txn(ctx).
		If(clientv3.Compare(clientv3.Value(prependPrefix(expected.Key)), "=", encodedPrev)).
		Then(clientv3.OpPut(prependPrefix(expected.Key), base64.StdEncoding.EncodeToString(replaceWith.Value), opts...)).
		Commit()
	txLatencies.Observe(time.Since(start).Seconds())
	txRequests.Inc()
	if err != nil {
		err = convertErr(err)
		if trace.IsNotFound(err) {
			return nil, trace.CompareFailed(err.Error())
		}
	}
	if !re.Succeeded {
		return nil, trace.CompareFailed("key %q did not match expected value", string(expected.Key))
	}
	return &lease, nil
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (b *EtcdBackend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	var opts []clientv3.OpOption
	var lease backend.Lease
	if !item.Expires.IsZero() {
		if err := b.setupLease(ctx, item, &lease, &opts); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	start := b.clock.Now()
	_, err := b.client.Put(
		ctx,
		prependPrefix(item.Key),
		base64.StdEncoding.EncodeToString(item.Value),
		opts...)
	writeLatencies.Observe(time.Since(start).Seconds())
	writeRequests.Inc()
	if err != nil {
		return nil, convertErr(err)
	}
	return &lease, nil
}

// KeepAlive updates TTL on the lease ID
func (b *EtcdBackend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	if lease.ID == 0 {
		return trace.BadParameter("lease is not specified")
	}
	re, err := b.client.Get(ctx, prependPrefix(lease.Key), clientv3.WithSerializable(), clientv3.WithKeysOnly())
	if err != nil {
		return convertErr(err)
	}
	if len(re.Kvs) == 0 {
		return trace.NotFound("item %q is not found", string(lease.Key))
	}
	// instead of keep-alive on the old lease, setup a new lease
	// because we would like the event to be generated
	// which does not happen in case of lease keep-alive
	var opts []clientv3.OpOption
	var newLease backend.Lease
	if err := b.setupLease(ctx, backend.Item{Expires: expires}, &newLease, &opts); err != nil {
		return trace.Wrap(err)
	}
	opts = append(opts, clientv3.WithIgnoreValue())
	kv := re.Kvs[0]
	_, err = b.client.Put(ctx, string(kv.Key), "", opts...)
	return convertErr(err)
}

// Get returns a single item or not found error
func (b *EtcdBackend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	re, err := b.client.Get(ctx, prependPrefix(key), clientv3.WithSerializable())
	if err != nil {
		return nil, convertErr(err)
	}
	if len(re.Kvs) == 0 {
		return nil, trace.NotFound("item %q is not found", string(key))
	}
	kv := re.Kvs[0]
	bytes, err := unmarshal(kv.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &backend.Item{Key: key, Value: bytes, ID: kv.ModRevision, LeaseID: kv.Lease}, nil
}

// Delete deletes item by key
func (b *EtcdBackend) Delete(ctx context.Context, key []byte) error {
	start := b.clock.Now()
	re, err := b.client.Delete(ctx, prependPrefix(key))
	writeLatencies.Observe(time.Since(start).Seconds())
	writeRequests.Inc()
	if err != nil {
		return trace.Wrap(convertErr(err))
	}
	if re.Deleted == 0 {
		return trace.NotFound("%q is not found", key)
	}
	return nil
}

// DeleteRange deletes range of items with keys between startKey and endKey
func (b *EtcdBackend) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	if len(startKey) == 0 {
		return trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return trace.BadParameter("missing parameter endKey")
	}
	start := b.clock.Now()
	_, err := b.client.Delete(ctx, prependPrefix(startKey), clientv3.WithRange(prependPrefix(endKey)))
	writeLatencies.Observe(time.Since(start).Seconds())
	writeRequests.Inc()
	if err != nil {
		return trace.Wrap(convertErr(err))
	}
	return nil
}

func (b *EtcdBackend) setupLease(ctx context.Context, item backend.Item, lease *backend.Lease, opts *[]clientv3.OpOption) error {
	ttl := b.ttl(item.Expires)
	elease, err := b.client.Grant(ctx, seconds(ttl))
	if err != nil {
		return convertErr(err)
	}
	*opts = []clientv3.OpOption{clientv3.WithLease(elease.ID)}
	lease.ID = int64(elease.ID)
	lease.Key = item.Key
	return nil
}

func (b *EtcdBackend) ttl(expires time.Time) time.Duration {
	return backend.TTL(b.clock, expires)
}

func (b *EtcdBackend) fromEvent(ctx context.Context, e clientv3.Event) (*backend.Event, error) {
	event := &backend.Event{
		Type: fromType(e.Type),
		Item: backend.Item{
			Key: trimPrefix(e.Kv.Key),
			ID:  e.Kv.ModRevision,
		},
	}
	if event.Type == backend.OpDelete {
		return event, nil
	}
	// get the new expiration date if it was updated
	if e.Kv.Lease != 0 {
		re, err := b.client.TimeToLive(ctx, clientv3.LeaseID(e.Kv.Lease))
		if err != nil {
			return nil, convertErr(err)
		}
		event.Item.Expires = b.clock.Now().UTC().Add(time.Second * time.Duration(re.TTL))
	}
	value, err := unmarshal(e.Kv.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	event.Item.Value = value
	return event, nil
}

// seconds converts duration to seconds, rounds up to 1 second
func seconds(ttl time.Duration) int64 {
	i := int64(ttl / time.Second)
	if i <= 0 {
		i = 1
	}
	return i
}

func unmarshal(value []byte) ([]byte, error) {
	if len(value) == 0 {
		return nil, trace.BadParameter("missing value")
	}
	dbuf := make([]byte, base64.StdEncoding.DecodedLen(len(value)))
	n, err := base64.StdEncoding.Decode(dbuf, value)
	return dbuf[:n], err
}

func convertErr(err error) error {
	if err == nil {
		return nil
	}
	if err == context.Canceled {
		return trace.ConnectionProblem(err, "operation has been cancelled")
	} else if err == context.DeadlineExceeded {
		return trace.ConnectionProblem(err, "operation has timed out")
	} else if err == rpctypes.ErrEmptyKey {
		return trace.BadParameter(err.Error())
	} else if ev, ok := status.FromError(err); ok {
		switch ev.Code() {
		// server-side context might have timed-out first (due to clock skew)
		// while original client-side context is not timed-out yet
		case codes.DeadlineExceeded:
			return trace.ConnectionProblem(err, "operation has timed out")
		case codes.NotFound:
			return trace.NotFound(err.Error())
		case codes.AlreadyExists:
			return trace.AlreadyExists(err.Error())
		case codes.FailedPrecondition:
			return trace.CompareFailed(err.Error())
		default:
			return trace.BadParameter(err.Error())
		}
	}
	// bad cluster endpoints, which are not etcd servers
	return trace.ConnectionProblem(err, "bad cluster endpoints")
}

func fromType(eventType mvccpb.Event_EventType) backend.OpType {
	switch eventType {
	case mvccpb.PUT:
		return backend.OpPut
	default:
		return backend.OpDelete
	}
}

func trimPrefix(in []byte) []byte {
	return bytes.TrimPrefix(in, []byte(keyPrefix))
}

func prependPrefix(in []byte) string {
	return keyPrefix + string(in)
}
