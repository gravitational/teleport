import { Server, ServerCredentials } from '@grpc/grpc-js';

import createLoggerService from 'teleterm/services/logger';
import { RuntimeSettings } from 'teleterm/mainProcess/types';
import Logger from 'teleterm/logger';

import { PtyHostService } from './api/protogen/ptyHostService_grpc_pb';
import { createPtyHostService } from './ptyHost/ptyHostService';

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

function initializeLogger(runtimeSettings: RuntimeSettings): void {
  const loggerService = createLoggerService({
    dev: runtimeSettings.dev,
    dir: runtimeSettings.userDataDir,
    name: 'shared',
  });

  Logger.init(loggerService);
  const logger = new Logger();
  process.on('uncaughtException', logger.error);
}

function initializeServer(runtimeSettings: RuntimeSettings): void {
  const address = runtimeSettings.sharedProcess.networkAddr;
  const logger = new Logger('gRPC server');
  if (!address) {
    throw new Error('Provide gRPC server address');
  }

  const server = new Server();
  // @ts-expect-error we have a typed service
  server.addService(PtyHostService, createPtyHostService());
  server.bindAsync(address, ServerCredentials.createInsecure(), error => {
    if (error) {
      return logger.error(error.message);
    }
    server.start();
  });

  process.once('exit', () => {
    server.forceShutdown();
  });
}

const runtimeSettings = getRuntimeSettings();
initializeLogger(runtimeSettings);
initializeServer(runtimeSettings);
