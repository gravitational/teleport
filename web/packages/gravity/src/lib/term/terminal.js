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
import XTerm from 'xterm/dist/xterm';
import 'xterm/dist/xterm.css';
import Tty from './tty';
import {debounce, isInteger} from 'lodash';
import Logger from 'shared/libs/logger';
import { TermEventEnum } from './enums';

const logger = Logger.create('lib/term/terminal');
const DISCONNECT_TXT = 'disconnected';
const GRV_CLASS = 'grv-terminal';
const WINDOW_RESIZE_DEBOUNCE_DELAY = 200;

/**
 * TtyTerminal is a wrapper on top of xtermjs that handles connections
 * and resize events
 */
class TtyTerminal {

  constructor(options){
    const { addressResolver, el, scrollBack = 1000 } = options;
    this._el = el;
    this.tty = new Tty(addressResolver);
    this.scrollBack = scrollBack
    this.rows = undefined;
    this.cols = undefined;
    this.term = null;
    this.debouncedResize = debounce(
      this._requestResize.bind(this),
      WINDOW_RESIZE_DEBOUNCE_DELAY
    );
  }

  open() {
    this._el.classList.add(GRV_CLASS);

    // render xtermjs with default values
    this.term = new XTerm({
      cols: 15,
      rows: 5,
      scrollback: this.scrollBack,
      cursorBlink: false
    });

    this.term.open(this._el);

    // fit xterm to available space
    this.resize(this.cols, this.rows);

    // subscribe to xtermjs output
    this.term.on('data', data => {
      this.tty.send(data)
    })

    // subscribe to window resize events
    window.addEventListener('resize', this.debouncedResize);

    // subscribe to tty
    this.tty.on(TermEventEnum.RESET, this.reset.bind(this));
    this.tty.on(TermEventEnum.CONN_CLOSE, this._processClose.bind(this));
    this.tty.on(TermEventEnum.DATA, this._processData.bind(this));

    // subscribe tty resize event (used by session player)
    this.tty.on(TermEventEnum.RESIZE, ({h, w}) => this.resize(w, h));

    this.connect();
  }

  connect(){
    this.tty.connect(this.cols, this.rows);
  }

  destroy() {
    window.removeEventListener('resize', this.debouncedResize);
    this._disconnect();
    if(this.term !== null){
      this.term.destroy();
      this.term.removeAllListeners();
    }

    this._el.innerHTML = null;
    this._el.classList.remove(GRV_CLASS);
  }

  reset() {
    this.term.reset()
  }

  resize(cols, rows) {
    try {
      // if not defined, use the size of the container
      if(!isInteger(cols) || !isInteger(rows)){
        const dim = this._getDimensions();
        cols = dim.cols;
        rows = dim.rows;
      }

      if(cols === this.cols && rows === this.rows){
        return;
      }

      this.cols = cols;
      this.rows = rows;
      this.term.resize(cols, rows);
    } catch (err) {
      logger.error('xterm.resize', { w: cols, h: rows }, err);
      this.term.reset();
    }
  }

  _processData(data){
    try {
      this.term.write(data);
    } catch (err) {
      logger.error('xterm.write', data, err);
      // recover xtermjs by resetting it
      this.term.reset();
    }
  }

  _processClose(e) {
    const { reason } = e;
    let displayText = DISCONNECT_TXT;
    if (reason) {
      displayText = `${displayText}: ${reason}`;
    }

    displayText = `\x1b[31m${displayText}\x1b[m\r\n`;
    this.term.write(displayText)
  }

  _disconnect() {
    this.tty.disconnect();
    this.tty.removeAllListeners();
  }

  _requestResize(){
    const { cols, rows } = this._getDimensions();
    if (!isInteger(cols) || !isInteger(rows)) {
      logger.info(`unable to calculate terminal dimensions (container might be hidden) ${cols}:${rows}`);
      return;
    }

    // ensure min size
    const w = cols < 5 ? 5 : cols;
    const h = rows < 5 ? 5 : rows;

    this.resize(w, h);
    this.tty.requestResize(w, h);
  }

  _getDimensions(){
    const $terminal = this._el.querySelector('.terminal');

    const $fakeRow = document.createElement('div');
    $fakeRow.innerHTML = `<span>&nbsp;</span>`;
    $terminal.appendChild($fakeRow);

    const fakeColHeight = $fakeRow.getBoundingClientRect().height;
    const fakeColWidth = $fakeRow.firstElementChild.getBoundingClientRect().width;
    const width = this._el.clientWidth;
    const height = this._el.clientHeight;
    const cols = Math.floor(width / (fakeColWidth));
    const rows = Math.floor(height / (fakeColHeight));

    $terminal.removeChild($fakeRow);
    return { cols, rows };
  }
}

export default TtyTerminal;