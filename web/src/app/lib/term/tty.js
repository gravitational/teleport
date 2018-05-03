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

import BufferModule from 'buffer/';
import { EventEmitter } from 'events';
import Logger from './../logger';
import { EventTypeEnum, StatusCodeEnum } from './enums';

const Buffer = BufferModule.Buffer;

const logger = Logger.create('Tty');

const defaultOptions = {
  buffered: true
}

class Tty extends EventEmitter {

  socket = null;

  _buffered = true;
  _attachSocketBufferTimer;
  _addressResolver = null;

  constructor(addressResolver, props = {}) {    
    super();  
    const options = {
      ...defaultOptions,
      ...props
    }
    
    this._addressResolver = addressResolver;        
    this._buffered = options.buffered; 
    this._onOpenConnection = this._onOpenConnection.bind(this);
    this._onCloseConnection = this._onCloseConnection.bind(this);
    this._onReceiveData = this._onReceiveData.bind(this);
  }

  disconnect(reasonCode = StatusCodeEnum.NORMAL) {
    if (this.socket !== null) {
      this.socket.close(reasonCode);
    }  
  }
  
  connect(w, h) {
    const connStr = this._addressResolver.getConnStr(w, h);
    this.socket = new WebSocket(connStr);
    this.socket.onopen = this._onOpenConnection;
    this.socket.onmessage = this._onReceiveData;        
    this.socket.onclose = this._onCloseConnection;
  }
  
  send(data) {    
    const msg = {
      type: "raw",
      payload: Buffer(data, 'utf8').toString('base64')
    }

    this.socket.send(JSON.stringify(msg));
  }

  requestResize(w, h){                        
    const msg = {
      type: "resize.request",
      payload: {
        event: EventTypeEnum.RESIZE,
        width: w,
        height: h,
        size: `${w}:${h}`
      }
    }
    
    this.socket.send(JSON.stringify(msg));    
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
    logger.info('websocket is open');
  }

  _onCloseConnection(e) {
    this.socket.onopen = null;
    this.socket.onmessage = null;
    this.socket.onclose = null;
    this.socket = null;
    this.emit('close', e);      
    logger.info('websocket is closed');
  }

  _onReceiveData(ev) {
    let msg = JSON.parse(ev.data);
    //console.log("EVENT: ", msg)    
    if (msg.type === 'audit') {
      this._processEvent(msg.payload);
      return;
    }
    
    let data = Buffer(msg.payload, 'base64').toString('utf8');
    if (this._buffered) {
      this._pushToBuffer(data);
    } else {
      this.emit('data', data);            
    }
  }

  _processEvent(event) {
    if (event.event === EventTypeEnum.RESIZE) {
      let [w, h] = event.size.split(':');
      w = Number(w);
      h = Number(h);
      this.emit('audit.resize', { w, h });
      return;
    }

    if (event.event === EventTypeEnum.END) {
      this.emit('audit.end', event);
      return;
    }
  }      
}

export default Tty;