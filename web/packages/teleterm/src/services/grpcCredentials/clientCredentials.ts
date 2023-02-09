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

import { ChannelCredentials, credentials } from '@grpc/grpc-js';

import { RuntimeSettings } from 'teleterm/mainProcess/types';

import {
  readGrpcCert,
  generateAndSaveGrpcCert,
  shouldEncryptConnection,
} from './helpers';
import { GrpcCertName } from './types';

export async function getClientCredentials(
  runtimeSettings: RuntimeSettings
): Promise<{
  tsh: ChannelCredentials;
  shared: ChannelCredentials;
}> {
  if (shouldEncryptConnection(runtimeSettings)) {
    const certs = await getCerts(runtimeSettings.certsDir);
    return {
      tsh: createSecureCredentials(certs.clientKeyPair, certs.tshServerCert),
      shared: createSecureCredentials(
        certs.clientKeyPair,
        certs.sharedServerCert
      ),
    };
  }

  return {
    tsh: createInsecureCredentials(),
    shared: createInsecureCredentials(),
  };
}

async function getCerts(certsDir: string): Promise<{
  clientKeyPair: { cert: Buffer; key: Buffer };
  tshServerCert: Buffer;
  sharedServerCert: Buffer;
}> {
  const [clientKeyPair, tshServerCert, sharedServerCert] = await Promise.all([
    generateAndSaveGrpcCert(certsDir, GrpcCertName.Client),
    readGrpcCert(certsDir, GrpcCertName.TshServer),
    readGrpcCert(certsDir, GrpcCertName.SharedServer),
  ]);
  return { clientKeyPair, tshServerCert, sharedServerCert };
}

function createSecureCredentials(
  clientKeyPair: { cert: Buffer; key: Buffer },
  serverCert: Buffer
): ChannelCredentials {
  return credentials.createSsl(
    serverCert,
    clientKeyPair.key,
    clientKeyPair.cert
  );
}

function createInsecureCredentials(): ChannelCredentials {
  return credentials.createInsecure();
}
