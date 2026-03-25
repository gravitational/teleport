/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { GrpcTransport } from '@protobuf-ts/grpc-transport';

import {
  AutoUpdateServiceClient,
  IAutoUpdateServiceClient,
} from 'gen-proto-ts/teleport/lib/teleterm/auto_update/v1/auto_update_service_pb.client';
import {
  ITerminalServiceClient,
  TerminalServiceClient,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb.client';
import {
  IVnetServiceClient,
  VnetServiceClient,
} from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb.client';

import { CloneableClient, cloneClient } from './cloneableClient';

// Creating the client type based on the interface (ITerminalServiceClient) and not the class
// (TerminalServiceClient) lets us omit a bunch of properties when mocking a client.
export type TshdClient = CloneableClient<ITerminalServiceClient>;

export type VnetClient = CloneableClient<IVnetServiceClient>;

export type AutoUpdateClient = CloneableClient<IAutoUpdateServiceClient>;

export function createTshdClient(transport: GrpcTransport): TshdClient {
  return cloneClient(new TerminalServiceClient(transport));
}

export function createVnetClient(transport: GrpcTransport): VnetClient {
  return cloneClient(new VnetServiceClient(transport));
}
export function createAutoUpdateClient(
  transport: GrpcTransport
): AutoUpdateClient {
  return cloneClient(new AutoUpdateServiceClient(transport));
}
