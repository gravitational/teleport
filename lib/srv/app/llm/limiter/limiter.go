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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	llmquota "github.com/gravitational/teleport/lib/llm/quota"
)

// ReserveRequest is a request to reserve tokens out of the application limits.
type ReserveRequest struct {
	// App is the application that is serving the request.
	App types.Application
	// Usage is the exact amount to be reserved.
	// Mutually exclusive with MaxUsage.
	Usage *llmquota.Usage
	// MaxUsage asks for the available limit amount with max value defined by
	// this property. The values exact reserved values will be returned by
	// the reserve function.
	MaxUsage *llmquota.Usage
}

func (r ReserveRequest) CheckAndSetDefaults() error {
	switch {
	case r.App == nil:
		return trace.BadParameter("must provide App")
	case r.Usage != nil && r.MaxUsage != nil:
		return trace.BadParameter("can only provide Usage or MaxUsage, not both")
	case r.Usage == nil && r.MaxUsage == nil:
		return trace.BadParameter("must provide Usage or MaxUsage")
	default:
		return nil
	}
}

// ReserveInfo is the reservation info.
type ReserveInfo struct {
	// NotApplicable indicates the request is exempt from limiting. Callers can
	// still expect to receive a non-nil settle function, for example
	// [EmptySettleFunc].
	NotApplicable bool
	// OutputTokens is the max output tokens the provider can generate that
	// won't exceed the limit.
	OutputTokens uint
}

// SettleFunc settles a reservation with the usage reported for the request.
// Settling an empty [Usage] cancels the reservation.
type SettleFunc func(context.Context, llmquota.Usage)

// EmptySettleFunc is a no-op [SettleFunc], returned by limiters when there is
// nothing to settle.
func EmptySettleFunc(context.Context, llmquota.Usage) {}
