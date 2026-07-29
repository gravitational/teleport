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

package limiter

import (
	"context"
	"math"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	llmerrors "github.com/gravitational/teleport/lib/srv/app/llm/errors"
)

// InMemory provides tokens limiter in memory.
//
// This should act as initial implementation and behavior reference for future
// implementations like distributed solutions.
type InMemory struct {
	mu      sync.Mutex
	storage map[appKey]Usage

	inputTokensLimit  uint
	outputTokensLimit uint
}

// NewInMemory creates a new instance of the [InMemory] limiter.
func NewInMemory(input, output uint) *InMemory {
	return &InMemory{
		inputTokensLimit:  input,
		outputTokensLimit: output,
		storage:           make(map[appKey]Usage),
	}
}

// Reserve reserves LLM usage.
func (i *InMemory) Reserve(ctx context.Context, req ReserveRequest) (ReserveInfo, SettleFunc, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return ReserveInfo{}, EmptySettleFunc, trace.Wrap(err)
	}

	key := newAppKey(req.App)
	i.mu.Lock()
	defer i.mu.Unlock()
	curr := i.storage[key]

	// Consumption goes past the limits when a settlement reports more usage
	// than reserved, leaving nothing to reserve.
	if curr.InputTokens > i.inputTokensLimit || curr.OutputTokens > i.outputTokensLimit {
		return ReserveInfo{}, EmptySettleFunc, llmerrors.ErrLimitExceeded
	}

	inputLeft := i.inputTokensLimit - curr.InputTokens
	outputLeft := i.outputTokensLimit - curr.OutputTokens

	if req.MaxUsage != nil {
		req.Usage = &Usage{}
		req.Usage.OutputTokens = min(
			outputLeft,
			req.MaxUsage.OutputTokens,
		)

		// There is still a case where the usage is zero, and that is if there
		// is no limit left.
		if req.Usage.OutputTokens == 0 {
			return ReserveInfo{}, EmptySettleFunc, llmerrors.ErrLimitExceeded
		}
	}

	// Checked against the tokens left because summing the usage with the
	// current consumption can overflow and bypass the limits.
	if req.Usage.InputTokens > inputLeft || req.Usage.OutputTokens > outputLeft {
		return ReserveInfo{}, EmptySettleFunc, llmerrors.ErrLimitExceeded
	}

	curr.InputTokens += req.Usage.InputTokens
	curr.OutputTokens += req.Usage.OutputTokens
	i.storage[key] = curr

	var settleOnce sync.Once

	// TODO(gabrielcorado): cancel reservations using context.
	return ReserveInfo{OutputTokens: req.Usage.OutputTokens}, func(ctx context.Context, u Usage) {
		settleOnce.Do(func() {
			i.mu.Lock()
			defer i.mu.Unlock()
			curr := i.storage[key]

			// It is most common to have consumed less tokens than it was reserved
			// so we need to adjust the value.
			//
			// If the usage is greater than the requested, we should increase the
			// value stored instead of decreasing. This is already handled by
			// settleTokens.
			//
			// We wrap those into a check if the reported usage is greater than 0
			// in case the provider or recorder weren't able to extract usage
			// information. For those cases we want to keep the reservation values.
			if u.InputTokens > 0 {
				curr.InputTokens = settleTokens(curr.InputTokens, req.Usage.InputTokens, u.InputTokens)
			}
			if u.OutputTokens > 0 {
				curr.OutputTokens = settleTokens(curr.OutputTokens, req.Usage.OutputTokens, u.OutputTokens)
			}

			i.storage[key] = curr
		})
	}, nil
}

// Consumption retrieves the current app limit consumption.
func (i *InMemory) Consumption(app types.Application) Usage {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.storage[newAppKey(app)]
}

// settleTokens replaces the tokens held by a reservation with the usage
// reported on settlement. The reported usage is provider-controlled, so the
// adjustment saturates instead of wrapping around, which would reset the
// consumption back to a small value.
func settleTokens(consumed, reserved, reported uint) uint {
	if reported >= reserved {
		if total := consumed + (reported - reserved); total >= consumed {
			return total
		}
		return math.MaxUint
	}

	// Giving back more than consumed means the reservation was settled twice.
	if diff := reserved - reported; diff <= consumed {
		return consumed - diff
	}
	return 0
}

type appKey struct {
	name string
	uri  string
}

func newAppKey(app types.Application) appKey {
	return appKey{name: app.GetName(), uri: app.GetURI()}
}
