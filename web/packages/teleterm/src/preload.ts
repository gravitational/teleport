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

import { contextBridge } from 'electron';
import { ChannelCredentials, ServerCredentials } from '@grpc/grpc-js';
import { GrpcTransport } from '@protobuf-ts/grpc-transport';

import {
  createTshdClient,
  createVnetClient,
} from 'teleterm/services/tshd/createClient';
import { loggingInterceptor } from 'teleterm/services/tshd/interceptors';
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
  dir: runtimeSettings.logsDir,
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
  const tshdTransport = new GrpcTransport({
    host: addresses.tsh,
    channelCredentials: credentials.tshd,
    interceptors: [loggingInterceptor(new Logger('tshd'))],
  });
  const tshClient = createTshdClient(tshdTransport);
  const vnetClient = createVnetClient(tshdTransport);
  const ptyServiceClient = createPtyService(
    addresses.shared,
    credentials.shared,
    runtimeSettings,
    {
      noResume: mainProcessClient.configService.get('ssh.noResume').value,
    }
  );
  const {
    setupTshdEventContextBridgeService,
    resolvedAddress: tshdEventsServerAddress,
  } = await createTshdEventsServer(
    runtimeSettings.tshdEvents.requestedNetworkAddress,
    credentials.tshdEvents
  );

  // Here we send to tshd the address of the tshd events server that we just created. This makes
  // tshd prepare a client for the server.
  //
  // All uses of tshClient must wait before updateTshdEventsServerAddress finishes to ensure that
  // the client is ready. Otherwise we run into a risk of causing panics in tshd due to a missing
  // tshd events client.
  await tshClient.updateTshdEventsServerAddress({
    address: tshdEventsServerAddress,
  });

  return {
    mainProcessClient,
    tshClient,
    vnetClient,
    ptyServiceClient,
    setupTshdEventContextBridgeService,
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
    // tsh daemon expects both certs to be created before accepting connections. So even though the
    // renderer process does not use the cert of the main process, it must still wait for the cert
    // to be saved to disk.
    readGrpcCert(certsDir, GrpcCertName.MainProcess),
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
