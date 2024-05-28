// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prompt

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
)

type FakeReplyFunc func(context.Context) (string, error)

type FakeReader struct {
	mu              sync.Mutex
	replies         []FakeReplyFunc
	waitingForReply chan struct{}
}

// NewFakeReader returns a fake that can be used in place of a ContextReader.
// Call Add functions in the desired order to configure responses. Each call
// represents a read reply, in order.
func NewFakeReader() *FakeReader {
	return &FakeReader{}
}

func (r *FakeReader) IsTerminal() bool {
	return true
}

func (r *FakeReader) AddReply(fn FakeReplyFunc) *FakeReader {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replies = append(r.replies, fn)
	if r.waitingForReply != nil {
		close(r.waitingForReply)
		r.waitingForReply = nil
	}
	return r
}

func (r *FakeReader) AddString(s string) *FakeReader {
	return r.AddReply(func(context.Context) (string, error) {
		return s, nil
	})
}

func (r *FakeReader) AddError(err error) *FakeReader {
	return r.AddReply(func(context.Context) (string, error) {
		return "", err
	})
}

func (r *FakeReader) ReadContext(ctx context.Context) ([]byte, error) {
	r.mu.Lock()
	if len(r.replies) == 0 {
		// wait for a reply
		wait := make(chan struct{})
		r.waitingForReply = wait
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-time.After(5 * time.Second):
			return nil, trace.BadParameter("no fake replies available after wait")
		case <-wait:
			r.mu.Lock()
		}
	}

	// Pop first reply.
	fn := r.replies[0]
	r.replies = r.replies[1:]
	r.mu.Unlock()

	val, err := fn(ctx)
	return []byte(val), err
}

func (r *FakeReader) ReadPassword(ctx context.Context) ([]byte, error) {
	return r.ReadContext(ctx)
}
