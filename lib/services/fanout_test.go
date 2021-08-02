/*
Copyright 2020 Gravitational, Inc.

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

package services

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/assert"
)

// TestFanoutWatcherClose tests fanout watcher close
// removes it from the buffer
func TestFanoutWatcherClose(t *testing.T) {
	eventsCh := make(chan FanoutEvent, 1)
	f := NewFanout(eventsCh)
	w, err := f.NewWatcher(context.TODO(),
		types.Watch{Name: "test", Kinds: []types.WatchKind{{Name: "test"}}})
	assert.NoError(t, err)
	assert.Equal(t, f.Len(), 1)

	err = w.Close()
	select {
	case <-eventsCh:
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for event")
	}
	assert.NoError(t, err)
	assert.Equal(t, f.Len(), 0)
}
