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
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"io/ioutil"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"
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

type bk struct {
	nodes []string

	cfg     *Config
	etcdKey string
	client  *clientv3.Client
	cancelC chan bool
	stopC   chan bool
	clock   clockwork.Clock
}

// Config represents JSON config for etcd backend
type Config struct {
	Nodes       []string `json:"peers,omitempty"`
	Key         string   `json:"prefix,omitempty"`
	TLSKeyFile  string   `json:"tls_key_file,omitempty"`
	TLSCertFile string   `json:"tls_cert_file,omitempty"`
	TLSCAFile   string   `json:"tls_ca_file,omitempty"`
	Insecure    bool     `json:"insecure,omitempty"`
}

// GetName returns the name of etcd backend as it appears in 'storage/type' section
// in Teleport YAML file. This function is a part of backend API
func GetName() string {
	return "etcd"
}

// New returns new instance of Etcd-powered backend
func New(params backend.Params) (backend.Backend, error) {
	var err error
	if params == nil {
		return nil, trace.BadParameter("missing etcd configuration")
	}

	// convert generic backend parameters structure to etcd config:
	var cfg *Config
	if err = utils.ObjectToStruct(params, &cfg); err != nil {
		return nil, trace.BadParameter("invalid etcd configuration", err)
	}
	if err = cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	b := &bk{
		cfg:     cfg,
		nodes:   cfg.Nodes,
		etcdKey: cfg.Key,
		cancelC: make(chan bool, 1),
		stopC:   make(chan bool, 1),
		clock:   clockwork.NewRealClock(),
	}
	if err = b.reconnect(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Wrap backend in a input sanitizer and return it.
	return backend.NewSanitizer(b), nil
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
	return nil
}

func (b *bk) Clock() clockwork.Clock {
	return b.clock
}

func (b *bk) Close() error {
	return b.client.Close()
}

func (b *bk) key(keys ...string) string {
	return strings.Join(append([]string{b.etcdKey}, keys...), "/")
}

func (b *bk) reconnect() error {
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
		DialTimeout: defaults.DefaultDialTimeout,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	b.client = clt
	return nil
}

// GetItems fetches keys and values and returns them to the caller.
func (b *bk) GetItems(path []string) ([]backend.Item, error) {
	items, err := b.getItems(b.key(path...), false, clientv3.WithSerializable(), clientv3.WithPrefix())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return items, nil
}

// GetKeys fetches keys (and values) but only returns keys to the caller.
func (b *bk) GetKeys(path []string) ([]string, error) {
	items, err := b.getItems(b.key(path...), true, clientv3.WithSerializable(), clientv3.WithKeysOnly(), clientv3.WithPrefix())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert from []backend.Item to []string and return keys.
	keys := make([]string, len(items))
	for i, e := range items {
		keys[i] = e.Key
	}

	return keys, nil
}

func (b *bk) CreateVal(path []string, key string, val []byte, ttl time.Duration) error {
	var opts []clientv3.OpOption
	if ttl > 0 {
		if ttl/time.Second <= 0 {
			return trace.BadParameter("TTL should be in seconds, got %v instead", ttl)
		}
		lease, err := b.client.Grant(context.Background(), int64(ttl/time.Second))
		if err != nil {
			return convertErr(err)
		}
		opts = []clientv3.OpOption{clientv3.WithLease(lease.ID)}
	}
	keyPath := b.key(append(path, key)...)
	start := time.Now()
	re, err := b.client.Txn(context.Background()).
		If(clientv3.Compare(clientv3.CreateRevision(keyPath), "=", 0)).
		Then(clientv3.OpPut(keyPath, base64.StdEncoding.EncodeToString(val), opts...)).
		Commit()
	txLatencies.Observe(time.Since(start).Seconds())
	txRequests.Inc()
	if err != nil {
		return trace.Wrap(convertErr(err))
	}
	if !re.Succeeded {
		return trace.AlreadyExists("%q already exists", key)
	}
	return nil
}

// CompareAndSwapVal compares and swap values in atomic operation,
// succeeds if prevVal matches the value stored in the databases,
// requires prevVal as a non-empty value. Returns trace.CompareFailed
// in case if value did not match
func (b *bk) CompareAndSwapVal(path []string, key string, val []byte, prevVal []byte, ttl time.Duration) error {
	if len(prevVal) == 0 {
		return trace.BadParameter("missing prevVal parameter, to atomically create item, use CreateVal method")
	}
	var opts []clientv3.OpOption
	if ttl > 0 {
		if ttl/time.Second <= 0 {
			return trace.BadParameter("TTL should be in seconds, got %v instead", ttl)
		}
		lease, err := b.client.Grant(context.Background(), int64(ttl/time.Second))
		if err != nil {
			return convertErr(err)
		}
		opts = []clientv3.OpOption{clientv3.WithLease(lease.ID)}
	}
	keyPath := b.key(append(path, key)...)
	encodedPrev := base64.StdEncoding.EncodeToString(prevVal)
	start := time.Now()
	re, err := b.client.Txn(context.Background()).
		If(clientv3.Compare(clientv3.Value(keyPath), "=", encodedPrev)).
		Then(clientv3.OpPut(keyPath, base64.StdEncoding.EncodeToString(val), opts...)).
		Commit()
	txLatencies.Observe(time.Since(start).Seconds())
	txRequests.Inc()
	if err != nil {
		err = convertErr(err)
		if trace.IsNotFound(err) {
			return trace.CompareFailed(err.Error())
		}
	}
	if !re.Succeeded {
		return trace.CompareFailed("key %q did not match expected value", key)
	}
	return nil
}

// maxOptimisticAttempts is the number of attempts optimistic locking
const maxOptimisticAttempts = 5

func (bk *bk) UpsertItems(bucket []string, items []backend.Item) error {
	return trace.BadParameter("not implemented")
}

func (b *bk) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	var opts []clientv3.OpOption
	if ttl > 0 {
		if ttl/time.Second <= 0 {
			return trace.BadParameter("TTL should be in seconds, got %v instead", ttl)
		}
		lease, err := b.client.Grant(context.Background(), int64(ttl/time.Second))
		if err != nil {
			return convertErr(err)
		}
		opts = []clientv3.OpOption{clientv3.WithLease(lease.ID)}
	}
	start := time.Now()
	_, err := b.client.Put(
		context.Background(),
		b.key(append(path, key)...),
		base64.StdEncoding.EncodeToString(val),
		opts...)
	writeLatencies.Observe(time.Since(start).Seconds())
	writeRequests.Inc()
	return convertErr(err)
}

func (b *bk) GetVal(path []string, key string) ([]byte, error) {
	re, err := b.client.Get(context.Background(), b.key(append(path, key)...), clientv3.WithSerializable())
	if err == nil && len(re.Kvs) != 0 {
		bytes, err := unmarshal(re.Kvs[0].Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bytes, nil
	} else if err != nil {
		err = convertErr(err)
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}

	// now, check that it's a directory (this is a temp workaround)
	// before we refactor the backend interface
	start := time.Now()
	re, err = b.client.Get(context.Background(), b.key(append(path, key)...), clientv3.WithPrefix(), clientv3.WithSerializable(), clientv3.WithCountOnly())
	readLatencies.Observe(time.Since(start).Seconds())
	readRequests.Inc()
	if err := convertErr(err); err != nil {
		return nil, trace.Wrap(err)
	}
	if re.Count != 0 {
		return nil, trace.BadParameter("%q: trying to get value of bucket", key)
	}
	return nil, trace.NotFound("%q is not found", key)
}

func (b *bk) DeleteKey(path []string, key string) error {
	start := time.Now()
	re, err := b.client.Delete(context.Background(), b.key(append(path, key)...))
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

func (b *bk) DeleteBucket(path []string, key string) error {
	start := time.Now()
	re, err := b.client.Delete(context.Background(), b.key(append(path, key)...), clientv3.WithPrefix())
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

const delayBetweenLockAttempts = 100 * time.Millisecond

func (b *bk) AcquireLock(token string, ttl time.Duration) error {
	for {
		start := time.Now()
		err := b.CreateVal([]string{"locks"}, token, []byte("lock"), ttl)
		writeLatencies.Observe(time.Since(start).Seconds())
		writeRequests.Inc()
		err = convertErr(err)
		if err == nil {
			return nil
		}
		if err != nil {
			if !trace.IsCompareFailed(err) && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
			time.Sleep(delayBetweenLockAttempts)
		}
	}
}

func (b *bk) ReleaseLock(token string) error {
	start := time.Now()
	re, err := b.client.Delete(context.Background(), b.key("locks", token))
	writeLatencies.Observe(time.Since(start).Seconds())
	writeRequests.Inc()
	if err != nil {
		return trace.Wrap(convertErr(err))
	}
	if re.Deleted == 0 {
		return trace.NotFound("%q is not found", token)
	}
	return nil
}

func unmarshal(value []byte) ([]byte, error) {
	if len(value) == 0 {
		return nil, trace.BadParameter("missing value")
	}
	dbuf := make([]byte, base64.StdEncoding.DecodedLen(len(value)))
	n, err := base64.StdEncoding.Decode(dbuf, value)
	return dbuf[:n], err
}

// getItems fetches keys and values and returns them to the caller.
func (b *bk) getItems(path string, keysOnly bool, opts ...clientv3.OpOption) ([]backend.Item, error) {
	var vals []backend.Item

	start := time.Now()
	re, err := b.client.Get(context.Background(), path, opts...)
	batchReadLatencies.Observe(time.Since(start).Seconds())
	batchReadRequests.Inc()
	if err := convertErr(err); err != nil {
		if trace.IsNotFound(err) {
			return vals, nil
		}
		return nil, trace.Wrap(err)
	}

	items := make([]backend.Item, 0, len(re.Kvs))
	for _, kv := range re.Kvs {
		if strings.Compare(path, string(kv.Key[:len(path)])) == 0 && len(path) < len(kv.Key) {
			item := backend.Item{
				Key: suffix(string(kv.Key[len(path)+1:])),
			}
			if !keysOnly {
				value, err := unmarshal(kv.Value)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				item.Value = value
			}
			items = append(items, item)
		}
	}
	sort.Sort(backend.Items(items))
	return items, nil
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

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[0]
}
