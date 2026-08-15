//go:build race

// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
)

// activeRLocks is a hashset of [activeRLocksItem] (i.e. the key is
// activeRLocksItem and the value is always an untyped nil) used by [rwMutex].
var activeRLocks sync.Map

type activeRLocksItem struct {
	mu   *rwMutex
	goid uint64
}

var rLockChecking bool

// enableRLockCheck makes it so that [rwMutex] will keep track of the goroutine
// ID that RLocked and RUnlocked the mutex, panicking in case of reentrant
// locking. It should be called before tests start.
func enableRLockCheck() {
	rLockChecking = true
}

// finalRLockCheck checks that all instances of [rwMutex] are no longer locked.
// It should be called after tests are done.
func finalRLockCheck() {
	if !rLockChecking {
		return
	}

	activeRLocks.Range(func(key, value any) bool {
		i := key.(activeRLocksItem)
		panic(fmt.Sprintf("RWMutex at %p left RLocked by goroutine %d", i.mu, i.goid))
	})
}

// rwMutex is a [sync.RWMutex] with optional checking against reentrancy.
// Because of the checks, an acquired RLock must be RUnlocked in the same
// goroutine (unlike sync.RWMutex, which doesn't care about that).
type rwMutex struct {
	mu sync.RWMutex
}

// rLockCheckGoid returns the goroutine ID of the calling goroutine. It should
// only be called in test code, as the performance is terrible.
func rLockCheckGoid() uint64 {
	// "goroutine 18446744073709551616 [" is 32 characters
	buf := make([]byte, 32)
	const allGoroutinesFalse = false
	buf = buf[:runtime.Stack(buf, allGoroutinesFalse)]
	buf, found := bytes.CutPrefix(buf, []byte("goroutine "))
	if !found {
		panic("could not find goroutine id from stack dump")
	}
	buf, _, found = bytes.Cut(buf, []byte(" ["))
	if !found {
		panic("could not find goroutine id from stack dump")
	}
	goid, err := strconv.ParseUint(string(buf), 10, 64)
	if err != nil {
		panic(err)
	}
	return goid
}

func (mu *rwMutex) RLock() {
	if rLockChecking {
		goid := rLockCheckGoid()
		_, alreadyPresent := activeRLocks.LoadOrStore(activeRLocksItem{mu: mu, goid: goid}, nil)
		if alreadyPresent {
			panic(fmt.Sprintf("goroutine %d attempted to RLock the RWMutex at %p twice", goid, mu))
		}
	}
	mu.mu.RLock()
}
func (mu *rwMutex) RUnlock() {
	if rLockChecking {
		goid := rLockCheckGoid()
		_, deleted := activeRLocks.LoadAndDelete(activeRLocksItem{mu: mu, goid: goid})
		if !deleted {
			panic(fmt.Sprintf("goroutine %d attempted to RUnlock the RWMutex at %p without holding the RLock", goid, mu))
		}
	}
	mu.mu.RUnlock()
}

func (mu *rwMutex) Lock() {
	mu.mu.Lock()
}
func (mu *rwMutex) Unlock() {
	mu.mu.Unlock()
}
