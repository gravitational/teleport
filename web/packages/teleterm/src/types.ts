import { TshClient } from 'teleterm/services/tshd/types';
import { PtyServiceClient } from 'teleterm/services/pty/types';
import { RuntimeSettings, MainProcessClient } from 'teleterm/mainProcess/types';
import { Logger, LoggerService } from './services/logger/types';
import { FileStorage } from 'teleterm/services/fileStorage';
import { AppearanceConfig } from 'teleterm/services/config';

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
  readonly loggerService: LoggerService;
};
