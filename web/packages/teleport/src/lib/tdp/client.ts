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

  constructor(socketAddr: string, username: string) {
    super();
    this.socketAddr = socketAddr;
    this.username = username;
    this.codec = new Codec();
  }

  // Create the websocket and register event handlers.
  connect() {
    try {
      this.socket = new WebSocket(this.socketAddr);

      this.socket.onopen = () => {
        this.logger.info('websocket is open');
        this.emit('open');
      };
      this.socket.onmessage = (ev: MessageEvent) => {
        this.processMessage(ev.data);
      };
      this.socket.onerror = () => {
        this.handleError(new Error('websocket internal error'));
      };
      this.socket.onclose = () => {
        this.logger.info('websocket is closed');

        // Clean up all of our socket's listeners and the socket itself.
        this.socket.onopen = null;
        this.socket.onmessage = null;
        this.socket.onerror = null;
        this.socket.onclose = null;
        this.socket = null;

        // Emit close event so Client's owner knows to remove all the listeners belonging to Client itself.
        this.emit('close');
      };
    } catch (err) {
      this.handleError(err);
    }
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

  sendUsername() {
    this.socket.send(this.codec.encodeUsername(this.username));
  }

  resize(w: number, h: number) {
    this.socket.send(this.codec.encodeScreenSpec(w, h));
  }

  handleError(err: Error) {
    this.emit('error', err);
    this.logger.error(err);
  }
}
