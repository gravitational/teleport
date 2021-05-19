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
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
	"go.etcd.io/etcd/mvcc/mvccpb"
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

type EtcdBackend struct {
	nodes []string
	*log.Entry
	cfg              *Config
	client           *clientv3.Client
	cancelC          chan bool
	stopC            chan bool
	clock            clockwork.Clock
	buf              *backend.CircularBuffer
	ctx              context.Context
	cancel           context.CancelFunc
	watchStarted     context.Context
	signalWatchStart context.CancelFunc
	watchDone        chan struct{}
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

// legacyDefaultPrefix was used instead of Config.Key prior to 4.3. It's used
// below to allow a safe migration to the correct usage of Config.Key during
// 4.3 and will be removed in 4.4
//
// DELETE IN 4.4: legacy prefix support for migration of
// https://github.com/gravitational/teleport/issues/2883
const legacyDefaultPrefix = "/teleport/"

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
		watchDone:        make(chan struct{}),
		buf:              buf,
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
	// Wait for watch goroutine to start to avoid data races around the config
	// struct in tests.
	select {
	case <-watchStarted.Done():
	case <-ctx.Done():
		b.Close()
		return nil, trace.Wrap(ctx.Err())
	}

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
		cfg.BufferSize = backend.DefaultBufferSize
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = defaults.DefaultDialTimeout
	}
	if cfg.PasswordFile != "" {
		out, err := ioutil.ReadFile(cfg.PasswordFile)
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
	<-b.watchDone
	b.buf.Close()
	return b.client.Close()
}

// CloseWatchers closes all the watchers
// without closing the backend
func (b *EtcdBackend) CloseWatchers() {
	b.buf.Reset()
}

func (b *EtcdBackend) reconnect(ctx context.Context) error {
	if b.client != nil {
		if err := b.client.Close(); err != nil {
			b.Entry.WithError(err).Warning("Failed closing existing etcd client on reconnect.")
		}
	}

	tlsConfig := utils.TLSConfig(nil)

	if b.cfg.TLSCertFile != "" {
		clientCertPEM, err := ioutil.ReadFile(b.cfg.TLSCertFile)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		clientKeyPEM, err := ioutil.ReadFile(b.cfg.TLSKeyFile)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		tlsCert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
		if err != nil {
			return trace.BadParameter("failed to parse private key: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}

	var caCertPEM []byte
	if b.cfg.TLSCAFile != "" {
		var err error
		caCertPEM, err = ioutil.ReadFile(b.cfg.TLSCAFile)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}

	certPool := x509.NewCertPool()
	parsedCert, err := tlsca.ParseCertificatePEM(caCertPEM)
	if err != nil {
		return trace.Wrap(err, "failed to parse CA certificate")
	}
	certPool.AddCert(parsedCert)

	tlsConfig.RootCAs = certPool
	tlsConfig.ClientCAs = certPool

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
	err := b.watchEvents()
	b.Debugf("Watch exited: %v.", err)
}

func (b *EtcdBackend) watchEvents() error {
	defer close(b.watchDone)

start:
	eventsC := b.client.Watch(b.ctx, b.cfg.Key, clientv3.WithPrefix())
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

func (b *EtcdBackend) Migrate(ctx context.Context) error {
	// DELETE IN 4.4: legacy prefix support for migration of
	// https://github.com/gravitational/teleport/issues/2883
	if err := b.syncLegacyPrefix(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DELETE IN 4.4: legacy prefix support for migration of
// https://github.com/gravitational/teleport/issues/2883
//
// syncLegacyPrefix is a temporary migration step for 4.3 release. It will
// attempt to replicate the data from '/teleport' prefix (legacyDefaultPrefix)
// into the correct prefix specified in teleport.yaml (b.cfg.Key).
//
// The goal is to prevent the need for admin intervention when upgrading
// Teleport clusters and to avoid losing any data during the upgrade to a fixed
// version of Teleport. See issue linked above for more context.
//
// The replication will happen when:
// - there's data under legacy prefix
// - the configured prefix is different from the legacy prefix
// - the configured prefix is empty OR older than the legacy prefix
func (b *EtcdBackend) syncLegacyPrefix(ctx context.Context) error {
	// Using the same prefix, nothing to migrate.
	if b.cfg.Key == legacyDefaultPrefix {
		return nil
	}
	legacyData, err := b.client.Get(ctx, legacyDefaultPrefix, clientv3.WithPrefix())
	if err != nil {
		return trace.Wrap(err)
	}
	// No data in the legacy prefix, assume this is a new Teleport cluster and
	// skip sync early.
	if legacyData.Count == 0 {
		return nil
	}
	prefixData, err := b.client.Get(ctx, b.cfg.Key, clientv3.WithPrefix())
	if err != nil {
		return trace.Wrap(err)
	}
	if !shouldSync(legacyData.Kvs, prefixData.Kvs) {
		return nil
	}

	b.Infof("Migrating Teleport etcd data from legacy prefix %q to configured prefix %q, see https://github.com/gravitational/teleport/issues/2883 for context", legacyDefaultPrefix, b.cfg.Key)
	defer b.Infof("Teleport etcd data migration complete")

	// Now we know that legacy prefix has some data newer than the configured
	// prefix. Migrate it over to configured prefix.
	//
	// First, let's backup the data under configured prefix, in case the
	// migration kicked in by mistake.
	backupPrefix := b.backupPrefix(b.cfg.Key)
	b.Infof("Backup everything under %q to %q", b.cfg.Key, backupPrefix)
	for _, kv := range prefixData.Kvs {
		// Replace the prefix.
		key := backupPrefix + strings.TrimPrefix(string(kv.Key), b.cfg.Key)
		b.Debugf("Copying %q -> %q", kv.Key, key)
		if _, err := b.client.Put(ctx, key, string(kv.Value)); err != nil {
			return trace.WrapWithMessage(err, "failed backing up %q to %q: %v; the problem could be with your etcd credentials or etcd cluster itself (e.g. running out of disk space); this backup is a safety precaution for migrating the data from etcd prefix %q (old default) to %q (from your teleport.yaml config), see https://github.com/gravitational/teleport/issues/2883 for context", kv.Key, key, err, legacyDefaultPrefix, b.cfg.Key)
		}
	}

	// Now delete existing prefix data.
	b.Infof("Deleting everything under %q", b.cfg.Key)
	deletePrefix := b.cfg.Key
	// Make sure the prefix ends with a '/', so that we don't delete the backup
	// created above or any other unrelated data.
	if !strings.HasSuffix(deletePrefix, "/") {
		deletePrefix += "/"
	}
	if _, err := b.client.Delete(ctx, deletePrefix, clientv3.WithPrefix()); err != nil {
		return trace.Wrap(err)
	}

	b.Infof("Copying everything under %q to %q", legacyDefaultPrefix, b.cfg.Key)
	var errs []error
	// Finally, copy over all the data from the legacy prefix to the new one.
	for _, kv := range legacyData.Kvs {
		// Replace the prefix.
		key := b.cfg.Key + "/" + strings.TrimPrefix(string(kv.Key), legacyDefaultPrefix)
		b.Debugf("Copying %q -> %q", kv.Key, key)
		if _, err := b.client.Put(ctx, key, string(kv.Value)); err != nil {
			errs = append(errs, trace.WrapWithMessage(err, "failed copying %q to %q: %v", kv.Key, key, err))
		}
	}
	return trace.NewAggregate(errs...)
}

func (b *EtcdBackend) backupPrefix(p string) string {
	return fmt.Sprintf("%s-backup-%s/", strings.TrimSuffix(p, "/"), b.clock.Now().UTC().Format(time.RFC3339))
}

func shouldSync(legacyData, prefixData []*mvccpb.KeyValue) bool {
	latestRev := func(kvs []*mvccpb.KeyValue) int64 {
		var rev int64
		for _, kv := range kvs {
			if kv.CreateRevision > rev {
				rev = kv.CreateRevision
			}
			if kv.ModRevision > rev {
				rev = kv.ModRevision
			}
		}
		return rev
	}
	if len(legacyData) == 0 {
		return false
	}
	if len(prefixData) == 0 {
		return true
	}
	// Data under the new prefix was updated more recently than data under the
	// legacy prefix. Assume we already did a sync before and legacy prefix
	// hasn't been touched since.
	return latestRev(legacyData) > latestRev(prefixData)
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
		case codes.ResourceExhausted:
			return trace.LimitExceeded(err.Error())
		default:
			return trace.BadParameter(err.Error())
		}
	}
	return trace.ConnectionProblem(err, err.Error())
}

func fromType(eventType mvccpb.Event_EventType) backend.OpType {
	switch eventType {
	case mvccpb.PUT:
		return backend.OpPut
	default:
		return backend.OpDelete
	}
}

func (b *EtcdBackend) trimPrefix(in []byte) []byte {
	return bytes.TrimPrefix(in, []byte(b.cfg.Key))
}

func (b *EtcdBackend) prependPrefix(in []byte) string {
	return b.cfg.Key + string(in)
}
