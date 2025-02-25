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

import { DeepLinkParseResult } from 'teleterm/deepLinks';
import { CreateAgentConfigFileArgs } from 'teleterm/mainProcess/createAgentConfigFile';
import { FileStorage } from 'teleterm/services/fileStorage';
import { Document } from 'teleterm/ui/services/workspacesService';
import { RootClusterUri } from 'teleterm/ui/uri';

import { ConfigService } from '../services/config';
import { Shell } from './shell';

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
  sessionDataDir: string;
  tempDataDir: string;
  // Points to a directory that should be prepended to PATH. Only present in the packaged version.
  binDir: string | undefined;
  certsDir: string;
  kubeConfigsDir: string;
  // TODO(ravicious): Replace with app.getPath('logs'). We started storing logs under a custom path.
  // Before switching to the recommended path, we need to investigate the impact of this change.
  // https://www.electronjs.org/docs/latest/api/app#appgetpathname
  logsDir: string;
  /** Identifier of default OS shell. */
  defaultOsShellId: string;
  availableShells: Shell[];
  platform: Platform;
  agentBinaryPath: string;
  tshd: {
    requestedNetworkAddress: string;
    binaryPath: string;
    homeDir: string;
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
  /**
   * The {@link appVersion} is set to a real version only for packaged apps that went through our CI build pipeline.
   * In local builds, both for the development version and for packaged apps, settings.appVersion is set to 1.0.0-dev.
   */
  isLocalBuild: boolean;
  username: string;
  hostname: string;
};

export type MainProcessClient = {
  /** Subscribes to updates of the native theme. Returns a cleanup function. */
  subscribeToNativeThemeUpdate: (
    listener: (value: { shouldUseDarkColors: boolean }) => void
  ) => {
    cleanup: () => void;
  };
  subscribeToAgentUpdate: (
    rootClusterUri: RootClusterUri,
    listener: (state: AgentProcessState) => void
  ) => {
    cleanup: () => void;
  };
  subscribeToDeepLinkLaunch: (
    listener: (args: DeepLinkParseResult) => void
  ) => {
    cleanup: () => void;
  };

  getRuntimeSettings(): RuntimeSettings;
  getResolvedChildProcessAddresses(): Promise<ChildProcessAddresses>;
  openTerminalContextMenu(): void;
  openTabContextMenu(options: TabContextMenuOptions): void;
  showFileSaveDialog(
    filePath: string
  ): Promise<{ canceled: boolean; filePath: string | undefined }>;
  /**
   * saveTextToFile shows the save file dialog that lets the user pick a file location. Once the
   * location is picked, it saves the text to the location, overwriting an existing file if any.
   *
   * If the user closes the dialog, saveTextToFile returns early with canceled set to true. The
   * caller must inspect this value before assuming that the file was saved.
   *
   * If writing to the file fails, saveTextToFile returns a rejected promise.
   */
  saveTextToFile(options: {
    text: string;
    /**
     * The name for the file that will be suggested in the save file dialog.
     */
    defaultBasename: string;
  }): Promise<{
    /**
     * Whether the dialog was closed by the user or not.
     */
    canceled: boolean;
  }>;
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
  downloadAgent(): Promise<void>;
  verifyAgent(): Promise<void>;
  createAgentConfigFile(args: CreateAgentConfigFileArgs): Promise<void>;
  openAgentLogsDirectory(args: {
    rootClusterUri: RootClusterUri;
  }): Promise<void>;
  runAgent(args: { rootClusterUri: RootClusterUri }): Promise<void>;
  isAgentConfigFileCreated(args: {
    rootClusterUri: RootClusterUri;
  }): Promise<boolean>;
  killAgent(args: { rootClusterUri: RootClusterUri }): Promise<void>;
  removeAgentDirectory(args: { rootClusterUri: RootClusterUri }): Promise<void>;
  /**
   * tryRemoveConnectMyComputerAgentBinary removes the agent binary but only if all agents are
   * stopped.
   *
   * Rejects on filesystem errors.
   */
  tryRemoveConnectMyComputerAgentBinary(): Promise<void>;
  getAgentState(args: { rootClusterUri: RootClusterUri }): AgentProcessState;
  getAgentLogs(args: { rootClusterUri: RootClusterUri }): string;
  /**
   * Signals to the windows manager that the UI has been fully initialized, that is the user has
   * interacted with the relevant modals during startup and is free to use the app.
   */
  signalUserInterfaceReadiness(args: { success: boolean }): void;
  refreshClusterList(): void;
};

export type ChildProcessAddresses = {
  tsh: string;
  shared: string;
};

export type GrpcServerAddresses = ChildProcessAddresses & {
  tshdEvents: string;
};

export type Platform = NodeJS.Platform;

export type AgentProcessState =
  | {
      status: 'not-started';
    }
  | {
      status: 'running';
    }
  | {
      status: 'exited';
      code: number | null;
      signal: NodeJS.Signals | null;
      exitedSuccessfully: boolean;
      /** Fragment of a stack trace when the process did not exit successfully. */
      logs?: string;
    }
  | {
      // TODO(ravicious): 'error' should not be considered a separate process state. Instead,
      // AgentRunner.start should not resolve until 'spawn' is emitted or reject if 'error' is
      // emitted. AgentRunner.kill should not resolve until 'exit' is emitted or reject if 'error'
      // is emitted.
      status: 'error';
      message: string;
    };

export interface ClusterContextMenuOptions {
  isClusterConnected: boolean;

  onRefresh(): void;

  onLogin(): void;

  onLogout(): void;

  onRemove(): void;
}

export interface TabContextMenuOptions {
  document: Document;
  onClose(): void;
  onCloseOthers(): void;
  onCloseToRight(): void;
  onDuplicatePty(): void;
  onReopenPtyInShell(shell: Shell): void;
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
  ReopenPtyInShell = 'ReopenPtyInShell',
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
  GetFileName = 'GetFileName',
  GetFileLoadingError = 'GetFileLoadingError',
}

/*
 * IPC channel enums
 *
 * The enum values are used as IPC channels [1], so they should be unique across all enums. That's
 * why the values are prefixed with the recipient name.
 *
 * The enums are grouped by the recipient, e.g. RendererIpc contains messages sent from the main
 * process to the renderer, WindowsManagerIpc contains messages sent from the renderer to the
 * windows manager (which lives in the main process).
 *
 * [1] https://www.electronjs.org/docs/latest/tutorial/ipc
 */

export enum RendererIpc {
  NativeThemeUpdate = 'renderer-native-theme-update',
  ConnectMyComputerAgentUpdate = 'renderer-connect-my-computer-agent-update',
  DeepLinkLaunch = 'renderer-deep-link-launch',
}

export enum MainProcessIpc {
  GetRuntimeSettings = 'main-process-get-runtime-settings',
  TryRemoveConnectMyComputerAgentBinary = 'main-process-try-remove-connect-my-computer-agent-binary',
  RefreshClusterList = 'main-process-refresh-cluster-list',
  DownloadConnectMyComputerAgent = 'main-process-connect-my-computer-download-agent',
  VerifyConnectMyComputerAgent = 'main-process-connect-my-computer-verify-agent',
  SaveTextToFile = 'main-process-save-text-to-file',
}

export enum WindowsManagerIpc {
  SignalUserInterfaceReadiness = 'windows-manager-signal-user-interface-readiness',
}

/**
 * A custom message to gracefully quit a process.
 * It is sent to the child process with `process.send`.
 *
 * We need this because `process.kill('SIGTERM')` doesn't work on Windows,
 * so we couldn't run any cleanup logic.
 */
export const TERMINATE_MESSAGE = 'TERMINATE_MESSAGE';
