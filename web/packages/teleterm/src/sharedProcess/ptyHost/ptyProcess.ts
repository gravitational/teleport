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

import * as nodePTY from 'node-pty';
import { readlink } from 'fs';
import { promisify } from 'util';
import { exec } from 'child_process';
import { EventEmitter } from 'events';
import { PtyProcessOptions, IPtyProcess } from './types';
import Logger from 'teleterm/logger';

type Status = 'open' | 'not_initialized' | 'terminated';

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
      `PtyProcess (id: ${options.ptyId} ${options.path} ${options.args})`
    );
  }

  start(cols: number, rows: number) {
    this._process = nodePTY.spawn(this.options.path, this.options.args, {
      cols,
      rows,
      name: 'xterm-color',
      // HOME should be always defined. But just in case it isn't let's use the cwd from process.
      // https://unix.stackexchange.com/questions/123858
      cwd: this.options.cwd || this.options.env['HOME'] || process.cwd(),
      env: this.options.env,
    });

    this._setStatus('open');
    this.emit(TermEventEnum.OPEN);

    this._process.onData(data => this._handleData(data));
    this._process.onExit(ev => this._handleExit(ev));

    if (this.options.initCommand) {
      this._process.write(this.options.initCommand + '\r');
    }
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
    this.addListener(TermEventEnum.DATA, cb);
  }

  onOpen(cb: () => void) {
    this.addListener(TermEventEnum.OPEN, cb);
  }

  onExit(cb: (ev: { exitCode: number; signal?: number }) => void) {
    this.addListener(TermEventEnum.EXIT, cb);
  }

  private getPid() {
    return this._process?.pid;
  }

  private _flushBuffer() {
    this.emit(TermEventEnum.DATA, this._attachedBuffer);
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
    this.emit(TermEventEnum.EXIT, e);
    this._logger.info(`pty has been terminated with exit code: ${e.exitCode}`);
    this._setStatus('terminated');
  }

  private _handleData(data: string) {
    try {
      if (this._buffered) {
        this._pushToBuffer(data);
      } else {
        this.emit(TermEventEnum.DATA, data);
      }
    } catch (err) {
      this._logger.error('failed to parse incoming message.', err);
    }
  }

  private _setStatus(value: Status) {
    this._status = value;
    this._logger.info(`status -> ${value}`);
  }
}

export const TermEventEnum = {
  CLOSE: 'terminal.close',
  RESET: 'terminal.reset',
  DATA: 'terminal.data',
  OPEN: 'terminal.open',
  EXIT: 'terminal.exit',
};

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
