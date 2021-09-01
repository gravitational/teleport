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

const logger = Logger.create('TDPClient');

// Client is the TDP client. It is responsible for connecting to a websocket serving the tdp server,
// sending client commands, and recieving and processing server messages. In the case of recieving a
// png frame from the server, it will draw the frame to the supplied canvas.
// TODO: clipboard syncronization.
export default class Client extends EventEmitter {
  socketAddr: string;
  codec: Codec;
  socket: WebSocket;

  constructor(socketAddr: string) {
    super();
    this.socketAddr = socketAddr;
    this.codec = new Codec();
  }

  // Create the websocket and register event handlers.
  // Passes the username and the screens initial width and height of the screen,
  // and sends that data to the TDP server as required by the protocol.`
  connect(username: string, width: number, height: number) {
    try {
      this.socket = new WebSocket(this.socketAddr);

      this.socket.onopen = () => {
        logger.info('websocket is open');
        logger.info(`opening tdp connection with username ${username}`);
        this.socket.send(this.codec.encodeUsername(username));
        logger.info(
          `sending initial screen size of width = ${width}, height = ${height}`
        );
        this.resize(width, height);
      };
      this.socket.onmessage = (ev: MessageEvent) => {
        this.onMessage(ev);
      };
      this.socket.onclose = () => {
        this.emit('close');
        logger.info('websocket is closed');
      };
      this.socket.onerror = () => {
        this.handleError(new Error('websocket internal error'));
      };
    } catch (err) {
      this.handleError(err);
    }
  }

  onMessage(ev: MessageEvent) {
    this.codec
      .decodeMessageType(ev.data)
      .then(messageType => {
        if (messageType === MessageType.PNG_FRAME) {
          this.processFrame(ev.data);
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

  resize(w: number, h: number) {
    this.socket.send(this.codec.encodeScreenSpec(w, h));
  }

  handleError(err: Error) {
    this.emit('error', err);
    logger.error(err);
  }
}
