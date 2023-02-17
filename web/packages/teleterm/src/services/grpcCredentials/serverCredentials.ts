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

import { ServerCredentials } from '@grpc/grpc-js';

import { RuntimeSettings } from 'teleterm/mainProcess/types';

import {
  readGrpcCert,
  generateAndSaveGrpcCert,
  shouldEncryptConnection,
} from './helpers';
import { GrpcCertName } from './types';

export async function getServerCredentials(
  runtimeSettings: RuntimeSettings
): Promise<{ shared: ServerCredentials }> {
  if (shouldEncryptConnection(runtimeSettings)) {
    return { shared: await createSecureCredentials(runtimeSettings.certsDir) };
  }
  return { shared: createInsecureCredentials() };
}

async function createSecureCredentials(
  certsDir: string
): Promise<ServerCredentials> {
  const [serverKeyPair, clientCert] = await Promise.all([
    generateAndSaveGrpcCert(certsDir, GrpcCertName.SharedServer),
    readGrpcCert(certsDir, GrpcCertName.Client),
  ]);

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

function createInsecureCredentials(): ServerCredentials {
  return ServerCredentials.createInsecure();
}
