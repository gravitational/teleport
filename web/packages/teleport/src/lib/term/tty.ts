/*
Copyright 2019-2021 Gravitational, Inc.

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

import Logger from 'shared/libs/logger';
import { EventEmitterWebAuthnSender } from 'teleport/lib/EventEmitterWebAuthnSender';
import { WebauthnAssertionResponse } from 'teleport/services/auth';
import { EventTypeEnum, TermEventEnum, StatusCodeEnum } from './enums';
import { Protobuf, MessageTypeEnum } from './protobuf';

const logger = Logger.create('Tty');

const defaultOptions = {
  buffered: true,
};

class Tty extends EventEmitterWebAuthnSender {
  socket = null;

  _buffered = true;
  _attachSocketBufferTimer;
  _attachSocketBuffer: string;
  _addressResolver = null;
  _proto = new Protobuf();

  constructor(addressResolver, props = {}) {
    super();
    const options = {
      ...defaultOptions,
      ...props,
    };

    this._addressResolver = addressResolver;
    this._buffered = options.buffered;
    this._onOpenConnection = this._onOpenConnection.bind(this);
    this._onCloseConnection = this._onCloseConnection.bind(this);
    this._onMessage = this._onMessage.bind(this);
  }

  disconnect(reasonCode = StatusCodeEnum.NORMAL) {
    if (this.socket !== null) {
      this.socket.close(reasonCode);
    }
  }

  connect(w: number, h: number) {
    const connStr = this._addressResolver.getConnStr(w, h);
    this.socket = new WebSocket(connStr);
    this.socket.binaryType = 'arraybuffer';
    this.socket.onopen = this._onOpenConnection;
    this.socket.onmessage = this._onMessage;
    this.socket.onclose = this._onCloseConnection;
  }

  send(data) {
    if (!this.socket || !data) {
      return;
    }

    const msg = this._proto.encodeRawMessage(data);
    const bytearray = new Uint8Array(msg);
    this.socket.send(bytearray.buffer);
  }

  sendWebAuthn(data: WebauthnAssertionResponse) {
    this.send(JSON.stringify(data));
  }

  // part of the flow control
  pauseFlow() {}

  // part of the flow control
  resumeFlow() {}

  requestResize(w: number, h: number) {
    if (!this.socket) {
      return;
    }

    logger.info('requesting new screen size', `w:${w} and h:${h}`);
    var data = JSON.stringify({
      event: EventTypeEnum.RESIZE,
      width: w,
      height: h,
      size: `${w}:${h}`,
    });

    var encoded = this._proto.encodeResizeMessage(data);
    var bytearray = new Uint8Array(encoded);
    this.socket.send(bytearray.buffer);
  }

  _flushBuffer() {
    this.emit(TermEventEnum.DATA, this._attachSocketBuffer);
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
    this.emit(TermEventEnum.CONN_CLOSE, e);
    logger.info('websocket is closed');
  }

  _onMessage(ev) {
    try {
      const uintArray = new Uint8Array(ev.data);
      const msg = this._proto.decode(uintArray);
      switch (msg.type) {
        case MessageTypeEnum.U2F_CHALLENGE:
          this.emit(TermEventEnum.U2F_CHALLENGE, msg.payload);
          break;
        case MessageTypeEnum.WEBAUTHN_CHALLENGE:
          this.emit(TermEventEnum.WEBAUTHN_CHALLENGE, msg.payload);
          break;
        case MessageTypeEnum.AUDIT:
          this._processAuditPayload(msg.payload);
          break;
        case MessageTypeEnum.SESSION_END:
          this.emit(TermEventEnum.CLOSE, msg.payload);
          break;
        case MessageTypeEnum.RAW:
          if (this._buffered) {
            this._pushToBuffer(msg.payload);
          } else {
            this.emit(TermEventEnum.DATA, msg.payload);
          }
          break;
        default:
          throw Error(`unknown message type: ${msg.type}`);
      }
    } catch (err) {
      logger.error('failed to parse incoming message.', err);
    }
  }

  _processAuditPayload(payload) {
    const event = JSON.parse(payload);
    if (event.event === EventTypeEnum.RESIZE) {
      let [w, h] = event.size.split(':');
      w = Number(w);
      h = Number(h);
      this.emit(TermEventEnum.RESIZE, { w, h });
    }
  }
}

export default Tty;
