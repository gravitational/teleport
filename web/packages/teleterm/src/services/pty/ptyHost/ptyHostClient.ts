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

import { ChannelCredentials, Metadata } from '@grpc/grpc-js';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';

import {
  PtyHostClient as GrpcClient,
  PtyCreate,
  PtyId,
} from 'teleterm/sharedProcess/ptyHost';

import { PtyHostClient } from '../types';

import { PtyEventsStreamHandler } from './ptyEventsStreamHandler';

export function createPtyHostClient(
  address: string,
  credentials: ChannelCredentials
): PtyHostClient {
  const client = new GrpcClient(address, credentials);
  return {
    createPtyProcess(ptyOptions) {
      const request = new PtyCreate()
        .setArgsList(ptyOptions.args)
        .setPath(ptyOptions.path)
        .setEnv(Struct.fromJavaScript(ptyOptions.env));

      if (ptyOptions.cwd) {
        request.setCwd(ptyOptions.cwd);
      }
      if (ptyOptions.initMessage) {
        request.setInitMessage(ptyOptions.initMessage);
      }

      return new Promise<string>((resolve, reject) => {
        client.createPtyProcess(request, (error, response) => {
          if (error) {
            reject(error);
          } else {
            resolve(response.toObject().id);
          }
        });
      });
    },
    getCwd(ptyId) {
      return new Promise((resolve, reject) => {
        client.getCwd(new PtyId().setId(ptyId), (error, response) => {
          if (error) {
            reject(error);
          } else {
            resolve(response.toObject().cwd);
          }
        });
      });
    },
    exchangeEvents(ptyId) {
      const metadata = new Metadata();
      metadata.set('ptyId', ptyId);
      const stream = client.exchangeEvents(metadata);
      return new PtyEventsStreamHandler(stream, ptyId);
    },
  };
}
