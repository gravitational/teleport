// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package log

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport"
)

func TestPackageLogger(t *testing.T) {
	ctx := context.Background()

	logger := NewPackageLogger(teleport.ComponentKey, "test").With("animal", "llama")

	logger2 := NewPackageLogger(teleport.ComponentKey, "test2").WithGroup("a").With("1", "foo").WithGroup("b").With("2", "bar")

	w := &safeWriter{}
	slog.SetDefault(slog.New(NewSlogTextHandler(w, SlogTextHandlerConfig{Level: TraceLevel, ConfiguredFields: []string{LevelField, ComponentField}})))

	logger.DebugContext(ctx, "test 123")

	logger.With("fruit", "pear").InfoContext(ctx, "abcxyz")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		logger2.DebugContext(ctx, "test 123")
	}()

	go func() {
		defer wg.Done()
		logger2.WithGroup("c").With("fruit", "pear").InfoContext(ctx, "abcxyz")
	}()

	expected := []string{
		"DEBU [TEST]      test 123 animal:llama",
		"INFO [TEST]      abcxyz animal:llama fruit:pear",
		"DEBU [TEST2]     test 123 a.1:foo a.b.2:bar",
		"INFO [TEST2]     abcxyz a.1:foo a.b.2:bar a.b.c.fruit:pear",
	}

	wg.Wait()

	out := w.String()
	for _, line := range expected {
		assert.Contains(t, out, line)
	}
}

type safeWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.buf.Write(p)
}

func (s *safeWriter) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.buf.String()
}
