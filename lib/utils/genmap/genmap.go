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

	"github.com/gravitational/teleport/api/utils/retryutils"
)

// Config configures a generative map. The only required parameter is the generator itself.
type Config[K comparable, V any] struct {
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

	// ErrEntryTerminated is returned in the case that a specific target entry was terminated while
	// the caller is waiting on it to initialize.
	ErrEntryTerminated = errors.New("generative map entry was concurrently terminated")
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

// New sets up a new GenMap based on the supplied configuration.
func New[K comparable, V any](cfg Config[K, V]) *GenMap[K, V] {
	if cfg.Generator == nil {
		// no program that specifies a nil genetor is valid
		panic("genmap.New called with nil Generator value")
	}

	if cfg.RegenInterval < 1 {
		cfg.RegenInterval = time.Minute
	}

	if cfg.Jitter == nil {
		cfg.Jitter = retryutils.NewSeventhJitter()
	}

	if cfg.MaxFailures < 1 {
		cfg.MaxFailures = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &GenMap[K, V]{
		cfg:     cfg,
		entries: make(map[K]*entry[V]),
		ctx:     ctx,
		cancel:  cancel,
	}
}

type entry[V any] struct {
	ptr       atomic.Pointer[value[V]]
	init      chan struct{}
	wantRegen chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

type value[V any] struct {
	val        V
	err        error
	regenOnErr chan struct{}
}

func (v *value[V]) read() (V, error) {
	if v.err != nil {
		// reads of error values trigger early regen
		select {
		case v.regenOnErr <- struct{}{}:
		default:
		}
	}
	return v.val, v.err
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

// Terminate halts generation for the target key. Note that future calls to Get/Generate will restart generation. For this
// reason a robust 'cleanup' strategy should also include providing a generator that returns errors when called for keys
// that are no longer needed. This will ensure that concurrent get/generate calls can't result in dangling entries long-term,
// since the entry will eventually get cleaned up naturally when it hits MaxFailures.
func (m *GenMap[K, V]) Terminate(key K) {
	m.rw.RLock()
	ent, ok := m.entries[key]
	m.rw.RUnlock()
	if !ok {
		return
	}

	m.rw.Lock()
	ent, ok = m.entries[key]
	if ok {
		delete(m.entries, key)
	}
	m.rw.Unlock()
	if !ok {
		return
	}
	ent.cancel()
}

// Get loads the value associated with the target key. If the key was not yet being generated by this
// map instance, generation will be started. This method is non-blocking, except in the case where
// the key being accessed is/was just added to the mapping.
func (m *GenMap[K, V]) Get(ctx context.Context, key K) (val V, err error) {
	ent, _ := m.ensureEntry(key)

	// optimistically load the value pointer (this will always succeed unless the genmap just
	// started tracking this value, so over time most calls should follow this path).
	if vp := ent.ptr.Load(); vp != nil {
		return vp.read()
	}

	// note that we aren't selecting on the per-entry context. each entry always generates at
	// lease one value before exiting, so we only care about exiting early if the map as a whole
	// is closed.
	select {
	case <-ent.init:
		return ent.ptr.Load().read()
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
			return
		}

		m.rw.Lock()
		if m.entries[key] == entry {
			delete(m.entries, key)
		}
		m.rw.Unlock()
	}()

	t := time.NewTimer(m.cfg.Jitter(m.cfg.RegenInterval))
	defer t.Stop()

	var init bool
	var errs int
	for {
		// drain the reload channel since we're about to reload
		select {
		case <-entry.wantRegen:
		default:
		}

		// reset the timer before loading to help compensate
		// for slow generators.
		t.Stop()
		select {
		case <-t.C:
		default:
		}
		t.Reset(m.cfg.Jitter(m.cfg.RegenInterval))

		// note that the generator is invoked with the map-level context, not the entry-level context.
		// the entry-level context is only used to terminate background generators *between* regens as
		// we want to avoid concurrent calls to Terminate generating spurious cancellation errors.
		val, err := m.cfg.Generator(m.ctx, key)

		// error values want to be reloaded, but reloading in a hot loop on errors is
		// potentially problematic. as a compromise, we store a signal channel alongside
		// error values that gets triggered by readers. this allows us to attempt regen
		// relatively promptly for those values that are in high/frequent demand, while
		// also preventing dangling/low-demand values from generating excess load due
		// to constant regeneration.
		var regenOnErr chan struct{}
		if err != nil {
			errs++
			regenOnErr = make(chan struct{}, 1)
		} else {
			errs = 0
		}

		// update the atomic pointer to store the newly generated value.
		entry.ptr.Store(&value[V]{
			val:        val,
			err:        err,
			regenOnErr: regenOnErr,
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
		case <-regenOnErr:
		case <-entry.wantRegen:
		case <-t.C:
		case <-entry.ctx.Done():
			return
		}
	}
}

func (m *GenMap[K, V]) Close() error {
	m.cancel()
	return nil
}
