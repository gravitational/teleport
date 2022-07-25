import fs from 'fs';
import os from 'os';
import path from 'path';

import { app } from 'electron';

import Logger from 'teleterm/logger';

import { RuntimeSettings } from './types';

const { argv, env } = process;

const RESOURCES_PATH = app.isPackaged
  ? process.resourcesPath
  : path.join(__dirname, '../../../../');

const dev = env.NODE_ENV === 'development' || env.DEBUG_PROD === 'true';

// Allows running tsh in insecure mode (development)
const isInsecure = dev || argv.includes('--insecure');

function getRuntimeSettings(): RuntimeSettings {
  const userDataDir = app.getPath('userData');
  const binDir = getBinDir();
  const tshNetworkAddr = getTshNetworkAddress();
  const tshd = {
    insecure: isInsecure,
    binaryPath: getTshBinaryPath(),
    homeDir: getTshHomeDir(),
    networkAddr: tshNetworkAddr,
    flags: ['daemon', 'start', `--addr=${tshNetworkAddr}`],
  };
  const sharedProcess = {
    networkAddr: getSharedProcessNetworkAddress(),
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
    defaultShell: getDefaultShell(),
    platform: process.platform,
  };
}

function getTshNetworkAddress() {
  return getUnixSocketNetworkAddress('tsh.socket');
}

function getSharedProcessNetworkAddress() {
  return getUnixSocketNetworkAddress('shared.socket');
}

function getUnixSocketNetworkAddress(socketName: string) {
  const unixSocketPath = path.resolve(app.getPath('userData'), socketName);

  // try to cleanup after previous process that unexpectedly crashed
  if (fs.existsSync(unixSocketPath)) {
    fs.unlinkSync(unixSocketPath);
  }

  return `unix://${path.resolve(app.getPath('userData'), socketName)}`;
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
    return path.join(getBinDir(), 'tsh');
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

export { getRuntimeSettings, getAssetPath };
