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

var EventEmitter = require('events').EventEmitter;
var session = require('app/services/session');
var cfg = require('app/config');
var {actions} = require('app/modules/currentSession/');
var Buffer = require('buffer/').Buffer;
var {isNumber} = require('_');

const logger = require('app/common/logger').create('Tty');

class Tty extends EventEmitter {

  constructor({serverId, login, sid, rows, cols }){
    super();
    this.options = { serverId, login, sid, rows, cols };
    this.socket = null;
  }

  disconnect(){
    this.socket.close();
  }

  reconnect(options){
    this.disconnect();
    this.socket.onopen = null;
    this.socket.onmessage = null;
    this.socket.onclose = null;

    this.connect(options);
  }

  connect(options){
    this.options = { ...this.options, ...options};

    logger.info('connect', options);

    let {token} = session.getUserData();
    let connStr = cfg.api.getTtyConnStr({token, ...this.options});

    this.socket = new WebSocket(connStr, 'proto');

    this.socket.onopen = () => {
      this.emit('open');
    }

    this.socket.onmessage = (e)=>{
      let data = new Buffer(e.data, 'base64').toString('utf8');
      this.emit('data', data);
    }

    this.socket.onclose = ()=>{
      this.emit('close');
    }
  }

  resize(cols, rows){
    if(isNumber(cols) && isNumber(rows) && cols > 0 && rows > 0){
      actions.resize(cols, rows);
    }else{
      logger.error('invalid resize parameters', {cols, rows});
    }
  }

  send(data){
    this.socket.send(data);
  }
}

module.exports = Tty;
