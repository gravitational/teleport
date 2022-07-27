import fs from 'fs';
import os from 'os';
import path from 'path';

import { app } from 'electron';

import Logger from 'teleterm/logger';

import { ChildProcessAddresses, RuntimeSettings } from './types';

const { argv, env } = process;

const RESOURCES_PATH = app.isPackaged
  ? process.resourcesPath
  : path.join(__dirname, '../../../../');

const dev = env.NODE_ENV === 'development' || env.DEBUG_PROD === 'true';

// Allows running tsh in insecure mode (development)
const isInsecure = dev || argv.includes('--insecure');

function getRuntimeSettings(): RuntimeSettings {
  const userDataDir = app.getPath('userData');
  const { tsh: tshAddress, shared: sharedAddress } =
    requestChildProcessesAddresses();
  const binDir = getBinDir();
  const tshd = {
    insecure: isInsecure,
    binaryPath: getTshBinaryPath(),
    homeDir: getTshHomeDir(),
    requestedNetworkAddress: tshAddress,
    flags: [
      'daemon',
      'start',
      // grpc-js requires us to pass localhost:port for TCP connections,
      // for tshd we have to specify the protocol as well.
      `--addr=${tshAddress}`,
      `--certs-dir=${getCertsDir()}`,
    ],
  };
  const sharedProcess = {
    requestedNetworkAddress: sharedAddress,
  };

  if (isInsecure) {
    tshd.flags.unshift('--debug');
    tshd.flags.unshift('--insecure');
  }

  return {
    dev,
    tshd,
    sharedProcess,
    userDataDir,
    binDir,
    certsDir: getCertsDir(),
    defaultShell: getDefaultShell(),
    platform: process.platform,
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

function getTshHomeDir() {
  const tshPath = path.resolve(app.getPath('userData'), 'tsh');
  if (!fs.existsSync(tshPath)) {
    fs.mkdirSync(tshPath);
  }
  return tshPath;
}

function getTshBinaryPath() {
  if (app.isPackaged) {
    return path.join(
      getBinDir(),
      process.platform === 'win32' ? 'tsh.exe' : 'tsh'
    );
  }

  const tshPath = env.TELETERM_TSH_PATH;
  if (!tshPath) {
    throw Error('tsh path is not defined');
  }

  return tshPath;
}

function getBinDir() {
  if (!app.isPackaged) {
    return;
  }

  return path.join(RESOURCES_PATH, 'bin');
}

function getAssetPath(...paths: string[]): string {
  return path.join(RESOURCES_PATH, 'assets', ...paths);
}

function getDefaultShell(): string {
  const logger = new Logger();
  switch (process.platform) {
    case 'linux':
    case 'darwin': {
      const fallbackShell = 'bash';
      const { shell } = os.userInfo();

      if (!shell) {
        logger.error(
          `Failed to read ${process.platform} platform default shell, using fallback: ${fallbackShell}.\n`
        );

        return fallbackShell;
      }

      return shell;
    }
    case 'win32':
      return 'powershell.exe';
  }
}

function requestChildProcessesAddresses(): ChildProcessAddresses {
  switch (process.platform) {
    case 'win32': {
      return {
        tsh: 'localhost:0',
        shared: 'localhost:0',
      };
    }
    case 'linux':
    case 'darwin':
      return {
        tsh: getUnixSocketNetworkAddress('tsh.socket'),
        shared: getUnixSocketNetworkAddress('shared.socket'),
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

export { getRuntimeSettings, getAssetPath };
