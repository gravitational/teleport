/*
Copyright 2015-2019 Gravitational, Inc.

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
	"os"
	"sort"
	"strings"
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	cq "github.com/gravitational/teleport/lib/utils/concurrentqueue"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport"
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
	eventCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "etcd_events",
			Help: "Number of etcd events",
		},
	)
	eventBackpressure = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "etcd_event_backpressure",
			Help: "Number of etcd events that hit backpressure",
		},
	)

	prometheusCollectors = []prometheus.Collector{
		writeLatencies, txLatencies, batchReadLatencies,
		readLatencies, writeRequests, txRequests, batchReadRequests, readRequests,
	}
)

type EtcdBackend struct {
	nodes []string
	*log.Entry
	cfg       *Config
	client    *clientv3.Client
	cancelC   chan bool
	stopC     chan bool
	clock     clockwork.Clock
	buf       *backend.CircularBuffer
	ctx       context.Context
	cancel    context.CancelFunc
	watchDone chan struct{}
}

// Config represents JSON config for etcd backend
type Config struct {
	// Nodes is a list of nodes
	Nodes []string `json:"peers,omitempty"`
	// Key is an optional prefix for etcd
	Key string `json:"prefix,omitempty"`
	// TLSKeyFile is a private key, implies mTLS client authentication
	TLSKeyFile string `json:"tls_key_file,omitempty"`
	// TLSCertFile is a client certificate implies mTLS client authentication
	TLSCertFile string `json:"tls_cert_file,omitempty"`
	// TLSCAFile is a trusted certificate authority certificate
	TLSCAFile string `json:"tls_ca_file,omitempty"`
	// Insecure turns off TLS
	Insecure bool `json:"insecure,omitempty"`
	// BufferSize is a default buffer size
	// used to pull events
	BufferSize int `json:"buffer_size,omitempty"`
	// DialTimeout specifies dial timeout
	DialTimeout time.Duration `json:"dial_timeout,omitempty"`
	// Username is an optional username for HTTPS basic authentication
	Username string `json:"username,omitempty"`
	// Password is initialized from password file, and is not read from the config
	Password string `json:"-"`
	// PasswordFile is an optional password file for HTTPS basic authentication,
	// expects path to a file
	PasswordFile string `json:"password_file,omitempty"`
	// MaxClientMsgSizeBytes optionally specifies the size limit on client send message size.
	// See https://github.com/etcd-io/etcd/blob/221f0cc107cb3497eeb20fb241e1bcafca2e9115/clientv3/config.go#L49
	MaxClientMsgSizeBytes int `json:"etcd_max_client_msg_size_bytes,omitempty"`
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
	err := utils.RegisterPrometheusCollectors(prometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if params == nil {
		return nil, trace.BadParameter("missing etcd configuration")
	}

	// convert generic backend parameters structure to etcd config:
	var cfg *Config
	if err = apiutils.ObjectToStruct(params, &cfg); err != nil {
		return nil, trace.BadParameter("invalid etcd configuration: %v", err)
	}
	if err = cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	buf := backend.NewCircularBuffer(
		backend.BufferCapacity(cfg.BufferSize),
	)
	closeCtx, cancel := context.WithCancel(ctx)
	b := &EtcdBackend{
		Entry:     log.WithFields(log.Fields{trace.Component: GetName()}),
		cfg:       cfg,
		nodes:     cfg.Nodes,
		cancelC:   make(chan bool, 1),
		stopC:     make(chan bool, 1),
		clock:     clockwork.NewRealClock(),
		cancel:    cancel,
		ctx:       closeCtx,
		watchDone: make(chan struct{}),
		buf:       buf,
	}

	// Check that the etcd nodes are at least the minimum version supported
	if err = b.reconnect(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	timeout, cancel := context.WithTimeout(ctx, time.Second*3*time.Duration(len(cfg.Nodes)))
	defer cancel()
	for _, n := range cfg.Nodes {
		status, err := b.client.Status(timeout, n)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ver := semver.New(status.Version)
		min := semver.New(teleport.MinimumEtcdVersion)
		if ver.LessThan(*min) {
			return nil, trace.BadParameter("unsupported version of etcd %v for node %v, must be %v or greater",
				status.Version, n, teleport.MinimumEtcdVersion)
		}
	}

	// Reconnect the etcd client to work around a data race in their code.
	// Upstream fix: https://github.com/etcd-io/etcd/pull/12992
	if err = b.reconnect(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	go b.asyncWatch()

	// Wrap backend in a input sanitizer and return it.
	return b, nil
}

// Validate checks if all the parameters are present/valid
func (cfg *Config) Validate() error {
	if len(cfg.Key) == 0 {
		return trace.BadParameter(`etcd: missing "prefix" parameter`)
	}
	// Make sure the prefix starts with a '/'.
	if cfg.Key[0] != '/' {
		cfg.Key = "/" + cfg.Key
	}
	if len(cfg.Nodes) == 0 {
		return trace.BadParameter(`etcd: missing "peers" parameter`)
	}
	if !cfg.Insecure {
		if cfg.TLSCAFile == "" {
			return trace.BadParameter(`etcd: missing "tls_ca_file" parameter`)
		}
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferCapacity
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = apidefaults.DefaultDialTimeout
	}
	if cfg.PasswordFile != "" {
		out, err := os.ReadFile(cfg.PasswordFile)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		// trim newlines as passwords in files tend to have newlines
		cfg.Password = strings.TrimSpace(string(out))
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

// CloseWatchers closes all the watchers
// without closing the backend
func (b *EtcdBackend) CloseWatchers() {
	b.buf.Clear()
}

func (b *EtcdBackend) reconnect(ctx context.Context) error {
	if b.client != nil {
		if err := b.client.Close(); err != nil {
			b.Entry.WithError(err).Warning("Failed closing existing etcd client on reconnect.")
		}
	}

	tlsConfig := utils.TLSConfig(nil)

	if b.cfg.TLSCertFile != "" {
		clientCertPEM, err := os.ReadFile(b.cfg.TLSCertFile)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		clientKeyPEM, err := os.ReadFile(b.cfg.TLSKeyFile)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		tlsCert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
		if err != nil {
			return trace.BadParameter("failed to parse private key: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	if b.cfg.TLSCAFile != "" {
		caCertPEM, err := os.ReadFile(b.cfg.TLSCAFile)
		if err != nil {
			return trace.ConvertSystemError(err)
		}

		certPool := x509.NewCertPool()
		parsedCert, err := tlsca.ParseCertificatePEM(caCertPEM)
		if err != nil {
			return trace.Wrap(err, "failed to parse CA certificate %q", b.cfg.TLSCAFile)
		}
		certPool.AddCert(parsedCert)

		tlsConfig.RootCAs = certPool
		tlsConfig.ClientCAs = certPool
	}

	clt, err := clientv3.New(clientv3.Config{
		Endpoints:          b.nodes,
		TLS:                tlsConfig,
		DialTimeout:        b.cfg.DialTimeout,
		DialOptions:        []grpc.DialOption{grpc.WithBlock()},
		Username:           b.cfg.Username,
		Password:           b.cfg.Password,
		MaxCallSendMsgSize: b.cfg.MaxClientMsgSizeBytes,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	b.client = clt
	return nil
}

func (b *EtcdBackend) asyncWatch() {
	defer close(b.watchDone)
	var err error
WatchEvents:
	for b.ctx.Err() == nil {
		err = b.watchEvents(b.ctx)

		b.Debugf("Watch exited: %v", err)

		// pause briefly to prevent excessive watcher creation attempts
		select {
		case <-time.After(utils.HalfJitter(time.Millisecond * 1500)):
		case <-b.ctx.Done():
			break WatchEvents
		}
	}
	b.Debugf("Watch stopped: %v.", trace.NewAggregate(err, b.ctx.Err()))
}

// eventResult is used to ferry the result of event processing
type eventResult struct {
	original clientv3.Event
	event    backend.Event
	err      error
}

// watchEvents spawns an etcd watcher and forwards events to the event buffer. the internals of this
// function are complicated somewhat by the fact that we need to make a per-event API call to translate
// lease IDs into expiry times. if events are being created faster than their expiries can be resolved,
// this eventually results in runaway memory usage within the etcd client.  To combat this, we use a
// concurrentqueue.Queue to parallelize the event processing logic while preserving event order.  While
// effective, this strategy still suffers from a "head of line blocking"-esque issue since event order
// must be preserved.
func (b *EtcdBackend) watchEvents(ctx context.Context) error {

	// etcd watch client relies on context cancellation for cleanup,
	// so create a new subscope for this function.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// wrap fromEvent in a closure compatible with the concurrent queue
	workfn := func(v interface{}) interface{} {
		original := v.(clientv3.Event)
		var event backend.Event
		e, err := b.fromEvent(ctx, original)
		if e != nil {
			event = *e
		}
		return eventResult{
			original: original,
			event:    event,
			err:      err,
		}
	}

	// constants here are a bit arbitrary. the goal is to set up the queue s.t.
	// it could handle >100 events per second assuming an avg of .2 seconds of processing
	// time per event (as seen in tests of under-provisioned etcd instances).
	q := cq.New(
		workfn,
		cq.Workers(24),
		cq.Capacity(240),
		cq.InputBuf(120),
		cq.OutputBuf(48),
	)

	// emitDone signals that the background goroutine used for emitting the processed
	// events to the buffer has halted.
	emitDone := make(chan struct{})

	// watcher must be registered before we initialize the buffer
	eventsC := b.client.Watch(ctx, b.cfg.Key, clientv3.WithPrefix())

	// set buffer to initialized state.
	b.buf.SetInit()

	// ensure correct cleanup ordering (buffer must not be reset until event emission has halted).
	defer func() {
		q.Close()
		<-emitDone
		b.buf.Reset()
	}()

	// launch background process responsible for event emission.
	go func() {
		defer close(emitDone)
	EmitEvents:
		for {
			select {
			case p := <-q.Pop():
				r := p.(eventResult)
				if r.err != nil {
					b.WithError(r.err).Errorf("Failed to unmarshal event: %v.", r.original)
					continue EmitEvents
				}
				b.buf.Emit(r.event)
			case <-q.Done():
				return
			}
		}
	}()

	var lastBacklogWarning time.Time
	for {
		select {
		case e, ok := <-eventsC:
			if e.Canceled || !ok {
				return trace.ConnectionProblem(nil, "etcd watch channel closed")
			}

		PushToQueue:
			for i := range e.Events {
				eventCount.Inc()

				var event clientv3.Event = *e.Events[i]
				// attempt non-blocking push.  We allocate a large input buffer for the queue, so this
				// aught to succeed reliably.
				select {
				case q.Push() <- event:
					continue PushToQueue
				default:
				}

				eventBackpressure.Inc()

				// limit backlog warnings to once per minute to prevent log spam.
				if now := time.Now(); now.After(lastBacklogWarning.Add(time.Minute)) {
					b.Warnf("Etcd event processing backlog; may result in excess memory usage and stale cluster state.")
					lastBacklogWarning = now
				}

				// fallblack to blocking push
				select {
				case q.Push() <- event:
				case <-ctx.Done():
					return trace.ConnectionProblem(ctx.Err(), "context is closing")
				}
			}
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context is closing")
		}
	}
}

// NewWatcher returns a new event watcher
func (b *EtcdBackend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
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
	opts := []clientv3.OpOption{clientv3.WithSerializable(), clientv3.WithRange(b.prependPrefix(endKey))}
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(int64(limit)))
	}
	start := b.clock.Now()
	re, err := b.client.Get(ctx, b.prependPrefix(startKey), opts...)
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
			Key:     b.trimPrefix(kv.Key),
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
		If(clientv3.Compare(clientv3.CreateRevision(b.prependPrefix(item.Key)), "=", 0)).
		Then(clientv3.OpPut(b.prependPrefix(item.Key), base64.StdEncoding.EncodeToString(item.Value), opts...)).
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
		If(clientv3.Compare(clientv3.CreateRevision(b.prependPrefix(item.Key)), "!=", 0)).
		Then(clientv3.OpPut(b.prependPrefix(item.Key), base64.StdEncoding.EncodeToString(item.Value), opts...)).
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
	if !bytes.Equal(expected.Key, replaceWith.Key) {
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
		If(clientv3.Compare(clientv3.Value(b.prependPrefix(expected.Key)), "=", encodedPrev)).
		Then(clientv3.OpPut(b.prependPrefix(expected.Key), base64.StdEncoding.EncodeToString(replaceWith.Value), opts...)).
		Commit()
	txLatencies.Observe(time.Since(start).Seconds())
	txRequests.Inc()
	if err != nil {
		err = convertErr(err)
		if trace.IsNotFound(err) {
			return nil, trace.CompareFailed(err.Error())
		}
		return nil, trace.Wrap(err)
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
		b.prependPrefix(item.Key),
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
	re, err := b.client.Get(ctx, b.prependPrefix(lease.Key), clientv3.WithSerializable(), clientv3.WithKeysOnly())
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
	re, err := b.client.Get(ctx, b.prependPrefix(key), clientv3.WithSerializable())
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
	re, err := b.client.Delete(ctx, b.prependPrefix(key))
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
	_, err := b.client.Delete(ctx, b.prependPrefix(startKey), clientv3.WithRange(b.prependPrefix(endKey)))
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
			Key: b.trimPrefix(e.Kv.Key),
			ID:  e.Kv.ModRevision,
		},
	}
	if event.Type == types.OpDelete {
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
		return trace.ConnectionProblem(err, "operation has been canceled")
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
		case codes.ResourceExhausted:
			return trace.LimitExceeded(err.Error())
		default:
			return trace.BadParameter(err.Error())
		}
	}
	return trace.ConnectionProblem(err, err.Error())
}

func fromType(eventType mvccpb.Event_EventType) types.OpType {
	switch eventType {
	case mvccpb.PUT:
		return types.OpPut
	default:
		return types.OpDelete
	}
}

func (b *EtcdBackend) trimPrefix(in []byte) []byte {
	return bytes.TrimPrefix(in, []byte(b.cfg.Key))
}

func (b *EtcdBackend) prependPrefix(in []byte) string {
	return b.cfg.Key + string(in)
}
