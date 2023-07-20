/*
Copyright 2019-2022 Gravitational, Inc.

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
import 'xterm/css/xterm.css';
import { ITheme, Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { debounce, isInteger } from 'shared/utils/highbar';
import { WebLinksAddon } from 'xterm-addon-web-links';
import Logger from 'shared/libs/logger';

import cfg from 'teleport/config';

import { TermEvent } from './enums';
import Tty from './tty';

import type { DebouncedFunc } from 'shared/utils/highbar';

const logger = Logger.create('lib/term/terminal');
const DISCONNECT_TXT = 'disconnected';
const WINDOW_RESIZE_DEBOUNCE_DELAY = 200;

/**
 * TtyTerminal is a wrapper on top of xtermjs
 */
export default class TtyTerminal {
  term: Terminal;
  tty: Tty;

  _el: HTMLElement;
  _scrollBack: number;
  _fontFamily: string;
  _fontSize: number;
  _debouncedResize: DebouncedFunc<() => void>;
  _fitAddon = new FitAddon();
  _webLinksAddon = new WebLinksAddon();

  constructor(
    tty: Tty,
    private options: Options
  ) {
    const { el, scrollBack, fontFamily, fontSize } = options;
    this._el = el;
    this._fontFamily = fontFamily || undefined;
    this._fontSize = fontSize || 14;
    // Passing scrollback will overwrite the default config. This is to support ttyplayer.
    // Default to the config when not passed anything, which is the normal usecase
    this._scrollBack = scrollBack || cfg.ui.scrollbackLines;
    this.tty = tty;
    this.term = null;

    this._debouncedResize = debounce(() => {
      this._requestResize();
    }, WINDOW_RESIZE_DEBOUNCE_DELAY);
  }

  open() {
    this.term = new Terminal({
      lineHeight: 1,
      fontFamily: this._fontFamily,
      fontSize: this._fontSize,
      scrollback: this._scrollBack,
      cursorBlink: false,
      minimumContrastRatio: 4.5, // minimum for WCAG AA compliance
      theme: this.options.theme,
    });

    this.term.loadAddon(this._fitAddon);
    this.term.loadAddon(this._webLinksAddon);
    this.term.open(this._el);
    this._fitAddon.fit();
    this.term.focus();
    this.term.onData(data => {
      this.tty.send(data);
    });

    this.tty.on(TermEvent.RESET, () => this.reset());
    this.tty.on(TermEvent.CONN_CLOSE, e => this._processClose(e));
    this.tty.on(TermEvent.DATA, data => this._processData(data));

    // subscribe tty resize event (used by session player)
    this.tty.on(TermEvent.RESIZE, ({ h, w }) => this.resize(w, h));

    this.connect();

    // subscribe to window resize events
    window.addEventListener('resize', this._debouncedResize);
  }

  connect() {
    this.tty.connect(this.term.cols, this.term.rows);
  }

  updateTheme(theme: ITheme): void {
    this.term.options.theme = theme;
  }

  destroy() {
    this._disconnect();
    this._debouncedResize.cancel();
    this._fitAddon.dispose();
    this._el.innerHTML = null;
    this.term?.dispose();

    window.removeEventListener('resize', this._debouncedResize);
  }

  reset() {
    this.term.reset();
  }

  resize(cols, rows) {
    try {
      // if not defined, use the size of the container
      if (!isInteger(cols) || !isInteger(rows)) {
        cols = this.term.cols;
        rows = this.term.rows;
      }

      if (cols === this.term.cols && rows === this.term.rows) {
        return;
      }

      this.term.resize(cols, rows);
    } catch (err) {
      logger.error('xterm.resize', { w: cols, h: rows }, err);
      this.term.reset();
    }
  }

  _disconnect() {
    this.tty.disconnect();
    this.tty.removeAllListeners();
  }

  _requestResize() {
    const visible = !!this._el.clientWidth && !!this._el.clientHeight;
    if (!visible) {
      logger.info(`unable to resize terminal (container might be hidden)`);
      return;
    }

    this._fitAddon.fit();
    this.tty.requestResize(this.term.cols, this.term.rows);
  }

  _processData(data) {
    try {
      this.tty.pauseFlow();
      this.term.write(data, () => this.tty.resumeFlow());
    } catch (err) {
      logger.error('xterm.write', data, err);
      // recover xtermjs by resetting it
      this.term.reset();
      this.tty.resumeFlow();
    }
  }

  _processClose(e) {
    const { reason } = e;
    let displayText = DISCONNECT_TXT;
    if (reason) {
      displayText = `${displayText}: ${reason}`;
    }

    displayText = `\x1b[31m${displayText}\x1b[m\r\n`;
    this.term.write(displayText);
  }
}

type Options = {
  el: HTMLElement;
  theme: ITheme;
  scrollBack?: number;
  fontFamily?: string;
  fontSize?: number;
};
