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
import { VnetServiceClient } from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb.grpc-client';

import Logger from 'teleterm/logger';
import * as uri from 'teleterm/ui/uri';

import { TshAbortSignal } from './types';
import { loggingInterceptor } from './interceptors';

export function createVnetClient(
  addr: string,
  credentials: grpc.ChannelCredentials
): VnetClient {
  const logger = new Logger('vnet');
  const vnet = new VnetServiceClient(addr, credentials, {
    interceptors: [loggingInterceptor(logger)],
  });

  return {
    start(
      rootClusterUri: uri.RootClusterUri,
      abortSignal?: TshAbortSignal
    ): Promise<void> {
      return withAbort(abortSignal, callRef => {
        return new Promise((resolve, reject) => {
          callRef.current = vnet.start({ rootClusterUri }, err => {
            if (err) {
              reject(err);
            } else {
              resolve();
            }
          });
        });
      });
    },

    stop(
      rootClusterUri: uri.RootClusterUri,
      abortSignal?: TshAbortSignal
    ): Promise<void> {
      return withAbort(abortSignal, callRef => {
        return new Promise((resolve, reject) => {
          callRef.current = vnet.stop({ rootClusterUri }, err => {
            if (err) {
              reject(err);
            } else {
              resolve();
            }
          });
        });
      });
    },
  };
}

export type VnetClient = {
  start: (
    rootClusterUri: uri.RootClusterUri,
    abortSignal?: TshAbortSignal
  ) => Promise<void>;

  stop: (
    rootClusterUri: uri.RootClusterUri,
    abortSignal?: TshAbortSignal
  ) => Promise<void>;
};

type CallRef = {
  current: {
    cancel(): void;
  } | null;
};

// TODO(ravicious): Extract withAbort and use it in both tshd and vnet clients.
async function withAbort<T>(
  sig: TshAbortSignal | undefined,
  cb: (ref: CallRef) => Promise<T>
) {
  const ref: CallRef = {
    current: null,
  };

  const abort = () => {
    ref?.current?.cancel();
  };

  sig?.addEventListener(abort);

  return cb(ref).finally(() => {
    sig?.removeEventListener(abort);
  });
}
