/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { readlink } from 'node:fs';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';
import { EventEmitter } from 'node:events';

import * as nodePTY from 'node-pty';
import which from 'which';

import Logger from 'teleterm/logger';

import { PtyProcessOptions, IPtyProcess } from './types';

type Status = 'open' | 'not_initialized' | 'terminated';

const pathEnvVar = process.platform === 'win32' ? 'Path' : 'PATH';

export class PtyProcess extends EventEmitter implements IPtyProcess {
  private _buffered = true;
  private _attachedBufferTimer;
  private _attachedBuffer: string;
  private _process: nodePTY.IPty;
  private _logger: Logger;
  private _status: Status = 'not_initialized';
  private _disposed = false;

  constructor(private options: PtyProcessOptions & { ptyId: string }) {
    super();
    this._logger = new Logger(
      `PtyProcess (id: ${options.ptyId} ${options.path} ${options.args.join(
        ' '
      )})`
    );
  }

  getPtyId() {
    return this.options.ptyId;
  }

  /**
   * start spawns a new PTY with the arguments given through the constructor.
   * It emits TermEventEnum.StartError on error. start itself always returns a fulfilled promise.
   */
  async start(cols: number, rows: number) {
    try {
      // which throws an error if the argument is not found in path.
      // TODO(ravicious): Remove the manual check for the existence of the executable after node-pty
      // makes its behavior consistent across platforms.
      // https://github.com/microsoft/node-pty/issues/689
      await which(this.options.path, { path: this.options.env[pathEnvVar] });

      // TODO(ravicious): Set argv0 when node-pty adds support for it.
      // https://github.com/microsoft/node-pty/issues/472
      this._process = nodePTY.spawn(this.options.path, this.options.args, {
        cols,
        rows,
        name: 'xterm-color',
        // HOME should be always defined. But just in case it isn't let's use the cwd from process.
        // https://unix.stackexchange.com/questions/123858
        cwd: this.options.cwd || getDefaultCwd(this.options.env),
        env: this.options.env,
        // Turn off ConPTY due to an uncaught exception being thrown when a PTY is closed.
        useConpty: false,
      });
    } catch (error) {
      this._logger.error(error);
      this.handleStartError(error);
      return;
    }

    this._setStatus('open');
    this.emit(TermEventEnum.Open);

    // Emit the init/help message before registering data handler. This ensures
    // the message is printed first and will not conflict with data coming from
    // the PTY.
    if (this.options.initMessage) {
      this.emit(TermEventEnum.Data, this.options.initMessage);
    }

    this._process.onData(data => this._handleData(data));
    this._process.onExit(ev => this._handleExit(ev));
  }

  write(data: string) {
    if (this._status !== 'open' || this._disposed) {
      this._logger.warn('pty is not started or has been terminated');
      return;
    }

    this._process.write(data);
  }

  resize(cols: number, rows: number) {
    if (this._status !== 'open' || this._disposed) {
      this._logger.warn('pty is not started or has been terminated');
      return;
    }

    this._process.resize(cols, rows);
  }

  async getCwd() {
    if (this._status !== 'open' || this._disposed) {
      return '';
    }

    try {
      return await getWorkingDirectory(this.getPid());
    } catch (err) {
      this._logger.error(
        `Unable to read directory for PID: ${this.getPid()}`,
        err
      );
    }
  }

  dispose() {
    this.removeAllListeners();
    this._process?.kill();
    this._disposed = true;
  }

  onData(cb: (data: string) => void) {
    return this.addListenerAndReturnRemovalFunction(TermEventEnum.Data, cb);
  }

  onOpen(cb: () => void) {
    return this.addListenerAndReturnRemovalFunction(TermEventEnum.Open, cb);
  }

  onExit(cb: (ev: { exitCode: number; signal?: number }) => void) {
    return this.addListenerAndReturnRemovalFunction(TermEventEnum.Exit, cb);
  }

  onStartError(cb: (message: string) => void) {
    return this.addListenerAndReturnRemovalFunction(
      TermEventEnum.StartError,
      cb
    );
  }

  private addListenerAndReturnRemovalFunction(
    eventName: TermEventEnum,
    listener: (...args: any[]) => void
  ) {
    this.addListener(eventName, listener);

    // The removal function is not used from within the shared process code, it is returned only to
    // comply with the IPtyProcess interface.
    return () => {
      this.removeListener(eventName, listener);
    };
  }

  private getPid() {
    return this._process?.pid;
  }

  private _flushBuffer() {
    this.emit(TermEventEnum.Data, this._attachedBuffer);
    this._attachedBuffer = null;
    clearTimeout(this._attachedBufferTimer);
    this._attachedBufferTimer = null;
  }

  private _pushToBuffer(data: string) {
    if (this._attachedBuffer) {
      this._attachedBuffer += data;
    } else {
      this._attachedBuffer = data;
      setTimeout(this._flushBuffer.bind(this), 10);
    }
  }

  private _handleExit(e: { exitCode: number; signal?: number }) {
    this.emit(TermEventEnum.Exit, e);
    this._logger.info(`pty has been terminated with exit code: ${e.exitCode}`);
    this._setStatus('terminated');
  }

  private _handleData(data: string) {
    try {
      if (this._buffered) {
        this._pushToBuffer(data);
      } else {
        this.emit(TermEventEnum.Data, data);
      }
    } catch (err) {
      this._logger.error('failed to parse incoming message.', err);
    }
  }

  private handleStartError(error: Error) {
    const command = `${this.options.path} ${this.options.args.join(' ')}`;
    this.emit(
      TermEventEnum.StartError,
      `Cannot execute ${command}: ${error.message}`
    );
  }

  private _setStatus(value: Status) {
    this._status = value;
    this._logger.info(`status -> ${value}`);
  }
}

export enum TermEventEnum {
  Close = 'terminal.close',
  Reset = 'terminal.reset',
  Data = 'terminal.data',
  Open = 'terminal.open',
  Exit = 'terminal.exit',
  StartError = 'terminal.start_error',
}

async function getWorkingDirectory(pid: number): Promise<string> {
  switch (process.platform) {
    case 'darwin':
      const asyncExec = promisify(exec);
      // -a: join using AND instead of OR for the -p and -d options
      // -p: PID
      // -d: only include the file descriptor, cwd
      // -F: fields to output (the n character outputs 3 things, the last one is cwd)
      const { stdout } = await asyncExec(`lsof -a -p ${pid} -d cwd -F n`);
      return stdout.split('\n').filter(Boolean).reverse()[0].substring(1);
    case 'linux':
      const asyncReadlink = promisify(readlink);
      return await asyncReadlink(`/proc/${pid}/cwd`);
    case 'win32':
      return undefined;
  }
}

function getDefaultCwd(env: Record<string, string>): string {
  const userDir = process.platform === 'win32' ? env.USERPROFILE : env.HOME;

  return userDir || process.cwd();
}
