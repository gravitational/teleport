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

package service

import (
	"context"
	"math/rand"
	"time"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
)

// Service implements gRPC service for VNet.
type Service struct {
	api.UnimplementedVnetServiceServer
}

func (s *Service) Start(ctx context.Context, req *api.StartRequest) (*api.StartResponse, error) {
	n := rand.Intn(10)
	randomDelay := time.Duration(n) * 100 * time.Millisecond
	time.Sleep(randomDelay + 400*time.Millisecond)
	return &api.StartResponse{}, nil
}

func (s *Service) Stop(ctx context.Context, req *api.StopRequest) (*api.StopResponse, error) {
	return &api.StopResponse{}, nil
}

// Close stops the current VNet instance and prevents new instances from being started.
//
// Intended for cleanup code when tsh daemon gets terminated.
func (s *Service) Close() error {
	return nil
}
