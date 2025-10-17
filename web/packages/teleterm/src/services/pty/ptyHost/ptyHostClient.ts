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

import { ChannelCredentials } from '@grpc/grpc-js';
import { GrpcTransport } from '@protobuf-ts/grpc-transport';

import { Struct } from 'gen-proto-ts/google/protobuf/struct_pb';
import { CreatePtyProcessRequest } from 'gen-proto-ts/teleport/web/teleterm/ptyhost/v1/pty_host_service_pb';
import { PtyHostServiceClient as GrpcClient } from 'gen-proto-ts/teleport/web/teleterm/ptyhost/v1/pty_host_service_pb.client';

import { PtyHostClient } from '../types';
import { PtyEventsStreamHandler } from './ptyEventsStreamHandler';

export function createPtyHostClient(
  address: string,
  credentials: ChannelCredentials
): PtyHostClient {
  const transport = new GrpcTransport({
    host: address,
    channelCredentials: credentials,
  });
  const client = new GrpcClient(transport);

  return {
    async createPtyProcess(ptyOptions) {
      const request = CreatePtyProcessRequest.create({
        args: ptyOptions.args,
        path: ptyOptions.path,
        env: Struct.fromJson(ptyOptions.env),
        useConpty: ptyOptions.useConpty,
      });

      if (ptyOptions.cwd) {
        request.cwd = ptyOptions.cwd;
      }
      if (ptyOptions.initMessage) {
        request.initMessage = ptyOptions.initMessage;
      }

      const { response } = await client.createPtyProcess(request);
      return response.id;
    },
    async getCwd(ptyId) {
      const { response } = await client.getCwd({ id: ptyId });
      return response.cwd;
    },
    managePtyProcess(ptyId) {
      const stream = client.managePtyProcess({ meta: { ptyId: ptyId } });
      return new PtyEventsStreamHandler(stream, ptyId);
    },
  };
}
