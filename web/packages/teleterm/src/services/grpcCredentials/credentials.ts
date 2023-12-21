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

import {
  ChannelCredentials,
  credentials,
  ServerCredentials,
} from '@grpc/grpc-js';

import { RuntimeSettings } from 'teleterm/mainProcess/types';

export function createClientCredentials(
  clientKeyPair: { cert: Buffer; key: Buffer },
  serverCert: Buffer
): ChannelCredentials {
  return credentials.createSsl(
    serverCert,
    clientKeyPair.key,
    clientKeyPair.cert
  );
}

export function createServerCredentials(
  serverKeyPair: { cert: Buffer; key: Buffer },
  clientCert: Buffer
): ServerCredentials {
  return ServerCredentials.createSsl(
    clientCert,
    [
      {
        cert_chain: serverKeyPair.cert,
        private_key: serverKeyPair.key,
      },
    ],
    true
  );
}

export function createInsecureClientCredentials(): ChannelCredentials {
  return credentials.createInsecure();
}

export function createInsecureServerCredentials(): ServerCredentials {
  return ServerCredentials.createInsecure();
}

/**
 * Checks if the gRPC connection should be encrypted.
 * The only source of truth is the type of tshd protocol.
 * Any protocol other than `unix` should be encrypted.
 * The same check is performed on the tshd side.
 */
export function shouldEncryptConnection(
  runtimeSettings: RuntimeSettings
): boolean {
  return (
    new URL(runtimeSettings.tshd.requestedNetworkAddress).protocol !== 'unix:'
  );
}
