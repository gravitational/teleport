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
