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

package expression

import (
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
)

// Environment in which expressions will be evaluated.
type Environment struct {
	Metadata             *headerv1.Metadata
	Spec                 *machineidv1.BotInstanceSpec
	LatestHeartbeat      *machineidv1.BotInstanceStatusHeartbeat
	LatestAuthentication *machineidv1.BotInstanceStatusAuthentication
}

func (e *Environment) GetMetadata() *headerv1.Metadata {
	if e == nil {
		return nil
	}
	return e.Metadata
}

func (e *Environment) GetSpec() *machineidv1.BotInstanceSpec {
	if e == nil {
		return nil
	}
	return e.Spec
}

func (e *Environment) GetLatestHeartbeat() *machineidv1.BotInstanceStatusHeartbeat {
	if e == nil {
		return nil
	}
	return e.LatestHeartbeat
}

func (e *Environment) GetLatestAuthentication() *machineidv1.BotInstanceStatusAuthentication {
	if e == nil {
		return nil
	}
	return e.LatestAuthentication
}
