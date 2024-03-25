/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import grpc from '@grpc/grpc-js';
import * as vnetServiceProtobuf from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb.client';
import { GrpcTransport } from '@protobuf-ts/grpc-transport';

import Logger from 'teleterm/logger';

import { loggingInterceptor } from './interceptors';
import { CloneableClient, cloneClient } from './cloneableClient';

export type VnetServiceClient =
  CloneableClient<vnetServiceProtobuf.VnetServiceClient>;

export function createVnetClient(
  addr: string,
  credentials: grpc.ChannelCredentials
): VnetServiceClient {
  const logger = new Logger('vnet');
  const transport = new GrpcTransport({
    host: addr,
    channelCredentials: credentials,
    interceptors: [loggingInterceptor(logger)],
  });
  const client = cloneClient(
    new vnetServiceProtobuf.VnetServiceClient(transport)
  );

  return client;
}
