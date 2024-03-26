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

import grpc from '@grpc/grpc-js';
import { GrpcTransport } from '@protobuf-ts/grpc-transport';
import { TerminalServiceClient } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb.client';

import Logger from 'teleterm/logger';

import { cloneClient } from './cloneableClient';
import * as types from './types';
import { loggingInterceptor } from './interceptors';

export function createTshdClient(
  addr: string,
  credentials: grpc.ChannelCredentials
): types.TshdClient {
  const logger = new Logger('tshd');
  const transport = new GrpcTransport({
    host: addr,
    channelCredentials: credentials,
    interceptors: [loggingInterceptor(logger)],
  });
  return cloneClient(new TerminalServiceClient(transport));
}
