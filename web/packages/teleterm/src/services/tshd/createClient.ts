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
import { TerminalServiceClient } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb.client';
import * as vnetServiceProtobuf from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb.client';

import { CloneableClient, cloneClient } from './cloneableClient';
import * as types from './types';

export function createTshdClient(transport: GrpcTransport): types.TshdClient {
  return cloneClient(new TerminalServiceClient(transport));
}

export function createVnetClient(transport: GrpcTransport): VnetServiceClient {
  return cloneClient(new vnetServiceProtobuf.VnetServiceClient(transport));
}

export type VnetServiceClient =
  CloneableClient<vnetServiceProtobuf.VnetServiceClient>;
