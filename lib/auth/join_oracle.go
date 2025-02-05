// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// RegisterUsingOracleMethod registers the caller using the Oracle join method and
// returns signed certs to join the cluster.
func (a *Server) RegisterUsingOracleMethod(
	ctx context.Context,
	tokenReq *types.RegisterUsingTokenRequest,
	challengeResponse client.RegisterOracleChallengeResponseFunc,
) (certs *proto.Certs, err error) {
	return nil, trace.NotImplemented("Not implemented")
}
