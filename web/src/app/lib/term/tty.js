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

import { EventEmitter } from 'events';
import { StatusCodeEnum } from './ttyEnums';

const defaultOptions = {
  buffered: true
}

class Tty extends EventEmitter {

  _buffered = true;
  _attachSocketBufferTimer;

  constructor(props = {}) {
    let options = {
      ...defaultOptions,
      ...props
    }

    super();  
    
    this.socket = null;
    this._buffered = options.buffered; 
    this._onOpenConnection = this._onOpenConnection.bind(this);
    this._onCloseConnection = this._onCloseConnection.bind(this);
    this._onReceiveData = this._onReceiveData.bind(this);
  }

  disconnect(reasonCode = StatusCodeEnum.NORMAL){
    this.socket.close(reasonCode);
  }

  reconnect(options){
    this.disconnect();
    this.socket.onopen = null;
    this.socket.onmessage = null;
    this.socket.onclose = null;
    this.socket = null;
    this.connect(options);
  }

  connect(connStr) {
    this.socket = new WebSocket(connStr);
    this.socket.onopen = this._onOpenConnection;
    this.socket.onmessage = this._onReceiveData;        
    this.socket.onclose = this._onCloseConnection;
  }
  
  send(data){
    this.socket.send(data);
  }

  _flushBuffer() {    
    this.emit('data', this._attachSocketBuffer);      
    this._attachSocketBuffer = null;
    clearTimeout(this._attachSocketBufferTimer);
    this._attachSocketBufferTimer = null;    
  }

  _pushToBuffer(data) {
    if (this._attachSocketBuffer) {
      this._attachSocketBuffer += data;
    } else {
      this._attachSocketBuffer = data;
      setTimeout(this._flushBuffer.bind(this), 10);
    }
  }

  _onOpenConnection() {
    this.emit('open');
  }

  _onCloseConnection(e) {
    this.emit('close', e);      
  }

  _onReceiveData(ev) {
    if (this._buffered) {
      this._pushToBuffer(ev.data);
    } else {
      this.emit('data', ev.data);            
    }
  }
}

export default Tty;