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

import { Kind } from 'teleterm/ui/services/workspacesService';
import { FileStorage } from 'teleterm/services/fileStorage';

import { ConfigService } from '../services/config';

export type RuntimeSettings = {
  /**
   * dev controls whether the app runs in development mode. This mostly controls what kind of URL
   * the Electron app opens after launching and affects the expectations of the app wrt the
   * environment its being executed in.
   */
  dev: boolean;
  /**
   * debug controls the level of logs emitted by tshd and gates access to devtools. In a packaged
   * app it's false by default, but it can be enabled by executing the packaged app with the
   * --connect-debug flag.
   *
   * dev implies debug.
   */
  debug: boolean;
  /**
   * insecure controls whether tsh invocations and Connect My Computer agents are run in insecure
   * mode. This typically skips the cert checks when talking to the proxy.
   *
   * False by default in both packaged apps and in development mode. It can be turned on by:
   * - Starting a packaged version of the app with the --insecure flag.
   * - Starting the app in dev mode with the CONNECT_INSECURE env var.
   */
  insecure: boolean;
  userDataDir: string;
  // Points to a directory that should be prepended to PATH. Only present in the packaged version.
  binDir: string | undefined;
  certsDir: string;
  kubeConfigsDir: string;
  defaultShell: string;
  platform: Platform;
  tshd: {
    requestedNetworkAddress: string;
    binaryPath: string;
    homeDir: string;
    flags: string[];
  };
  sharedProcess: {
    requestedNetworkAddress: string;
  };
  tshdEvents: {
    requestedNetworkAddress: string;
  };
  installationId: string;
  arch: string;
  osVersion: string;
  appVersion: string;
};

export type MainProcessClient = {
  getRuntimeSettings(): RuntimeSettings;
  getResolvedChildProcessAddresses(): Promise<ChildProcessAddresses>;
  openTerminalContextMenu(): void;
  openTabContextMenu(options: TabContextMenuOptions): void;
  showFileSaveDialog(
    filePath: string
  ): Promise<{ canceled: boolean; filePath: string | undefined }>;
  configService: ConfigService;
  fileStorage: FileStorage;
  removeKubeConfig(options: {
    relativePath: string;
    isDirectory?: boolean;
  }): Promise<void>;
  forceFocusWindow(): void;
  /**
   * The promise returns true if tsh got successfully symlinked, false if the user closed the
   * osascript prompt. The promise gets rejected if osascript encountered an error.
   */
  symlinkTshMacOs(): Promise<boolean>;
  /**
   * The promise returns true if the tsh symlink got removed, false if the user closed the osascript
   * prompt. The promise gets rejected if osascript encountered an error.
   */
  removeTshSymlinkMacOs(): Promise<boolean>;

  /** Opens config file and returns a path to it. */
  openConfigFile(): Promise<string>;
  shouldUseDarkColors(): boolean;
  /** Subscribes to updates of the native theme. Returns a cleanup function. */
  subscribeToNativeThemeUpdate: (
    listener: (value: { shouldUseDarkColors: boolean }) => void
  ) => {
    cleanup: () => void;
  };
};

export type ChildProcessAddresses = {
  tsh: string;
  shared: string;
};

export type GrpcServerAddresses = ChildProcessAddresses & {
  tshdEvents: string;
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

export const TerminalContextMenuEventChannel =
  'TerminalContextMenuEventChannel';
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
  Set = 'Set',
  GetConfigError = 'GetConfigError',
}

export enum FileStorageEventType {
  Get = 'Get',
  Put = 'Put',
  Write = 'Write',
  Replace = 'Replace',
  GetFilePath = 'GetFilePath',
  GetFileLoadingError = 'GetFileLoadingError',
}
