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

package endpoint_test

import (
	"context"
	"log/slog"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	smithyendpoints "github.com/aws/smithy-go/endpoints"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/aws/endpoint"
)

type fakeResolver[P any] struct {
	e smithyendpoints.Endpoint
}

func (f *fakeResolver[P]) ResolveEndpoint(ctx context.Context, params P) (smithyendpoints.Endpoint, error) {
	return f.e, nil
}

type fakeHandler struct {
	mu       sync.Mutex
	resolved []string
	slog.Handler
}

func (*fakeHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *fakeHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "uri" {
			return true
		}

		h.resolved = append(h.resolved, attr.Value.String())
		return false
	})

	return nil
}

func (h *fakeHandler) Resolved() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return slices.Clone(h.resolved)
}

func TestResolution(t *testing.T) {
	ctx := context.Background()

	handler := &fakeHandler{}

	expected := smithyendpoints.Endpoint{URI: url.URL{Scheme: "https", Host: "example.com"}}
	fake := &fakeResolver[string]{e: expected}
	r, err := endpoint.NewLoggingResolver(fake, slog.New(handler))
	require.NoError(t, err)

	// Resolve the same endpoint several times and validate that
	// it is only emitted once.
	var wg sync.WaitGroup
	barrier := make(chan struct{})
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			<-barrier
			resolved, err := r.ResolveEndpoint(ctx, "test")
			assert.NoError(t, err)
			assert.Equal(t, expected, resolved)

			switch res := handler.Resolved(); len(res) {
			case 0:
				assert.EventuallyWithT(t, func(t *assert.CollectT) {
					assert.Equal(t, []string{expected.URI.String()}, handler.Resolved())
				}, 5*time.Second, 100*time.Millisecond)
			case 1:
				assert.Equal(t, []string{expected.URI.String()}, res)
			}
		}()
	}

	close(barrier)
	wg.Wait()

	// Alter the resolved endpoint
	expected = smithyendpoints.Endpoint{URI: url.URL{Scheme: "test", Host: "example.com"}}
	fake.e = expected

	// Resolve again and validate that the new endpoint is emitted
	resolved, err := r.ResolveEndpoint(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, expected, resolved)
	require.Equal(t, []string{"https://example.com", "test://example.com"}, handler.Resolved())
}
