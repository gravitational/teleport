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

var Term = require('Terminal');
var Tty = require('./tty');
var TtyEvents = require('./ttyEvents');
var {debounce, isNumber} = require('_');

var cfg = require('app/config');
var api = require('app/services/api');
var logger = require('app/common/logger').create('terminal');
var $ = require('jQuery');

Term.colors[256] = '#252323';

const DISCONNECT_TXT = '\x1b[31mdisconnected\x1b[m\r\n';
const CONNECTED_TXT = 'Connected!\r\n';
const GRV_CLASS = 'grv-terminal';

class TtyTerminal {
  constructor(options){
    let {
      tty,
      cols,
      rows,
      scrollBack = 1000 } = options;

    this.ttyParams = tty;
    this.tty = new Tty();
    this.ttyEvents = new TtyEvents();

    this.scrollBack = scrollBack
    this.rows = rows;
    this.cols = cols;
    this.term = null;
    this._el = options.el;

    this.debouncedResize = debounce(this._requestResize.bind(this), 200);
  }

  open() {
    $(this._el).addClass(GRV_CLASS);

    this.term = new Term({
      cols: 15,
      rows: 5,
      scrollback: this.scrollback,
      useStyle: true,
      screenKeys: true,
      cursorBlink: true
    });

    this.term.open(this._el);

    this.resize(this.cols, this.rows);

    // term events
    this.tty.on('data', (data) => {
      console.info(data);
    });

    this.term.on('data', (data) => this.tty.send(data));



    // tty
    this.tty.on('resize', ({h, w})=> this.resize(w, h));
    this.tty.on('reset', ()=> this.term.reset());
    this.tty.on('open', ()=> this.term.write(CONNECTED_TXT));
    this.tty.on('close', ()=> this.term.write(DISCONNECT_TXT));
    this.tty.on('data', (data) => {
      try{
        this.term.write(data);
      }catch(err){
        console.error(err);
      }
    });

    // ttyEvents
    this.ttyEvents.on('data', this._handleTtyEventsData.bind(this));
    this.connect();
    window.addEventListener('resize', this.debouncedResize);
  }

  connect(){
    this.tty.connect(this._getTtyConnStr());
    this.ttyEvents.connect(this._getTtyEventsConnStr());
  }

  destroy() {
    if(this.tty !== null){
      this.tty.disconnect();
    }

    if(this.ttyEvents !== null){
      this.ttyEvents.disconnect();
      this.ttyEvents.removeAllListeners();
    }

    if(this.term !== null){
      this.term.destroy();
      this.term.removeAllListeners();
    }

    $(this._el).empty().removeClass(GRV_CLASS);

    window.removeEventListener('resize', this.debouncedResize);
  }

  resize(cols, rows) {
    // if not defined, use the size of the container
    if(!isNumber(cols) || !isNumber(rows)){
      let dim = this._getDimensions();
      cols = dim.cols;
      rows = dim.rows;
    }

    this.cols = cols;
    this.rows = rows;
    this.term.resize(this.cols, this.rows);
  }

  _requestResize(){
    let {cols, rows} = this._getDimensions();
    let w = cols;
    let h = rows;

    // some min values
    w = w < 5 ? 5 : w;
    h = h < 5 ? 5 : h;

    let {sid } = this.ttyParams;
    let reqData = { terminal_params: { w, h } };

    logger.info('resize', `w:${w} and h:${h}`);
    api.put(cfg.api.getTerminalSessionUrl(sid), reqData)
      .done(()=> logger.info('resized'))
      .fail((err)=> logger.error('failed to resize', err));
  }

  _handleTtyEventsData(data){
    if(data && data.terminal_params){
      let {w, h} = data.terminal_params;
      if(h !== this.rows || w !== this.cols){
        this.resize(w, h);
      }
    }
  }

  _getDimensions(){
    let $container = $(this._el);
    let fakeRow = $('<div><span>&nbsp;</span></div>');

    $container.find('.terminal').append(fakeRow);
    // get div height
    let fakeColHeight = fakeRow[0].getBoundingClientRect().height;
    // get span width
    let fakeColWidth = fakeRow.children().first()[0].getBoundingClientRect().width;

    let width = $container[0].clientWidth;
    let height = $container[0].clientHeight;

    let cols = Math.floor(width / (fakeColWidth));
    let rows = Math.floor(height / (fakeColHeight));
    fakeRow.remove();

    return {cols, rows};
  }

  _getTtyEventsConnStr(){
    let {sid, url, token } = this.ttyParams;
    return `${url}/sessions/${sid}/events/stream?access_token=${token}`;
  }

  _getTtyConnStr(){
    let {serverId, login, sid, url, token } = this.ttyParams;
    var params = {
      server_id: serverId,
      login,
      sid,
      term: {
        h: this.rows,
        w: this.cols
      }
    }

    var json = JSON.stringify(params);
    var jsonEncoded = window.encodeURI(json);

    return `${url}/connect?access_token=${token}&params=${jsonEncoded}`;
  }

}

module.exports = TtyTerminal;
