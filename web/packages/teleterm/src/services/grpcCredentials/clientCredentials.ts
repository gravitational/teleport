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
