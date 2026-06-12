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

import { Server, ServerCredentials } from '@grpc/grpc-js';

import Logger from 'teleterm/logger';
import { RuntimeSettings, TERMINATE_MESSAGE } from 'teleterm/mainProcess/types';
import {
  createInsecureServerCredentials,
  createServerCredentials,
  generateAndSaveGrpcCert,
  GrpcCertName,
  readGrpcCert,
  shouldEncryptConnection,
} from 'teleterm/services/grpcCredentials';
import { createStdoutLoggerService } from 'teleterm/services/logger';
import { ptyHostDefinition } from 'teleterm/sharedProcess/api/protogen/ptyHostService_pb.grpc-server';

import { createPtyHostService } from './ptyHost/ptyHostService';

const runtimeSettings = getRuntimeSettings();
initializeLogger();
initializeServer(runtimeSettings);

function getRuntimeSettings(): RuntimeSettings {
  const args = process.argv.slice(2);
  const argName = '--runtimeSettingsJson=';
  const runtimeSettingsJson = args[0].startsWith(argName)
    ? args[0].replace(argName, '')
    : undefined;
  const runtimeSettings: RuntimeSettings =
    runtimeSettingsJson && JSON.parse(runtimeSettingsJson);

  if (!runtimeSettings) {
    throw new Error('Provide process runtime settings');
  }
  return runtimeSettings;
}

function initializeLogger(): void {
  const loggerService = createStdoutLoggerService();
  Logger.init(loggerService);
  const logger = new Logger('uncaught exception');

  process.on('uncaughtException', (error, origin) => {
    logger.error(origin, error);
  });
}

async function initializeServer(
  runtimeSettings: RuntimeSettings
): Promise<void> {
  const address = runtimeSettings.sharedProcess.requestedNetworkAddress;
  const logger = new Logger('gRPC server');
  if (!address) {
    throw new Error('Provide gRPC server address');
  }

  const server = new Server();
  const ptyHostService = createPtyHostService();
  server.addService(ptyHostDefinition, ptyHostService);

  // grpc-js requires us to pass localhost:port for TCP connections,
  const grpcServerAddress = address.replace('tcp://', '');

  try {
    const credentials = await createGrpcCredentials(runtimeSettings);

    server.bindAsync(grpcServerAddress, credentials, (error, port) => {
      sendBoundNetworkPortToStdout(port);

      if (error) {
        return logger.error(error.message);
      }
    });
  } catch (e) {
    logger.error('Could not start shared server', e);
  }

  process.on('message', async message => {
    if (message === TERMINATE_MESSAGE) {
      new Logger('Process').info('Received terminate message, exiting');
      server.forceShutdown();
      await ptyHostService.dispose();
      process.exit(0);
    }
  });
}

function sendBoundNetworkPortToStdout(port: number) {
  console.log(`{CONNECT_GRPC_PORT: ${port}}`);
}

/**
 * Creates credentials for the gRPC server running in the shared process.
 */
async function createGrpcCredentials(
  runtimeSettings: RuntimeSettings
): Promise<ServerCredentials> {
  if (!shouldEncryptConnection(runtimeSettings)) {
    return createInsecureServerCredentials();
  }

  const { certsDir } = runtimeSettings;
  const [sharedKeyPair, rendererCert] = await Promise.all([
    generateAndSaveGrpcCert(certsDir, GrpcCertName.Shared),
    readGrpcCert(certsDir, GrpcCertName.Renderer),
  ]);

  return createServerCredentials(sharedKeyPair, rendererCert);
}
