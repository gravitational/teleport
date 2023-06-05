/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
