/*
Copyright 2015 Gravitational, Inc.

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
import Tty from './tty';
import {debounce, isNumber} from 'lodash';
import Logger from 'app/lib/logger';
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
      if(!isNumber(cols) || !isNumber(rows)){
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
    // ensure min size
    const w = cols < 5 ? 5 : cols;
    const h = rows < 5 ? 5 : rows;

    this.resize(w, h);
    this.tty.requestResize(w, h);    
  }

  _getDimensions(){    
    const parentElementStyle = window.getComputedStyle(this.term.element.parentElement);
    const parentElementHeight = parseInt(parentElementStyle.getPropertyValue('height'));
    const parentElementWidth = Math.max(0, parseInt(parentElementStyle.getPropertyValue('width')) /*- 17*/);
    const elementStyle = window.getComputedStyle(this.term.element);
    const elementPaddingVer = parseInt(elementStyle.getPropertyValue('padding-top')) + parseInt(elementStyle.getPropertyValue('padding-bottom'));
    const elementPaddingHor = parseInt(elementStyle.getPropertyValue('padding-right')) + parseInt(elementStyle.getPropertyValue('padding-left'));
    const availableHeight = parentElementHeight - elementPaddingVer;
    const availableWidth = parentElementWidth - elementPaddingHor;    
    const subjectRow = this.term.rowContainer.firstElementChild;
    const contentBuffer = subjectRow.innerHTML;
        
    subjectRow.style.display = 'inline';
    // common character for measuring width, although on monospace
    subjectRow.innerHTML = 'W'; 
    
    const characterWidth = subjectRow.getBoundingClientRect().width;
    // revert style before calculating height, since they differ.
    subjectRow.style.display = ''; 
    
    const characterHeight = parseInt(subjectRow.offsetHeight);
    subjectRow.innerHTML = contentBuffer;

    const rows = parseInt(availableHeight / characterHeight);
    const cols = parseInt(availableWidth / characterWidth);
    return { cols, rows };
  }  
}

export default TtyTerminal;