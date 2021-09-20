// Copyright 2021 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
import { EventEmitter } from 'events';
import Codec, { MessageType } from './codec';
import Logger from 'shared/libs/logger';

// Client is the TDP client. It is responsible for connecting to a websocket serving the tdp server,
// sending client commands, and recieving and processing server messages.
export default class Client extends EventEmitter {
  codec: Codec;
  socket: WebSocket;
  socketAddr: string;
  username: string;
  logger = Logger.create('TDPClient');
  userDisconnected = false;

  constructor(socketAddr: string, username: string) {
    super();
    this.socketAddr = socketAddr;
    this.codec = new Codec();
    this.username = username;
  }

  // Connect to the websocket and register websocket event handlers.
  init() {
    this.socket = new WebSocket(this.socketAddr);

    this.socket.onopen = () => {
      this.logger.info('websocket is open');
      this.emit('init');
    };

    this.socket.onmessage = (ev: MessageEvent) => {
      this.processMessage(ev.data);
    };

    // The 'error' event will only ever be emitted by the socket
    // prior to a 'close' event (https://stackoverflow.com/a/40084550/6277051).
    // Therefore, we can rely on our onclose handler to account for any websocket errors.
    this.socket.onerror = null;
    this.socket.onclose = () => {
      this.logger.info('websocket is closed');

      // Clean up all of our socket's listeners and the socket itself.
      this.socket.onopen = null;
      this.socket.onmessage = null;
      this.socket.onclose = null;
      this.socket = null;

      if (this.userDisconnected) {
        this.emit('disconnect');
      } else {
        this.handleError(new Error('websocket connection failed'));
      }
    };
  }

  // After websocket is connected, caller can initialize the tdp connection by calling connect.
  connect(initialWidth: number, initialHeight: number) {
    this.sendUsername(this.username);
    this.resize(initialWidth, initialHeight);
    this.emit('connect');
  }

  processMessage(blob: Blob) {
    this.codec
      .decodeMessageType(blob)
      .then(messageType => {
        if (messageType === MessageType.PNG_FRAME) {
          this.processFrame(blob);
        } else {
          this.handleError(
            new Error(`recieved unsupported message type ${messageType}`)
          );
        }
      })
      .catch(err => {
        this.handleError(err);
      });
  }

  // Assuming we have a message of type PNG_FRAME, extract its
  // bounds and png bitmap and emit a render event.
  processFrame(blob: Blob) {
    Promise.all([this.codec.decodeRegion(blob), this.codec.decodePng(blob)])
      .then(values => {
        const { left, top } = values[0];
        const bitmap = values[1];
        this.emit('render', { bitmap, left, top });
      })
      .catch(err => {
        this.handleError(err);
      });
  }

  sendUsername(username: string) {
    this.socket?.send(this.codec.encodeUsername(username));
  }

  resize(w: number, h: number) {
    this.socket?.send(this.codec.encodeScreenSpec(w, h));
  }

  // Called to cleanup websocket when the connection is intentionally
  // closed by the end user (customer). Causes 'disconnect' event to be emitted.
  disconnect() {
    this.userDisconnected = true;
    this.socket?.close();
  }

  // Ensures full cleanup of this object.
  // Note that it removes all listeners first and then cleans up the socket,
  // so don't call this if your calling object is relying on listeners.
  nuke() {
    this.removeAllListeners();
    this.socket?.close();
  }

  handleError(err: Error) {
    this.emit('error', err);
    this.logger.error(err);
  }
}

export type RenderData = {
  bitmap: ImageBitmap;
  left: number;
  top: number;
};
