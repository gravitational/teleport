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

import { contextBridge } from 'electron';
import { ChannelCredentials, ServerCredentials } from '@grpc/grpc-js';

import createTshClient from 'teleterm/services/tshd/createClient';
import createMainProcessClient from 'teleterm/mainProcess/mainProcessClient';
import { createFileLoggerService } from 'teleterm/services/logger';
import Logger from 'teleterm/logger';
import { createPtyService } from 'teleterm/services/pty/ptyService';
import {
  GrpcCertName,
  createClientCredentials,
  createServerCredentials,
  createInsecureClientCredentials,
  createInsecureServerCredentials,
  generateAndSaveGrpcCert,
  readGrpcCert,
  shouldEncryptConnection,
} from 'teleterm/services/grpcCredentials';
import { ElectronGlobals, RuntimeSettings } from 'teleterm/types';
import { createTshdEventsServer } from 'teleterm/services/tshdEvents';

const mainProcessClient = createMainProcessClient();
const runtimeSettings = mainProcessClient.getRuntimeSettings();
const loggerService = createFileLoggerService({
  dev: runtimeSettings.dev,
  dir: runtimeSettings.userDataDir,
  name: 'renderer',
});

Logger.init(loggerService);
const logger = new Logger('preload');

contextBridge.exposeInMainWorld('loggerService', loggerService);

contextBridge.exposeInMainWorld(
  'electron',
  withErrorLogging(getElectronGlobals())
);

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
  const { subscribeToTshdEvent, resolvedAddress: tshdEventsServerAddress } =
    await createTshdEventsServer(
      runtimeSettings.tshdEvents.requestedNetworkAddress,
      credentials.tshdEvents
    );

  // Here we send to tshd the address of the tshd events server that we just created. This makes
  // tshd prepare a client for the server.
  //
  // All uses of tshClient must wait before updateTshdEventsServerAddress finishes to ensure that
  // the client is ready. Otherwise we run into a risk of causing panics in tshd due to a missing
  // tshd events client.
  await tshClient.updateTshdEventsServerAddress(tshdEventsServerAddress);

  return {
    mainProcessClient,
    tshClient,
    ptyServiceClient,
    subscribeToTshdEvent,
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
  // Credentials for the tshd events server running in the renderer process.
  tshdEvents: ServerCredentials;
}> {
  if (!shouldEncryptConnection(runtimeSettings)) {
    return {
      tshd: createInsecureClientCredentials(),
      shared: createInsecureClientCredentials(),
      tshdEvents: createInsecureServerCredentials(),
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
    tshdEvents: createServerCredentials(rendererKeyPair, tshdCert),
  };
}

// withErrorLogging logs the error if the promise returns a rejected value. Electron's contextBridge
// loses the stack trace, so we want to log the error with its stack before it crosses the
// contextBridge.
async function withErrorLogging<ReturnValue>(
  promise: Promise<ReturnValue>
): Promise<ReturnValue> {
  try {
    return await promise;
  } catch (e) {
    logger.error(e);
    throw e;
  }
}
