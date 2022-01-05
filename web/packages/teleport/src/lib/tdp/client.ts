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
import Codec, {
  MessageType,
  MouseButton,
  ButtonState,
  ScrollAxis,
} from './codec';
import Logger from 'shared/libs/logger';

export enum TdpClientEvent {
  IMAGE_FRAGMENT = 'imgfrag',
  TDP_ERROR = 'tdperr',
  WS_OPEN = 'wsopen',
  WS_CLOSE = 'wsclose',
}

// Client is the TDP client. It is responsible for connecting to a websocket serving the tdp server,
// sending client commands, and recieving and processing server messages. It's listener is responsible for
// calling Client.nuke() (typically after Client emits a TdpClientEvent.DISCONNECT or TdpClientEvent.ERROR event) in order to clean
// up its websocket listeners.
export default class Client extends EventEmitter {
  codec: Codec;
  socket: WebSocket;
  socketAddr: string;
  username: string;
  logger = Logger.create('TDPClient');

  constructor(socketAddr: string, username: string) {
    super();
    this.socketAddr = socketAddr;
    this.codec = new Codec();
    this.username = username;
  }

  // Connect to the websocket and register websocket event handlers.
  init() {
    this.socket = new WebSocket(this.socketAddr);
    this.socket.binaryType = 'arraybuffer';

    this.socket.onopen = () => {
      this.logger.info('websocket is open');
      this.emit(TdpClientEvent.WS_OPEN);
    };

    this.socket.onmessage = (ev: MessageEvent) => {
      this.processMessage(ev.data as ArrayBuffer);
    };

    // The socket 'error' event will only ever be emitted by the socket
    // prior to a socket 'close' event (https://stackoverflow.com/a/40084550/6277051).
    // Therefore, we can rely on our onclose handler to account for any websocket errors.
    this.socket.onerror = null;
    this.socket.onclose = () => {
      this.logger.info('websocket is closed');

      // Clean up all of our socket's listeners and the socket itself.
      this.socket.onopen = null;
      this.socket.onmessage = null;
      this.socket.onclose = null;
      this.socket = null;

      this.emit(TdpClientEvent.WS_CLOSE);
    };
  }

  processMessage(buffer: ArrayBuffer) {
    const messageType = this.codec.decodeMessageType(buffer);
    try {
      if (messageType === MessageType.PNG_FRAME) {
        this.processFrame(buffer);
      } else if (messageType === MessageType.ERROR) {
        this.handleError(new Error(this.codec.decodeErrorMessage(buffer)));
      } else {
        this.handleError(
          new Error(`recieved unsupported message type ${messageType}`)
        );
      }
    } catch (err) {
      this.handleError(err);
    }
  }

  // Assuming we have a message of type PNG_FRAME, extract its
  // bounds and png bitmap and emit a render event.
  processFrame(buffer: ArrayBuffer) {
    const { left, top } = this.codec.decodeRegion(buffer);
    const image = new Image();
    image.onload = () =>
      this.emit(TdpClientEvent.IMAGE_FRAGMENT, { image, left, top });
    image.src = this.codec.decodePng(buffer);
  }

  sendUsername(username: string) {
    this.socket?.send(this.codec.encodeUsername(username));
  }

  sendMouseMove(x: number, y: number) {
    this.socket.send(this.codec.encodeMouseMove(x, y));
  }

  sendMouseButton(button: MouseButton, state: ButtonState) {
    this.socket.send(this.codec.encodeMouseButton(button, state));
  }

  sendMouseWheelScroll(axis: ScrollAxis, delta: number) {
    this.socket.send(this.codec.encodeMouseWheelScroll(axis, delta));
  }

  sendKeyboardInput(code: string, state: ButtonState) {
    // Only send message if key is recognized, otherwise do nothing.
    const msg = this.codec.encodeKeyboardInput(code, state);
    if (msg) this.socket.send(msg);
  }

  resize(w: number, h: number) {
    this.socket?.send(this.codec.encodeScreenSpec(w, h));
  }

  // Emits an TdpClientEvent.ERROR event. Sets this.errored to true to alert the socket.onclose handler that
  // it needn't emit a generic unknown error event.
  handleError(err: Error) {
    this.logger.error(err);
    this.emit(TdpClientEvent.TDP_ERROR, err);
    this.socket?.close();
  }

  // Ensures full cleanup of this object.
  // Note that it removes all listeners first and then cleans up the socket,
  // so don't call this if your calling object is relying on listeners.
  nuke() {
    this.removeAllListeners();
    this.socket?.close();
  }
}

export type ImageFragment = {
  image: HTMLImageElement;
  left: number;
  top: number;
};
