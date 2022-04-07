import { ConfigService } from '../services/config';
import { Kind } from 'teleterm/ui/services/workspacesService';
import { FileStorage } from 'teleterm/services/fileStorage';

export type RuntimeSettings = {
  dev: boolean;
  userDataDir: string;
  defaultShell: string;
  platform: Platform;
  tshd: {
    insecure: boolean;
    networkAddr: string;
    binaryPath: string;
    homeDir: string;
    flags: string[];
  };
};

export type MainProcessClient = {
  getRuntimeSettings(): RuntimeSettings;
  openTerminalContextMenu(): void;
  openTabContextMenu(options: TabContextMenuOptions): void;
  configService: ConfigService;
  fileStorage: FileStorage;
};

export type Platform = NodeJS.Platform;

export interface ClusterContextMenuOptions {
  isClusterConnected: boolean;
  onRefresh(): void;
  onLogin(): void;
  onLogout(): void;
  onRemove(): void;
}

export interface TabContextMenuOptions {
  documentKind: Kind;
  onClose(): void;
  onCloseOthers(): void;
  onCloseToRight(): void;
  onDuplicatePty(): void;
}

export const TerminalContextMenuEventChannel = 'TerminalContextMenuEventChannel';
export const TabContextMenuEventChannel = 'TabContextMenuEventChannel';
export const ConfigServiceEventChannel = 'ConfigServiceEventChannel';
export const FileStorageEventChannel = 'FileStorageEventChannel';

export enum TabContextMenuEventType {
  Close = 'Close',
  CloseOthers = 'CloseOthers',
  CloseToRight = 'CloseToRight',
  DuplicatePty = 'DuplicatePty',
}

export enum ConfigServiceEventType {
  Get = 'Get',
  Update = 'Update',
}

export enum FileStorageEventType {
  Get = 'Get',
  Put = 'Put',
  PutAllSync = 'PutAllSync',
}
