package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// SemaphoreKindConnection is the semaphore kind used by
// the Concurrent Session Control feature to limit concurrent
// connections (corresponds to the `max_connections`
// role option).
const SemaphoreKindConnection = "connection"

// Semaphores provides ability to control
// how many shared resources of some kind are acquired at the same time,
// used to implement concurrent sessions control in a distributed environment
type Semaphores interface {
	// AcquireSemaphore acquires lease with requested resources from semaphore
	AcquireSemaphore(ctx context.Context, params AcquireSemaphoreRequest) (*SemaphoreLease, error)
	// KeepAliveSemaphoreLease updates semaphore lease
	KeepAliveSemaphoreLease(ctx context.Context, lease SemaphoreLease) error
	// CancelSemaphoreLease cancels semaphore lease early
	CancelSemaphoreLease(ctx context.Context, lease SemaphoreLease) error
	// GetSemaphores returns a list of semaphores matching supplied filter.
	GetSemaphores(ctx context.Context, filter SemaphoreFilter) ([]Semaphore, error)
	// DeleteSemaphore deletes a semaphore matching supplied filter.
	DeleteSemaphore(ctx context.Context, filter SemaphoreFilter) error
}

// Match checks if the supplied semaphore matches this filter.
func (f *SemaphoreFilter) Match(sem Semaphore) bool {
	if f.SemaphoreKind != "" && f.SemaphoreKind != sem.GetSubKind() {
		return false
	}
	if f.SemaphoreName != "" && f.SemaphoreName != sem.GetName() {
		return false
	}
	return true
}

type SemaphoreLockConfig struct {
	// Service is the service against which all semaphore
	// operations are performed.
	Service Semaphores
	// Expiry is an optional lease expiry parameter.
	Expiry time.Duration
	// TickRate is the rate at which lease renewals are attempted
	// and defaults to 1/2 expiry.  Used to accelerate tests.
	TickRate time.Duration
	// Params holds the semaphore lease acquisition parameters.
	Params AcquireSemaphoreRequest
}

// CheckAndSetDefaults checks and sets default parameters
func (l *SemaphoreLockConfig) CheckAndSetDefaults() error {
	if l.Service == nil {
		return trace.BadParameter("missing semaphore service")
	}
	if l.Expiry == 0 {
		l.Expiry = defaults.SessionControlTimeout
	}
	if l.Expiry < time.Millisecond {
		return trace.BadParameter("sub-millisecond lease expiry is not supported: %v", l.Expiry)
	}
	if l.TickRate == 0 {
		l.TickRate = l.Expiry / 2
	}
	if l.TickRate >= l.Expiry {
		return trace.BadParameter("tick-rate must be less than expiry")
	}
	if l.Params.Expires.IsZero() {
		l.Params.Expires = time.Now().UTC().Add(l.Expiry)
	}
	if err := l.Params.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SemaphoreLock provides a convenient interface for managing
// semaphore lease keepalive operations.
type SemaphoreLock struct {
	cfg       SemaphoreLockConfig
	lease0    SemaphoreLease
	retry     utils.Retry
	ticker    *time.Ticker
	doneC     chan struct{}
	closeOnce sync.Once
	renewalC  chan struct{}
	cond      *sync.Cond
	err       error
	fin       bool
}

// finish registers the final result of the background
// goroutine.  must be called even if err is nil in
// order to wake any goroutines waiting on the error
// and mark the lock as finished.
func (l *SemaphoreLock) finish(err error) {
	l.cond.L.Lock()
	defer l.cond.L.Unlock()
	l.err = err
	l.fin = true
	l.cond.Broadcast()
}

// Done signals that lease keepalive operations
// have stopped.
func (l *SemaphoreLock) Done() <-chan struct{} {
	return l.doneC
}

// Wait blocks until the final result is available.  Note that
// this method may block longer than desired since cancellation of
// the parent context triggers the *start* of the release operation.
func (l *SemaphoreLock) Wait() error {
	l.cond.L.Lock()
	defer l.cond.L.Unlock()
	for !l.fin {
		l.cond.Wait()
	}
	return l.err
}

// Stop stops associated lease keepalive.
func (l *SemaphoreLock) Stop() {
	l.closeOnce.Do(func() {
		l.ticker.Stop()
		close(l.doneC)
	})
}

// Renewed notifies on next successful lease keepalive.
// Used in tests to block until next renewal.
func (l *SemaphoreLock) Renewed() <-chan struct{} {
	return l.renewalC
}

func (l *SemaphoreLock) KeepAlive(ctx context.Context) {
	var nodrop bool
	var err error
	lease := l.lease0
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		l.Stop()
		defer l.finish(err)
		if nodrop {
			// non-standard exit conditions; don't bother handling
			// cancellation/expiry.
			return
		}
		if lease.Expires.After(time.Now().UTC()) {
			// parent context is closed. create orphan context with generous
			// timeout for lease cancellation scope.  this will not block any
			// caller that is not explicitly waiting on the final error value.
			cancelContext, cancel := context.WithTimeout(context.Background(), l.cfg.Expiry/4)
			defer cancel()
			err = l.cfg.Service.CancelSemaphoreLease(cancelContext, lease)
			if err != nil {
				log.Warnf("Failed to cancel semaphore lease %s/%s: %v", lease.SemaphoreKind, lease.SemaphoreName, err)
			}
		} else {
			log.Errorf("Semaphore lease expired: %s/%s", lease.SemaphoreKind, lease.SemaphoreName)
		}
	}()
Outer:
	for {
		select {
		case tick := <-l.ticker.C:
			leaseContext, leaseCancel := context.WithDeadline(ctx, lease.Expires)
			nextLease := lease
			nextLease.Expires = tick.Add(l.cfg.Expiry)
			for {
				err = l.cfg.Service.KeepAliveSemaphoreLease(leaseContext, nextLease)
				if trace.IsNotFound(err) {
					leaseCancel()
					// semaphore and/or lease no longer exist; best to log the error
					// and exit immediately.
					log.Warnf("Halting keepalive on semaphore %s/%s early: %v", lease.SemaphoreKind, lease.SemaphoreName, err)
					nodrop = true
					return
				}
				if err == nil {
					leaseCancel()
					lease = nextLease
					l.retry.Reset()
					select {
					case l.renewalC <- struct{}{}:
					default:
					}
					continue Outer
				}
				log.Debugf("Failed to renew semaphore lease %s/%s: %v", lease.SemaphoreKind, lease.SemaphoreName, err)
				l.retry.Inc()
				select {
				case <-l.retry.After():
				case <-leaseContext.Done():
					leaseCancel() // demanded by linter
					return
				case <-l.Done():
					leaseCancel()
					return
				}
			}
		case <-ctx.Done():
			return
		case <-l.Done():
			return
		}
	}
}

// AcquireSemaphoreLock attempts to acquire and hold a semaphore lease.  If successfully acquired,
// background keepalive processes are started and an associated lock handle is returned.  Cancelling
// the supplied context releases the semaphore.
func AcquireSemaphoreLock(ctx context.Context, cfg SemaphoreLockConfig) (*SemaphoreLock, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	// set up retry with a ratio which will result in 3-4 retries before the lease expires
	retry, err := utils.NewLinear(utils.LinearConfig{
		Max:    cfg.Expiry / 4,
		Step:   cfg.Expiry / 16,
		Jitter: utils.NewJitter(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lease, err := cfg.Service.AcquireSemaphore(ctx, cfg.Params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lock := &SemaphoreLock{
		cfg:      cfg,
		lease0:   *lease,
		retry:    retry,
		ticker:   time.NewTicker(cfg.TickRate),
		doneC:    make(chan struct{}),
		renewalC: make(chan struct{}),
		cond:     sync.NewCond(&sync.Mutex{}),
	}
	return lock, nil
}

// Check verifies that all required parameters have been supplied.
func (s *AcquireSemaphoreRequest) Check() error {
	if s.SemaphoreKind == "" {
		return trace.BadParameter("missing parameter SemaphoreKind")
	}
	if s.SemaphoreName == "" {
		return trace.BadParameter("missing parameter SemaphoreName")
	}
	if s.MaxLeases == 0 {
		return trace.BadParameter("missing parameter MaxLeases")
	}
	if s.Expires.IsZero() {
		return trace.BadParameter("missing parameter Expires")
	}
	return nil
}

// ConfigureSemaphore configures an empty semaphore resource matching
// these acquire parameters.
func (s *AcquireSemaphoreRequest) ConfigureSemaphore() (Semaphore, error) {
	sem := SemaphoreV3{
		Kind:    KindSemaphore,
		SubKind: s.SemaphoreKind,
		Version: V3,
		Metadata: Metadata{
			Name:      s.SemaphoreName,
			Namespace: defaults.Namespace,
		},
	}
	sem.SetExpiry(s.Expires)
	if err := sem.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &sem, nil
}

// Semaphore represents distributed semaphore concept
type Semaphore interface {
	// Resource contains common resource values
	Resource
	// CheckAndSetDefaults checks and sets default parameters
	CheckAndSetDefaults() error
	// Contains checks if lease is member of this semaphore.
	Contains(lease SemaphoreLease) bool
	// Acquire attempts to acquire a lease with this semaphore.
	Acquire(leaseID string, params AcquireSemaphoreRequest) (*SemaphoreLease, error)
	// KeepAlive attempts to update the expiry of an existent lease.
	KeepAlive(lease SemaphoreLease) error
	// Cancel attempts to cancel an existent lease.
	Cancel(lease SemaphoreLease) error
	// LeaseRefs grants access to the underlying list
	// of lease references.
	LeaseRefs() []SemaphoreLeaseRef
	// RemoveExpiredLeases removes expired leases
	RemoveExpiredLeases(now time.Time)
}

// CheckAndSetDefaults checks and sets default values
func (l *SemaphoreLease) CheckAndSetDefaults() error {
	if l.SemaphoreKind == "" {
		return trace.BadParameter("missing parameter SemaphoreKind")
	}
	if l.SemaphoreName == "" {
		return trace.BadParameter("missing parameter SemaphoreName")
	}
	if l.LeaseID == "" {
		return trace.BadParameter("missing parameter LeaseID")
	}
	if l.Expires.IsZero() {
		return trace.BadParameter("missing lease expiry time")
	}
	return nil
}

// Contains checks if lease is member of this semaphore.
func (c *SemaphoreV3) Contains(lease SemaphoreLease) bool {
	if lease.SemaphoreKind != c.GetSubKind() || lease.SemaphoreName != c.GetName() {
		return false
	}
	for _, ref := range c.Spec.Leases {
		if ref.LeaseID == lease.LeaseID {
			return true
		}
	}
	return false
}

// Acquire attempts to acquire a lease with this semaphore.
func (c *SemaphoreV3) Acquire(leaseID string, params AcquireSemaphoreRequest) (*SemaphoreLease, error) {
	if params.SemaphoreKind != c.GetSubKind() || params.SemaphoreName != c.GetName() {
		return nil, trace.BadParameter("cannot acquire, params do not match")
	}

	if c.leaseCount() >= params.MaxLeases {
		return nil, trace.LimitExceeded("cannot acquire semaphore %s/%s (%s)",
			c.GetSubKind(),
			c.GetName(),
			teleport.MaxLeases,
		)
	}

	for _, ref := range c.Spec.Leases {
		if ref.LeaseID == leaseID {
			return nil, trace.AlreadyExists("semaphore lease already exists: %q", leaseID)
		}
	}

	if params.Expires.After(c.Expiry()) {
		c.SetExpiry(params.Expires)
	}

	c.Spec.Leases = append(c.Spec.Leases, SemaphoreLeaseRef{
		LeaseID: leaseID,
		Expires: params.Expires,
		Holder:  params.Holder,
	})

	return &SemaphoreLease{
		SemaphoreKind: params.SemaphoreKind,
		SemaphoreName: params.SemaphoreName,
		LeaseID:       leaseID,
		Expires:       params.Expires,
	}, nil
}

// KeepAlive attempts to update the expiry of an existent lease.
func (c *SemaphoreV3) KeepAlive(lease SemaphoreLease) error {
	if lease.SemaphoreKind != c.GetSubKind() || lease.SemaphoreName != c.GetName() {
		return trace.BadParameter("cannot keepalive, lease does not match")
	}
	for i := range c.Spec.Leases {
		if c.Spec.Leases[i].LeaseID == lease.LeaseID {
			c.Spec.Leases[i].Expires = lease.Expires
			if lease.Expires.After(c.Expiry()) {
				c.SetExpiry(lease.Expires)
			}
			return nil
		}
	}
	return trace.NotFound("cannot keepalive, lease not found: %q", lease.LeaseID)
}

// Cancel attempts to cancel an existent lease.
func (c *SemaphoreV3) Cancel(lease SemaphoreLease) error {
	if lease.SemaphoreKind != c.GetSubKind() || lease.SemaphoreName != c.GetName() {
		return trace.BadParameter("cannot cancel, lease does not match")
	}
	for i, ref := range c.Spec.Leases {
		if ref.LeaseID == lease.LeaseID {
			c.Spec.Leases = append(c.Spec.Leases[:i], c.Spec.Leases[i+1:]...)
			return nil
		}
	}
	return trace.NotFound("cannot cancel, lease not found: %q", lease.LeaseID)
}

// RemoveExpiredLeases removes expired leases
func (c *SemaphoreV3) RemoveExpiredLeases(now time.Time) {
	// See https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
	filtered := c.Spec.Leases[:0]
	for _, lease := range c.Spec.Leases {
		if lease.Expires.After(now) {
			filtered = append(filtered, lease)
		}
	}
	c.Spec.Leases = filtered
}

// leaseCount returns the number of active leases
func (c *SemaphoreV3) leaseCount() int64 {
	return int64(len(c.Spec.Leases))
}

// LeaseRefs grants access to the underlying list
// of lease references
func (c *SemaphoreV3) LeaseRefs() []SemaphoreLeaseRef {
	return c.Spec.Leases
}

// GetVersion returns resource version
func (c *SemaphoreV3) GetVersion() string {
	return c.Version
}

// GetSubKind returns resource subkind
func (c *SemaphoreV3) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *SemaphoreV3) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetKind returns resource kind
func (c *SemaphoreV3) GetKind() string {
	return c.Kind
}

// GetResourceID returns resource ID
func (c *SemaphoreV3) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *SemaphoreV3) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetName returns the name of the cluster.
func (c *SemaphoreV3) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the cluster.
func (c *SemaphoreV3) SetName(e string) {
	c.Metadata.Name = e
}

// Expires returns object expiry setting
func (c *SemaphoreV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (c *SemaphoreV3) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets Expires header using realtime clock
func (c *SemaphoreV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns object metadata
func (c *SemaphoreV3) GetMetadata() Metadata {
	return c.Metadata
}

// String represents a human readable version of the semaphore.
func (c *SemaphoreV3) String() string {
	return fmt.Sprintf("Semaphore(kind=%v, name=%v, leases=%v)",
		c.SubKind, c.Metadata.Name, c.leaseCount())
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (c *SemaphoreV3) CheckAndSetDefaults() error {
	// make sure we have defaults for all metadata fields
	err := c.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	// While theoretically there are scenarios with non-expiring semaphores
	// however the flow don't need them right now, and they add a lot of edge
	// cases, so the code does not support them.
	if c.Expiry().IsZero() {
		return trace.BadParameter("set semaphore expiry time")
	}
	if c.SubKind == "" {
		return trace.BadParameter("supply semaphore SubKind parameter")
	}
	return nil
}

// SemaphoreSpecSchemaTemplate is a template for Semaphore schema.
const SemaphoreSpecSchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "leases": {
	  "type": "array",
	  "items": {
        "type": "object",
		"properties": {
		  "lease_id": { "type": "string" },
		  "expires": { "type": "string" },
		  "holder": { "type": "string" }
        }
	  }
    }
  }
}`

// GetSemaphoreSchema returns the validation schema for this object
func GetSemaphoreSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, SemaphoreSpecSchemaTemplate, DefaultDefinitions)
}

// SemaphoreMarshaler implements marshal/unmarshal of Semaphore implementations
// mostly adds support for extended versions.
type SemaphoreMarshaler interface {
	Marshal(c Semaphore, opts ...MarshalOption) ([]byte, error)
	Unmarshal(bytes []byte, opts ...MarshalOption) (Semaphore, error)
}

var semaphoreMarshaler SemaphoreMarshaler = &TeleportSemaphoreMarshaler{}

// SetSemaphoreMarshaler sets the marshaler.
func SetSemaphoreMarshaler(m SemaphoreMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	semaphoreMarshaler = m
}

// GetSemaphoreMarshaler gets the marshaler.
func GetSemaphoreMarshaler() SemaphoreMarshaler {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return semaphoreMarshaler
}

// TeleportSemaphoreMarshaler is used to marshal and unmarshal Semaphore.
type TeleportSemaphoreMarshaler struct{}

// Unmarshal unmarshals Semaphore from JSON.
func (t *TeleportSemaphoreMarshaler) Unmarshal(bytes []byte, opts ...MarshalOption) (Semaphore, error) {
	var semaphore SemaphoreV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(bytes, &semaphore); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	} else {
		err = utils.UnmarshalWithSchema(GetSemaphoreSchema(), &semaphore, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	err = semaphore.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		semaphore.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		semaphore.SetExpiry(cfg.Expires)
	}
	return &semaphore, nil
}

// Marshal marshals Semaphore to JSON.
func (t *TeleportSemaphoreMarshaler) Marshal(c Semaphore, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := c.(type) {
	case *SemaphoreV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *resource
			copy.SetResourceID(0)
			resource = &copy
		}
		return utils.FastMarshal(resource)
	default:
		return nil, trace.BadParameter("unrecognized resource version %T", c)
	}
}
