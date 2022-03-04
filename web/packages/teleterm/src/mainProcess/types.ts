import { ConfigService } from '../services/config';
import { Kind } from 'teleterm/ui/services/workspacesService';

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
  openClusterContextMenu(options: ClusterContextMenuOptions): void;
  openTabContextMenu(options: TabContextMenuOptions): void;
  configService: ConfigService;
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

export const ClusterContextMenuEventChannel = 'ClusterContextMenuEventChannel';
export const TerminalContextMenuEventChannel = 'TerminalContextMenuEventChannel';
export const TabContextMenuEventChannel = 'TabContextMenuEventChannel';
export const ConfigServiceEventChannel = 'ConfigServiceEventChannel';

export enum ClusterContextMenuEventType {
  Refresh = 'Refresh',
  Login = 'Login',
  Logout = 'Logout',
  Remove = 'Remove',
}

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
