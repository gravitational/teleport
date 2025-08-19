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

import fs from 'fs';
import os from 'os';
import path from 'path';

import { app } from 'electron';

import { loadInstallationId } from './loadInstallationId';
import { getAvailableShells, getDefaultShell } from './shell';
import { GrpcServerAddresses, RuntimeSettings } from './types';

const { argv, env } = process;

const RESOURCES_PATH = app.isPackaged
  ? process.resourcesPath
  : path.join(__dirname, '../../../../');

const TSH_BIN_ENV_VAR = 'CONNECT_TSH_BIN_PATH';
// __dirname of this file in dev mode is teleport/web/packages/teleterm/build/app/main
// We default to teleport/build/tsh.
// prettier-ignore
const TSH_BIN_DEFAULT_PATH_FOR_DEV = path.resolve(
  __dirname,
  '..', '..', '..', '..', '..', '..',
  'build', process.platform === 'win32' ? 'tsh.exe' : 'tsh',
);

// Refer to the docs of RuntimeSettings type for an explanation behind dev, debug and insecure.
const dev = env.NODE_ENV === 'development';
// --debug is reserved by Node, so we have to use another flag.
const debug = argv.includes('--connect-debug') || dev;
const insecure =
  argv.includes('--insecure') ||
  // --insecure is already in our docs, but let's add --connect-insecure too in case Node or
  // Electron reserves it one day.
  argv.includes('--connect-insecure') ||
  // The flag is needed because it's not easy to pass a flag to the app in dev mode. `pnpm
  // start-term` causes a bunch of package scripts to be executed and each would have to pass the
  // flag one level down.
  (dev && !!env.CONNECT_INSECURE);

export async function getRuntimeSettings(): Promise<RuntimeSettings> {
  const userDataDir = app.getPath('userData');
  const sessionDataDir = app.getPath('sessionData');
  const tempDataDir = app.getPath('temp');
  const {
    tsh: tshAddress,
    shared: sharedAddress,
    tshdEvents: tshdEventsAddress,
  } = requestGrpcServerAddresses();
  const { binDir, tshBinPath } = getBinaryPaths();
  const { username } = os.userInfo();
  const hostname = os.hostname();
  const kubeConfigsDir = getKubeConfigsDir();
  // TODO(ravicious): Replace with app.getPath('logs'). We started storing logs under a custom path.
  // Before switching to the recommended path, we need to investigate the impact of this change.
  // https://www.electronjs.org/docs/latest/api/app#appgetpathname
  const logsDir = path.join(userDataDir, 'logs');
  const installationId = loadInstallationId(
    path.resolve(app.getPath('userData'), 'installation_id')
  );

  const tshd = {
    binaryPath: tshBinPath,
    homeDir: getTshHomeDir(),
    requestedNetworkAddress: tshAddress,
  };
  const sharedProcess = {
    requestedNetworkAddress: sharedAddress,
  };
  const tshdEvents = {
    requestedNetworkAddress: tshdEventsAddress,
  };

  // To start the app in dev mode, we run `electron path_to_main.js`. It means
  // that the app is run without package.json context, so it can not read the version
  // from it.
  // The way we run Electron can be changed (`electron .`), but it has one major
  // drawback - dev app and bundled app will use the same app data directory.
  //
  // A workaround is to read the version from `process.env.npm_package_version`.
  const appVersion = dev ? process.env.npm_package_version : app.getVersion();
  const availableShells = await getAvailableShells();

  return {
    dev,
    debug,
    insecure,
    tshd,
    sharedProcess,
    tshdEvents,
    userDataDir,
    sessionDataDir,
    tempDataDir,
    binDir,
    agentBinaryPath: path.resolve(sessionDataDir, 'teleport', 'teleport'),
    certsDir: getCertsDir(),
    availableShells,
    defaultOsShellId: getDefaultShell(availableShells),
    kubeConfigsDir,
    logsDir,
    platform: process.platform,
    installationId,
    arch: os.arch(),
    osVersion: os.release(),
    appVersion,
    isLocalBuild: appVersion === '1.0.0-dev',
    username,
    hostname,
  };
}

function getCertsDir() {
  const certsPath = path.resolve(app.getPath('userData'), 'certs');
  if (!fs.existsSync(certsPath)) {
    fs.mkdirSync(certsPath);
  }
  if (fs.readdirSync(certsPath)) {
    fs.rmSync(certsPath, { force: true, recursive: true });
    fs.mkdirSync(certsPath);
  }
  return certsPath;
}

function getKubeConfigsDir(): string {
  const kubeConfigsPath = path.resolve(app.getPath('userData'), 'kube');
  if (!fs.existsSync(kubeConfigsPath)) {
    fs.mkdirSync(kubeConfigsPath);
  }
  return kubeConfigsPath;
}

function getTshHomeDir() {
  const tshPath = path.resolve(app.getPath('userData'), 'tsh');
  if (!fs.existsSync(tshPath)) {
    fs.mkdirSync(tshPath);
  }
  return tshPath;
}

// binDir is used in the packaged version to add tsh to PATH.
// tshBinPath is used by Connect to call tsh directly.
function getBinaryPaths(): { binDir?: string; tshBinPath: string } {
  if (app.isPackaged) {
    const isWin = process.platform === 'win32';
    const isMac = process.platform === 'darwin';
    // On macOS, tsh lives within tsh.app:
    //
    //     Teleport Connect.app/Contents/MacOS/tsh.app/Contents/MacOS
    //
    // exe path is an absolute path to
    //
    //     Teleport Connect.app/Contents/MacOS/Teleport Connect
    const binDir = isMac
      ? path.join(app.getPath('exe'), '../tsh.app/Contents/MacOS')
      : path.join(RESOURCES_PATH, 'bin');
    const tshBinPath = path.join(binDir, isWin ? 'tsh.exe' : 'tsh');

    return { binDir, tshBinPath };
  }

  const tshBinPath = env[TSH_BIN_ENV_VAR] || TSH_BIN_DEFAULT_PATH_FOR_DEV;

  // Enforce absolute path. The current working directory of this script is not just `webapps` or
  // `webapps/packages/teleterm` as people would assume so we're going to save them the trouble of
  // figuring out that it's actually `webapps/packages/teleterm/build/app/dist/main`.
  if (!path.isAbsolute(tshBinPath)) {
    throw new Error(
      env[TSH_BIN_ENV_VAR]
        ? `${TSH_BIN_ENV_VAR} must be an absolute path, received ${tshBinPath}.`
        : `The default path to a tsh binary must be absolute, received ${tshBinPath}`
    );
  }

  if (!fs.existsSync(tshBinPath)) {
    throw new Error(
      env[TSH_BIN_ENV_VAR]
        ? `${TSH_BIN_ENV_VAR} must point at a tsh binary, could not find a tsh binary under ${tshBinPath}.`
        : `Could not find a tsh binary under the default location (${tshBinPath}).`
    );
  }

  return { tshBinPath };
}

export function getAssetPath(...paths: string[]): string {
  return path.join(RESOURCES_PATH, 'assets', ...paths);
}

/**
 * Describes what addresses the gRPC servers should attempt to obtain on app startup.
 */
function requestGrpcServerAddresses(): GrpcServerAddresses {
  switch (process.platform) {
    case 'win32': {
      return {
        tsh: 'tcp://localhost:0',
        shared: 'tcp://localhost:0',
        tshdEvents: 'tcp://localhost:0',
      };
    }
    case 'linux':
    case 'darwin':
      return {
        tsh: getUnixSocketNetworkAddress('tsh.socket'),
        shared: getUnixSocketNetworkAddress('shared.socket'),
        tshdEvents: getUnixSocketNetworkAddress('tshd_events.socket'),
      };
  }
}

function getUnixSocketNetworkAddress(socketName: string) {
  const unixSocketPath = path.resolve(app.getPath('userData'), socketName);

  // try to cleanup after previous process that unexpectedly crashed
  if (fs.existsSync(unixSocketPath)) {
    fs.unlinkSync(unixSocketPath);
  }

  return `unix://${path.resolve(app.getPath('userData'), socketName)}`;
}
