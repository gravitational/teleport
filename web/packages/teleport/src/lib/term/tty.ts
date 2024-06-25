/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import Logger from 'shared/libs/logger';

import { EventEmitterWebAuthnSender } from 'teleport/lib/EventEmitterWebAuthnSender';
import { WebauthnAssertionResponse } from 'teleport/services/auth';
import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';

import { EventType, TermEvent, WebsocketCloseCode } from './enums';
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
  _pendingUploads = {};

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

  disconnect(closeCode = WebsocketCloseCode.NORMAL) {
    if (this.socket !== null) {
      this.socket.close(closeCode);
    }
  }

  connect(w: number, h: number) {
    const connStr = this._addressResolver.getConnStr(w, h);
    this.socket = new AuthenticatedWebSocket(connStr);
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
    const encoded = this._proto.encodeChallengeResponse(JSON.stringify(data));
    const bytearray = new Uint8Array(encoded);
    this.socket.send(bytearray);
  }

  sendKubeExecData(data: KubeExecData) {
    const encoded = this._proto.encodeKubeExecData(JSON.stringify(data));
    const bytearray = new Uint8Array(encoded);
    this.socket.send(bytearray);
  }

  _sendFileTransferRequest(message: string) {
    const encoded = this._proto.encodeFileTransferRequest(message);
    const bytearray = new Uint8Array(encoded);
    this.socket.send(bytearray);
  }

  sendFileDownloadRequest(location: string) {
    const message = JSON.stringify({
      event: EventType.FILE_TRANSFER_REQUEST,
      download: true,
      location,
    });
    this._sendFileTransferRequest(message);
  }

  sendFileUploadRequest(location: string, file: File) {
    const locationAndName = location + file.name;
    this._pendingUploads[locationAndName] = file;
    const message = JSON.stringify({
      event: EventType.FILE_TRANSFER_REQUEST,
      download: false,
      location,
      filename: file.name,
    });
    this._sendFileTransferRequest(message);
  }

  approveFileTransferRequest(requestId: string, approved: boolean) {
    const message = JSON.stringify({
      event: EventType.FILE_TRANSFER_DECISION,
      requestId,
      approved,
    });
    const encoded = this._proto.encodeFileTransferDecision(message);
    const bytearray = new Uint8Array(encoded);
    this.socket.send(bytearray);
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
      event: EventType.RESIZE,
      width: w,
      height: h,
      size: `${w}:${h}`,
    });

    var encoded = this._proto.encodeResizeMessage(data);
    var bytearray = new Uint8Array(encoded);
    this.socket.send(bytearray.buffer);
  }

  _flushBuffer() {
    this.emit(TermEvent.DATA, this._attachSocketBuffer);
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
    this.emit(TermEvent.CONN_CLOSE, e);
    logger.info('websocket is closed');
  }

  _onMessage(ev) {
    try {
      const uintArray = new Uint8Array(ev.data);
      const msg = this._proto.decode(uintArray);

      switch (msg.type) {
        case MessageTypeEnum.WEBAUTHN_CHALLENGE:
          this.emit(TermEvent.WEBAUTHN_CHALLENGE, msg.payload);
          break;
        case MessageTypeEnum.AUDIT:
          this._processAuditPayload(msg.payload);
          break;
        case MessageTypeEnum.SESSION_DATA:
          this.emit(TermEvent.SESSION, msg.payload);
          break;
        case MessageTypeEnum.SESSION_END:
          this.emit(TermEvent.CLOSE, msg.payload);
          break;
        case MessageTypeEnum.RAW:
          if (this._buffered) {
            this._pushToBuffer(msg.payload);
          } else {
            this.emit(TermEvent.DATA, msg.payload);
          }
          break;
        case MessageTypeEnum.ERROR:
          this.emit(TermEvent.DATA, msg.payload + '\n');
          break;
        case MessageTypeEnum.LATENCY:
          this.emit(TermEvent.LATENCY, msg.payload);
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
    // received a new/updated file transfer request
    if (event.event === EventType.FILE_TRANSFER_REQUEST) {
      this.emit(EventType.FILE_TRANSFER_REQUEST, event);
    }

    // received a file transfer approval
    if (event.event === EventType.FILE_TRANSFER_REQUEST_APPROVE) {
      const isDownload = event.download === true;
      let pendingFile: File = null;
      // if the approval is for an upload, fetch the file pending upload
      if (!isDownload) {
        const locationAndName = event.location + event.filename;
        pendingFile = this._getPendingFile(locationAndName);
        // cleanup if file exists. It's ok if it doesn't exist, we check thaat in the handler
        if (pendingFile) {
          delete this._pendingUploads[locationAndName];
        }
      }
      this.emit(EventType.FILE_TRANSFER_REQUEST_APPROVE, event, pendingFile);
    }

    // received a file transfer denial
    if (event.event === EventType.FILE_TRANSFER_REQUEST_DENY) {
      const locationAndName = event.location + event.filename;
      delete this._pendingUploads[locationAndName];
      this.emit(EventType.FILE_TRANSFER_REQUEST_DENY, event);
    }

    // received a window resize
    if (event.event === EventType.RESIZE) {
      let [w, h] = event.size.split(':');
      w = Number(w);
      h = Number(h);
      this.emit(TermEvent.RESIZE, { w, h });
    }
  }

  _getPendingFile(location: string) {
    return this._pendingUploads[location];
  }
}

export type KubeExecData = {
  kubeCluster: string;
  namespace: string;
  pod: string;
  container: string;
  command: string;
  isInteractive: boolean;
};

export default Tty;
