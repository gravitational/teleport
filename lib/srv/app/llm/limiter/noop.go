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

import "context"

// NewNoop initialzes a [NoopLimiter].
func NewNoop() *NoopLimiter {
	return &NoopLimiter{}
}

// NoopLimiter tokens limiter that does nothing.
type NoopLimiter struct{}

// Close implements [Limiter].
func (n *NoopLimiter) Close(context.Context) error {
	return nil
}

// Run implements [Limiter].
func (n *NoopLimiter) Run(context.Context) error {
	return nil
}

// Reserve implements [Limiter].
func (n *NoopLimiter) Reserve(context.Context, ReserveRequest) (ReserveInfo, SettleFunc, error) {
	return ReserveInfo{
		NotApplicable: true,
	}, EmptySettleFunc, nil
}
