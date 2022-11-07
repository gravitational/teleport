import { contextBridge } from 'electron';
import { ChannelCredentials } from '@grpc/grpc-js';

import createTshClient from 'teleterm/services/tshd/createClient';
import createMainProcessClient from 'teleterm/mainProcess/mainProcessClient';
import { createFileLoggerService } from 'teleterm/services/logger';
import Logger from 'teleterm/logger';
import { createPtyService } from 'teleterm/services/pty/ptyService';
import {
  GrpcCertName,
  createClientCredentials,
  createInsecureClientCredentials,
  generateAndSaveGrpcCert,
  readGrpcCert,
  shouldEncryptConnection,
} from 'teleterm/services/grpcCredentials';
import { ElectronGlobals, RuntimeSettings } from 'teleterm/types';

const mainProcessClient = createMainProcessClient();
const runtimeSettings = mainProcessClient.getRuntimeSettings();
const loggerService = createFileLoggerService({
  dev: runtimeSettings.dev,
  dir: runtimeSettings.userDataDir,
  name: 'renderer',
});

Logger.init(loggerService);

contextBridge.exposeInMainWorld('loggerService', loggerService);

contextBridge.exposeInMainWorld('electron', getElectronGlobals());

async function getElectronGlobals(): Promise<ElectronGlobals> {
  const [addresses, credentials] = await Promise.all([
    mainProcessClient.getResolvedChildProcessAddresses(),
    createGrpcCredentials(runtimeSettings),
  ]);
  const tshClient = createTshClient(addresses.tsh, credentials.tshd);
  const ptyServiceClient = createPtyService(
    addresses.shared,
    credentials.shared,
    runtimeSettings
  );

  return {
    mainProcessClient,
    tshClient,
    ptyServiceClient,
  };
}

/**
 * For TCP transport, createGrpcCredentials generates the renderer key pair and reads the public key
 * for tshd and the shared process from disk. This lets us set up gRPC clients in the renderer
 * process that connect to the gRPC servers of tshd and the shared process.
 */
async function createGrpcCredentials(
  runtimeSettings: RuntimeSettings
): Promise<{
  // Credentials for talking to the tshd process.
  tshd: ChannelCredentials;
  // Credentials for talking to the shared process.
  shared: ChannelCredentials;
}> {
  if (!shouldEncryptConnection(runtimeSettings)) {
    return {
      tshd: createInsecureClientCredentials(),
      shared: createInsecureClientCredentials(),
    };
  }

  const { certsDir } = runtimeSettings;
  const [rendererKeyPair, tshdCert, sharedCert] = await Promise.all([
    generateAndSaveGrpcCert(certsDir, GrpcCertName.Renderer),
    readGrpcCert(certsDir, GrpcCertName.Tshd),
    readGrpcCert(certsDir, GrpcCertName.Shared),
  ]);

  return {
    tshd: createClientCredentials(rendererKeyPair, tshdCert),
    shared: createClientCredentials(rendererKeyPair, sharedCert),
  };
}
