import { TshClient } from 'teleterm/services/tshd/types';
import { PtyServiceClient } from 'teleterm/services/pty';
import { RuntimeSettings, MainProcessClient } from 'teleterm/mainProcess/types';

import { FileStorage } from 'teleterm/services/fileStorage';
import { AppearanceConfig } from 'teleterm/services/config';

import { Logger, LoggerService } from './services/logger/types';

export {
  Logger,
  LoggerService,
  FileStorage,
  RuntimeSettings,
  MainProcessClient,
  AppearanceConfig,
};

export type ElectronGlobals = {
  readonly mainProcessClient: MainProcessClient;
  readonly tshClient: TshClient;
  readonly ptyServiceClient: PtyServiceClient;
};
