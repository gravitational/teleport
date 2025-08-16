/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package internal

import (
	"context"

	"github.com/gravitational/teleport/lib/srv/alpnproxy"
)

var _ alpnproxy.LocalProxyMiddleware = (*ALPNProxyMiddleware)(nil)

type ALPNProxyMiddleware struct {
	OnNewConnectionFunc func(ctx context.Context, lp *alpnproxy.LocalProxy) error
	OnStartFunc         func(ctx context.Context, lp *alpnproxy.LocalProxy) error
}

func (a ALPNProxyMiddleware) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	if a.OnNewConnectionFunc != nil {
		return a.OnNewConnectionFunc(ctx, lp)
	}
	return nil
}

func (a ALPNProxyMiddleware) OnStart(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	if a.OnStartFunc != nil {
		return a.OnStartFunc(ctx, lp)
	}
	return nil
}
