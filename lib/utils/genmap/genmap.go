/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
package genmap

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

// Config configures a generative map. The only required parameter is the generator itself.
type Config[K comparable, V any] struct {
	// Context is an optional parent to use when setting up
	// the genmap's close context.
	Context context.Context

	// Generator is the closure used to generate values for a given key.
	Generator func(context.Context, K) (V, error)

	// RegenInterval is the interval on which entries are regenerated.
	RegenInterval time.Duration

	// Jitter is the jitter applied to the regen interval.
	Jitter retryutils.Jitter

	// MaxFailures is the maximum number of times generation can fail
	// in a row before we give up (note: giving up is temporary, the next
	// call to Get will start generation up again for that key).
	MaxFailures int
}

var (
	// ErrGenMapClosed is returned by blocking operations called on a GenMap that has been closed.
	ErrGenMapClosed = errors.New("generative map has been permanently closed")

	// ErrNoGenerator is the only possible error returned by New and indicates that the required
	// generator config value was not supplied.
	ErrNoGenerator = errors.New("generative map configured with nil generator (this is a bug)")
)

// GenMap is a mapping used to pre-generate and store values that we want to always have
// available *immediately*, such as client tls configs. It serves a purpose similar to FnCache, but
// with the notable distinction that it attempts to load/reload values *before* they
// are needed, so that callers never need to wait. Once a key is accessed for the first
// time it continues to be automatically regenerated until it is explicitly removed or the
// provided generator function returns MaxFailures errors in a row. Note that for most usecases
// the FnCache is probably a better choice. This helper is only really beneficial in cases where
// a small set of very frequently accessed values exist *and* said values are costly to generate *and*
// said values need to be regenerated very frequently, and possibly in response to events.
type GenMap[K comparable, V any] struct {
	cfg     Config[K, V]
	rw      sync.RWMutex
	entries map[K]*entry[V]
	ctx     context.Context
	cancel  context.CancelFunc
}

// CheckAndSetDefaults verifies config parameters and sets
// default values as needed.
func (c *Config[K, V]) CheckAndSetDefaults() error {
	if c.Context == nil {
		c.Context = context.Background()
	}

	if c.Generator == nil {
		return trace.Wrap(ErrNoGenerator)
	}

	if c.RegenInterval < 1 {
		c.RegenInterval = time.Minute
	}

	if c.Jitter == nil {
		c.Jitter = retryutils.SeventhJitter
	}

	if c.MaxFailures < 1 {
		c.MaxFailures = 3
	}

	return nil
}

// New sets up a new GenMap based on the supplied configuration.
func New[K comparable, V any](cfg Config[K, V]) (*GenMap[K, V], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(cfg.Context)

	return &GenMap[K, V]{
		cfg:     cfg,
		entries: make(map[K]*entry[V]),
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

type entry[V any] struct {
	ptr       atomic.Pointer[value[V]]
	init      chan struct{}
	wantRegen chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
}

type value[V any] struct {
	val        V
	err        error
	regenOnErr atomic.Bool
}

func (e *entry[V]) read() (val V, err error, ok bool) {
	v := e.ptr.Load()
	if v == nil {
		return
	}

	// the first read of an error value triggers regen
	if v.err != nil && !v.regenOnErr.Swap(true) {
		select {
		case e.wantRegen <- struct{}{}:
		default:
		}
	}

	return v.val, v.err, true
}

// Generate starts/restarts generation for the target key. This can be called to trigger generation for keys that
// are not yet being generated, or to trigger and early regen for existing keys.
func (m *GenMap[K, V]) Generate(key K) {
	ent, isNew := m.ensureEntry(key)
	if isNew {
		return
	}

	select {
	case ent.wantRegen <- struct{}{}:
	default:
	}
}

// RegenAll triggers regeneration of all currently tracked values.
func (m *GenMap[K, V]) RegenAll() {
	m.rw.RLock()
	defer m.rw.RUnlock()

	for _, ent := range m.entries {
		select {
		case ent.wantRegen <- struct{}{}:
		default:
		}
	}
}

// Terminate halts generation for the target key. Note that future calls to Get/Generate will restart generation. For this
// reason a robust 'cleanup' strategy should also include providing a generator that returns errors when called for keys
// that are no longer needed. This will ensure that concurrent get/generate calls can't result in dangling entries long-term,
// since the entry will eventually get cleaned up naturally when it hits MaxFailures.
func (m *GenMap[K, V]) Terminate(key K) {
	m.rw.RLock()
	_, ok := m.entries[key]
	m.rw.RUnlock()
	if !ok {
		return
	}

	m.rw.Lock()
	ent, ok := m.entries[key]
	if ok {
		delete(m.entries, key)
	}
	m.rw.Unlock()
	if !ok {
		return
	}
	ent.cancel()
	<-ent.done
}

// Get loads the value associated with the target key. If the key was not yet being generated by this
// map instance, generation will be started. This method is non-blocking, except in the case where
// the key being accessed is/was just added to the mapping.
func (m *GenMap[K, V]) Get(ctx context.Context, key K) (val V, err error) {
	ent, _ := m.ensureEntry(key)

	// optimistically load the value immediately (this will always succeed unless the genmap just
	// started tracking this value, so over time most calls should follow this path).
	if val, err, ok := ent.read(); ok {
		return val, err
	}

	// note that we aren't selecting on the per-entry context. each entry always generates at
	// lease one value before exiting, so we only care about exiting early if the map as a whole
	// is closed.
	select {
	case <-ent.init:
		val, err, _ = ent.read()
		return val, err
	case <-m.ctx.Done():
		return val, ErrGenMapClosed
	case <-ctx.Done():
		return val, ctx.Err()
	}
}

// ensureEntry gets the entry for the target key if it exists, falling back to setting up a new one.
func (m *GenMap[K, V]) ensureEntry(key K) (ent *entry[V], isNew bool) {
	m.rw.RLock()
	ent, ok := m.entries[key]
	m.rw.RUnlock()
	if ok {
		return ent, false
	}

	// create an entry-level context that is a child of the
	// main close context.
	ctx, cancel := context.WithCancel(m.ctx)

	newEnt := &entry[V]{
		init:      make(chan struct{}),
		wantRegen: make(chan struct{}, 1),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	m.rw.Lock()
	// check if another entry was set up concurrently
	ent, ok = m.entries[key]
	if !ok {
		m.entries[key] = newEnt
	}
	m.rw.Unlock()
	if ok {
		cancel()
		return ent, false
	}

	// start background opts for new entry
	go m.generate(key, newEnt)

	return newEnt, true
}

// generate is the background process that generates/regenerates the value associated with a given key.
func (m *GenMap[K, V]) generate(key K, entry *entry[V]) {
	defer func() {
		m.rw.RLock()
		current := m.entries[key]
		m.rw.RUnlock()
		if current != entry {
			close(entry.done)
			return
		}

		m.rw.Lock()
		if m.entries[key] == entry {
			delete(m.entries, key)
		}
		m.rw.Unlock()
		close(entry.done)
	}()

	t := time.NewTimer(m.cfg.Jitter(m.cfg.RegenInterval))
	defer t.Stop()
	var (
		// td tracks wether or not we've drained the timer (necessary for
		// safe resetting).
		td bool
		// init tracks wether or not we've already marked our associated
		// entry as initialized.
		init bool
		// errs counts the number of consecutive errors.
		errs int
	)
	for {
		// drain the reload channel since we're about to reload
		select {
		case <-entry.wantRegen:
		default:
		}

		// reset the timer before loading to help compensate
		// for slow generators.
		if !t.Stop() && !td {
			<-t.C
		}
		td = false
		t.Reset(m.cfg.Jitter(m.cfg.RegenInterval))

		// note that the generator is invoked with the map-level context, not the entry-level context.
		// the entry-level context is only used to terminate background generators *between* regens as
		// we want to avoid concurrent calls to Terminate generating spurious cancellation errors.
		val, err := m.cfg.Generator(m.ctx, key)

		// error values want to be reloaded, but reloading in a hot loop on errors is
		// potentially problematic. as a compromise, the first read of each error value
		// triggers an early regen, so retry logic effectively lives on the read half.
		if err != nil {
			errs++
		} else {
			errs = 0
		}

		// update the atomic pointer to store the newly generated value.
		entry.ptr.Store(&value[V]{
			val: val,
			err: err,
		})

		// if this was our first gen, mark the entry as initialized.
		if !init {
			close(entry.init)
			init = true
		}

		// if we hit too many consecutive errors, its preferable to just halt generation for the
		// time being. we will automatically start generation again if/when new calls to get come in.
		if errs > m.cfg.MaxFailures {
			return
		}

		select {
		case <-entry.wantRegen:
		case <-t.C:
			// timer doesn't need draining
			td = true
		case <-entry.ctx.Done():
			return
		}
	}
}

func (m *GenMap[K, V]) Close() error {
	m.cancel()
	return nil
}
