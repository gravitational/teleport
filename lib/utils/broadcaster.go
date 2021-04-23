/*
Copyright 2015 Gravitational, Inc.

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

package utils

import (
	"sync"
)

// NewCloseBroadcaster returns new instance of close broadcaster
func NewCloseBroadcaster() *CloseBroadcaster {
	return &CloseBroadcaster{
		C: make(chan struct{}),
	}
}

// CloseBroadcaster is a helper struct
// that implements io.Closer and uses channel
// to broadcast its closed state once called
type CloseBroadcaster struct {
	sync.Once
	C chan struct{}
}

// Close closes channel (once) to start broadcasting it's closed state
func (b *CloseBroadcaster) Close() error {
	b.Do(func() {
		close(b.C)
	})
	return nil
}
