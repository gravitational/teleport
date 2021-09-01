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
import Codec, { MessageType } from './codec';
import Logger from 'shared/libs/logger';

const logger = Logger.create('TDPClient');

// Client is the TDP client. It is responsible for connecting to a websocket serving the tdp server,
// sending client commands, and recieving and processing server messages. In the case of recieving a
// png frame from the server, it will draw the frame to the supplied canvas.
// TODO: clipboard syncronization.
export default class Client {
  canvas: HTMLCanvasElement;
  socketAddr: string;
  username: string;
  codec: Codec;
  socket: WebSocket;

  constructor(socketAddr: string, username: string) {
    this.socketAddr = socketAddr;
    this.username = username;
    this.codec = new Codec();
  }

  // Pass the canvas to draw to on connection
  connect(canvas: HTMLCanvasElement) {
    try {
      this.canvas = canvas;
      this.socket = new WebSocket(this.socketAddr);

      this.socket.onopen = () => {
        this.onOpen();
      };
      this.socket.onmessage = (ev: MessageEvent) => {
        this.onMessage(ev);
      };
      this.socket.onclose = () => {
        logger.info('websocket is closed');
      };
      this.socket.onerror = () => {
        this.logAndThrowError('websocket internal error');
      };
    } catch (err) {
      this.logAndThrowError('error connecting to websocket', err);
    }
  }

  onOpen() {
    logger.info('websocket is open');
    logger.info(`initial canvas width: ${this.canvas.width}`);
    logger.info(`initial canvas height: ${this.canvas.height}`);
    this.sendUsername(this.username);
    this.sendScreenSpec(this.canvas.width, this.canvas.height);
  }

  onMessage(ev: MessageEvent) {
    this.codec
      .decodeMessageType(ev.data)
      .then(messageType => {
        if (messageType === MessageType.PNG_FRAME) {
          this.drawFrame(ev.data);
        } else {
          this.logAndThrowError(
            `recieved unsupported message type: ${messageType}`
          );
        }
      })
      .catch(err => {
        this.logAndThrowError('failed to decode incoming message', err);
      });
  }

  // Assuming we have a message of type PNG_FRAME, extract its bounds
  // and draw the image to the canvas.
  drawFrame(blob: Blob) {
    Promise.all([this.codec.decodeRegion(blob), this.codec.decodePng(blob)])
      .then(values => {
        const { left, top } = values[0];
        const bitmap = values[1];
        this.canvas.getContext('2d').drawImage(bitmap, left, top);
      })
      .catch(err => {
        this.logAndThrowError('failed to draw frame', err);
      });
  }

  sendScreenSpec(w: number, h: number) {
    this.socket.send(this.codec.encodeScreenSpec(w, h));
  }

  sendUsername(username: string) {
    this.socket.send(this.codec.encodeUsername(username));
  }

  logAndThrowError(msg: string, err?: any) {
    logger.error(msg, err);
    if (err) {
      throw err;
    } else {
      throw new Error(msg);
    }
  }
}
