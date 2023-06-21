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
import Logger from 'shared/libs/logger';

import init, {
  init_wasm_log,
  FastPathProcessor,
} from 'teleport/ironrdp/pkg/ironrdp';

import { WebsocketCloseCode, TermEvent } from 'teleport/lib/term/enums';
import { EventEmitterWebAuthnSender } from 'teleport/lib/EventEmitterWebAuthnSender';

import Codec, {
  MessageType,
  FileType,
  SharedDirectoryErrCode,
  Severity,
} from './codec';
import {
  PathDoesNotExistError,
  SharedDirectoryManager,
} from './sharedDirectoryManager';

import type { FileOrDirInfo } from './sharedDirectoryManager';
import type {
  MouseButton,
  ButtonState,
  ScrollAxis,
  ClientScreenSpec,
  PngFrame,
  ClipboardData,
  SharedDirectoryInfoResponse,
  SharedDirectoryListResponse,
  SharedDirectoryMoveResponse,
  SharedDirectoryReadResponse,
  SharedDirectoryWriteResponse,
  SharedDirectoryCreateResponse,
  SharedDirectoryDeleteResponse,
  FileSystemObject,
} from './codec';
import type { WebauthnAssertionResponse } from 'teleport/services/auth';

export enum TdpClientEvent {
  TDP_CLIENT_SCREEN_SPEC = 'tdp client screen spec',
  TDP_PNG_FRAME = 'tdp png frame',
  TDP_BMP_FRAME = 'tdp bmp frame',
  TDP_CLIPBOARD_DATA = 'tdp clipboard data',
  // TDP_ERROR corresponds with the TDP error message
  TDP_ERROR = 'tdp error',
  // CLIENT_ERROR represents an error event in the client that isn't a TDP_ERROR
  CLIENT_ERROR = 'client error',
  // TDP_WARNING corresponds the TDP warning message
  TDP_WARNING = 'tdp warning',
  // CLIENT_WARNING represents a warning event that isn't a TDP_WARNING
  CLIENT_WARNING = 'client warning',
  WS_OPEN = 'ws open',
  WS_CLOSE = 'ws close',
}

export enum LogType {
  OFF = 'OFF',
  ERROR = 'ERROR',
  WARN = 'WARN',
  INFO = 'INFO',
  DEBUG = 'DEBUG',
  TRACE = 'TRACE',
}

// Client is the TDP client. It is responsible for connecting to a websocket serving the tdp server,
// sending client commands, and recieving and processing server messages. Its creator is responsible for
// ensuring the websocket gets closed and all of its event listeners cleaned up when it is no longer in use.
// For convenience, this can be done in one fell swoop by calling Client.shutdown().
export default class Client extends EventEmitterWebAuthnSender {
  protected codec: Codec;
  protected socket: WebSocket | undefined;
  private socketAddr: string;
  private sdManager: SharedDirectoryManager;
  private fastPathProcessor: FastPathProcessor;

  private logger = Logger.create('TDPClient');

  constructor(socketAddr: string, width: number, height: number) {
    super();
    this.socketAddr = socketAddr;
    this.codec = new Codec();
    this.sdManager = new SharedDirectoryManager();

    // select the wasm log level
    let wasmLogLevel = LogType.OFF;
    if (process.env.NODE_ENV === 'development') {
      wasmLogLevel = LogType.TRACE;
    }

    // init initializes the wasm module into memory
    init().then(() => {
      init_wasm_log(wasmLogLevel);
      this.fastPathProcessor = new FastPathProcessor(width, height);
    });
  }

  // Connect to the websocket and register websocket event handlers.
  init() {
    this.socket = new WebSocket(this.socketAddr);
    this.socket.binaryType = 'arraybuffer';

    this.socket.onopen = () => {
      this.logger.info('websocket is open');
      this.emit(TdpClientEvent.WS_OPEN);
    };

    this.socket.onmessage = async (ev: MessageEvent) => {
      await this.processMessage(ev.data as ArrayBuffer);
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

  // processMessage should be await-ed when called,
  // so that its internal await-or-not logic is obeyed.
  async processMessage(buffer: ArrayBuffer): Promise<void> {
    try {
      const messageType = this.codec.decodeMessageType(buffer);
      switch (messageType) {
        case MessageType.PNG_FRAME:
          this.handlePngFrame(buffer);
          break;
        case MessageType.PNG2_FRAME:
          this.handlePng2Frame(buffer);
          break;
        case MessageType.REMOTE_FX_FRAME:
          this.handleRDPFastPathPDU(buffer);
          break;
        case MessageType.CLIENT_SCREEN_SPEC:
          this.handleClientScreenSpec(buffer);
          break;
        case MessageType.MOUSE_BUTTON:
          this.handleMouseButton(buffer);
          break;
        case MessageType.MOUSE_MOVE:
          this.handleMouseMove(buffer);
          break;
        case MessageType.CLIPBOARD_DATA:
          this.handleClipboardData(buffer);
          break;
        case MessageType.ERROR:
          this.handleError(
            new Error(this.codec.decodeErrorMessage(buffer)),
            TdpClientEvent.TDP_ERROR
          );
          break;
        case MessageType.NOTIFICATION:
          this.handleTdpNotification(buffer);
          break;
        case MessageType.MFA_JSON:
          this.handleMfaChallenge(buffer);
          break;
        case MessageType.SHARED_DIRECTORY_ACKNOWLEDGE:
          this.handleSharedDirectoryAcknowledge(buffer);
          break;
        case MessageType.SHARED_DIRECTORY_INFO_REQUEST:
          this.handleSharedDirectoryInfoRequest(buffer);
          break;
        case MessageType.SHARED_DIRECTORY_CREATE_REQUEST:
          // A typical sequence is that we receive a SharedDirectoryCreateRequest
          // immediately followed by a SharedDirectoryWriteRequest. It's important
          // that we await here so that this client doesn't field the SharedDirectoryWriteRequest
          // until the create has successfully completed, or else we might get an error
          // trying to write to a file that hasn't been created yet.
          await this.handleSharedDirectoryCreateRequest(buffer);
          break;
        case MessageType.SHARED_DIRECTORY_DELETE_REQUEST:
          this.handleSharedDirectoryDeleteRequest(buffer);
          break;
        case MessageType.SHARED_DIRECTORY_READ_REQUEST:
          this.handleSharedDirectoryReadRequest(buffer);
          break;
        case MessageType.SHARED_DIRECTORY_WRITE_REQUEST:
          this.handleSharedDirectoryWriteRequest(buffer);
          break;
        case MessageType.SHARED_DIRECTORY_MOVE_REQUEST:
          this.handleSharedDirectoryMoveRequest(buffer);
          break;
        case MessageType.SHARED_DIRECTORY_LIST_REQUEST:
          this.handleSharedDirectoryListRequest(buffer);
          break;
        default:
          this.logger.warn(`received unsupported message type ${messageType}`);
      }
    } catch (err) {
      this.handleError(err, TdpClientEvent.CLIENT_ERROR);
    }
  }

  handleClientScreenSpec(buffer: ArrayBuffer) {
    this.logger.warn(
      `received unsupported message type ${this.codec.decodeMessageType(
        buffer
      )}`
    );
  }

  handleMouseButton(buffer: ArrayBuffer) {
    this.logger.warn(
      `received unsupported message type ${this.codec.decodeMessageType(
        buffer
      )}`
    );
  }

  handleMouseMove(buffer: ArrayBuffer) {
    this.logger.warn(
      `received unsupported message type ${this.codec.decodeMessageType(
        buffer
      )}`
    );
  }

  handleClipboardData(buffer: ArrayBuffer) {
    this.emit(
      TdpClientEvent.TDP_CLIPBOARD_DATA,
      this.codec.decodeClipboardData(buffer)
    );
  }

  handleTdpNotification(buffer: ArrayBuffer) {
    const notification = this.codec.decodeNotification(buffer);
    if (notification.severity === Severity.Error) {
      this.handleError(
        new Error(notification.message),
        TdpClientEvent.TDP_ERROR
      );
    } else if (notification.severity === Severity.Warning) {
      this.handleWarning(notification.message, TdpClientEvent.TDP_WARNING);
    }
  }

  // Assuming we have a message of type PNG_FRAME, extract its
  // bounds and png bitmap and emit a render event.
  handlePngFrame(buffer: ArrayBuffer) {
    this.codec.decodePngFrame(buffer, (pngFrame: PngFrame) =>
      this.emit(TdpClientEvent.TDP_PNG_FRAME, pngFrame)
    );
  }

  handlePng2Frame(buffer: ArrayBuffer) {
    this.codec.decodePng2Frame(buffer, (pngFrame: PngFrame) =>
      this.emit(TdpClientEvent.TDP_PNG_FRAME, pngFrame)
    );
  }

  handleRDPFastPathPDU(buffer: ArrayBuffer) {
    let rdpFastPathPDU = this.codec.decodeRDPFastPathPDU(buffer);

    this.fastPathProcessor.process(
      rdpFastPathPDU,
      this,
      (bmpFrame: BitmapFrame) => {
        this.emit(TdpClientEvent.TDP_BMP_FRAME, bmpFrame);
      },
      (responseFrame: ArrayBuffer) => {
        this.sendRDPResponsePDU(responseFrame);
      }
    );
  }

  handleMfaChallenge(buffer: ArrayBuffer) {
    try {
      const mfaJson = this.codec.decodeMfaJson(buffer);
      if (mfaJson.mfaType == 'n') {
        this.emit(TermEvent.WEBAUTHN_CHALLENGE, mfaJson.jsonString);
      } else {
        // mfaJson.mfaType === 'u', or else decodeMfaJson would have thrown an error.
        this.handleError(
          new Error(
            'Multifactor authentication is required for accessing this desktop, \
      however the U2F API for hardware keys is not supported for desktop sessions. \
      Please notify your system administrator to update cluster settings \
      to use WebAuthn as the second factor protocol.'
          ),
          TdpClientEvent.CLIENT_ERROR
        );
      }
    } catch (err) {
      this.handleError(err, TdpClientEvent.CLIENT_ERROR);
    }
  }

  private wasSuccessful(errCode: SharedDirectoryErrCode) {
    if (errCode === SharedDirectoryErrCode.Nil) {
      return true;
    }

    this.handleError(
      new Error(`Encountered shared directory error: ${errCode}`),
      TdpClientEvent.CLIENT_ERROR
    );
    return false;
  }

  handleSharedDirectoryAcknowledge(buffer: ArrayBuffer) {
    const ack = this.codec.decodeSharedDirectoryAcknowledge(buffer);

    if (!this.wasSuccessful(ack.errCode)) {
      return;
    }
    try {
      this.logger.info(
        'Started sharing directory: ' + this.sdManager.getName()
      );
    } catch (e) {
      this.handleError(e, TdpClientEvent.CLIENT_ERROR);
    }
  }

  async handleSharedDirectoryInfoRequest(buffer: ArrayBuffer) {
    const req = this.codec.decodeSharedDirectoryInfoRequest(buffer);
    const path = req.path;
    try {
      const info = await this.sdManager.getInfo(path);
      this.sendSharedDirectoryInfoResponse({
        completionId: req.completionId,
        errCode: SharedDirectoryErrCode.Nil,
        fso: this.toFso(info),
      });
    } catch (e) {
      if (e.constructor === PathDoesNotExistError) {
        this.sendSharedDirectoryInfoResponse({
          completionId: req.completionId,
          errCode: SharedDirectoryErrCode.DoesNotExist,
          fso: {
            lastModified: BigInt(0),
            fileType: FileType.File,
            size: BigInt(0),
            isEmpty: true,
            path: path,
          },
        });
      } else {
        this.handleError(e, TdpClientEvent.CLIENT_ERROR);
      }
    }
  }

  async handleSharedDirectoryCreateRequest(buffer: ArrayBuffer) {
    const req = this.codec.decodeSharedDirectoryCreateRequest(buffer);

    try {
      await this.sdManager.create(req.path, req.fileType);
      const info = await this.sdManager.getInfo(req.path);
      this.sendSharedDirectoryCreateResponse({
        completionId: req.completionId,
        errCode: SharedDirectoryErrCode.Nil,
        fso: this.toFso(info),
      });
    } catch (e) {
      this.sendSharedDirectoryCreateResponse({
        completionId: req.completionId,
        errCode: SharedDirectoryErrCode.Failed,
        fso: {
          lastModified: BigInt(0),
          fileType: FileType.File,
          size: BigInt(0),
          isEmpty: true,
          path: req.path,
        },
      });
      this.handleWarning(e.message, TdpClientEvent.CLIENT_WARNING);
    }
  }

  async handleSharedDirectoryDeleteRequest(buffer: ArrayBuffer) {
    const req = this.codec.decodeSharedDirectoryDeleteRequest(buffer);

    try {
      await this.sdManager.delete(req.path);
      this.sendSharedDirectoryDeleteResponse({
        completionId: req.completionId,
        errCode: SharedDirectoryErrCode.Nil,
      });
    } catch (e) {
      this.sendSharedDirectoryDeleteResponse({
        completionId: req.completionId,
        errCode: SharedDirectoryErrCode.Failed,
      });
      this.handleWarning(e.message, TdpClientEvent.CLIENT_WARNING);
    }
  }

  async handleSharedDirectoryReadRequest(buffer: ArrayBuffer) {
    const req = this.codec.decodeSharedDirectoryReadRequest(buffer);
    try {
      const readData = await this.sdManager.readFile(
        req.path,
        req.offset,
        req.length
      );
      this.sendSharedDirectoryReadResponse({
        completionId: req.completionId,
        errCode: SharedDirectoryErrCode.Nil,
        readDataLength: readData.length,
        readData,
      });
    } catch (e) {
      this.handleError(e, TdpClientEvent.CLIENT_ERROR);
    }
  }

  async handleSharedDirectoryWriteRequest(buffer: ArrayBuffer) {
    const req = this.codec.decodeSharedDirectoryWriteRequest(buffer);
    try {
      const bytesWritten = await this.sdManager.writeFile(
        req.path,
        req.offset,
        req.writeData
      );

      this.sendSharedDirectoryWriteResponse({
        completionId: req.completionId,
        errCode: SharedDirectoryErrCode.Nil,
        bytesWritten,
      });
    } catch (e) {
      this.handleError(e, TdpClientEvent.CLIENT_ERROR);
    }
  }

  handleSharedDirectoryMoveRequest(buffer: ArrayBuffer) {
    const req = this.codec.decodeSharedDirectoryMoveRequest(buffer);
    // Always send back Failed for now, see https://github.com/gravitational/webapps/issues/1064
    this.sendSharedDirectoryMoveResponse({
      completionId: req.completionId,
      errCode: SharedDirectoryErrCode.Failed,
    });
    this.handleWarning(
      'Moving files and directories within a shared \
        directory is not supported.',
      TdpClientEvent.CLIENT_WARNING
    );
  }

  async handleSharedDirectoryListRequest(buffer: ArrayBuffer) {
    try {
      const req = this.codec.decodeSharedDirectoryListRequest(buffer);
      const path = req.path;

      const infoList: FileOrDirInfo[] = await this.sdManager.listContents(path);
      const fsoList: FileSystemObject[] = infoList.map(info =>
        this.toFso(info)
      );

      this.sendSharedDirectoryListResponse({
        completionId: req.completionId,
        errCode: SharedDirectoryErrCode.Nil,
        fsoList,
      });
    } catch (e) {
      this.handleError(e, TdpClientEvent.CLIENT_ERROR);
    }
  }

  private toFso(info: FileOrDirInfo): FileSystemObject {
    return {
      lastModified: BigInt(info.lastModified),
      fileType: info.kind === 'file' ? FileType.File : FileType.Directory,
      size: BigInt(info.size),
      isEmpty: info.isEmpty,
      path: info.path,
    };
  }

  protected send(
    data: string | ArrayBufferLike | Blob | ArrayBufferView
  ): void {
    if (this.socket && this.socket.readyState === 1) {
      try {
        this.socket.send(data);
      } catch (e) {
        this.handleError(e, TdpClientEvent.CLIENT_ERROR);
      }
      return;
    }

    this.handleError(
      new Error('websocket unavailable'),
      TdpClientEvent.CLIENT_ERROR
    );
  }

  sendUsername(username: string) {
    this.send(this.codec.encodeUsername(username));
  }

  sendMouseMove(x: number, y: number) {
    this.send(this.codec.encodeMouseMove(x, y));
  }

  sendMouseButton(button: MouseButton, state: ButtonState) {
    this.send(this.codec.encodeMouseButton(button, state));
  }

  sendMouseWheelScroll(axis: ScrollAxis, delta: number) {
    this.send(this.codec.encodeMouseWheelScroll(axis, delta));
  }

  sendKeyboardInput(code: string, state: ButtonState) {
    // Only send message if key is recognized, otherwise do nothing.
    const msg = this.codec.encodeKeyboardInput(code, state);
    if (msg) this.send(msg);
  }

  sendClipboardData(clipboardData: ClipboardData) {
    this.send(this.codec.encodeClipboardData(clipboardData));
  }

  sendWebAuthn(data: WebauthnAssertionResponse) {
    const msg = this.codec.encodeMfaJson({
      mfaType: 'n',
      jsonString: JSON.stringify(data),
    });
    this.send(msg);
  }

  addSharedDirectory(sharedDirectory: FileSystemDirectoryHandle) {
    try {
      this.sdManager.add(sharedDirectory);
    } catch (err) {
      this.handleError(err, TdpClientEvent.CLIENT_ERROR);
    }
  }

  sendSharedDirectoryAnnounce() {
    let name: string;
    try {
      name = this.sdManager.getName();
      this.send(
        this.codec.encodeSharedDirectoryAnnounce({
          discard: 0, // This is always the first request.
          // Hardcode directoryId for now since we only support sharing 1 directory.
          // We're using 2 because the smartcard device is hardcoded to 1 in the backend.
          directoryId: 2,
          name,
        })
      );
    } catch (e) {
      this.handleError(e, TdpClientEvent.CLIENT_ERROR);
    }
  }

  sendSharedDirectoryInfoResponse(res: SharedDirectoryInfoResponse) {
    this.send(this.codec.encodeSharedDirectoryInfoResponse(res));
  }

  sendSharedDirectoryListResponse(res: SharedDirectoryListResponse) {
    this.send(this.codec.encodeSharedDirectoryListResponse(res));
  }

  sendSharedDirectoryMoveResponse(res: SharedDirectoryMoveResponse) {
    this.send(this.codec.encodeSharedDirectoryMoveResponse(res));
  }

  sendSharedDirectoryReadResponse(response: SharedDirectoryReadResponse) {
    this.send(this.codec.encodeSharedDirectoryReadResponse(response));
  }

  sendSharedDirectoryWriteResponse(response: SharedDirectoryWriteResponse) {
    this.send(this.codec.encodeSharedDirectoryWriteResponse(response));
  }

  sendSharedDirectoryCreateResponse(response: SharedDirectoryCreateResponse) {
    this.send(this.codec.encodeSharedDirectoryCreateResponse(response));
  }

  sendSharedDirectoryDeleteResponse(response: SharedDirectoryDeleteResponse) {
    this.send(this.codec.encodeSharedDirectoryDeleteResponse(response));
  }

  resize(spec: ClientScreenSpec) {
    this.send(this.codec.encodeClientScreenSpec(spec));
  }

  sendRDPResponsePDU(responseFrame: ArrayBuffer) {
    this.send(this.codec.encodeRDPResponsePDU(responseFrame));
  }

  // Emits an errType event, closing the socket if the error was fatal.
  private handleError(
    err: Error,
    errType: TdpClientEvent.TDP_ERROR | TdpClientEvent.CLIENT_ERROR
  ) {
    this.logger.error(err);
    this.emit(errType, err);
    this.socket?.close();
  }

  // Emits an warnType event
  private handleWarning(
    warning: string,
    warnType: TdpClientEvent.TDP_WARNING | TdpClientEvent.CLIENT_WARNING
  ) {
    this.logger.warn(warning);
    this.emit(warnType, warning);
  }

  // Ensures full cleanup of this object.
  // Note that it removes all listeners first and then cleans up the socket,
  // so don't call this if your calling object is relying on listeners.
  // It's safe to call this multiple times, calls subsequent to the first call
  // will simply do nothing.
  shutdown(closeCode = WebsocketCloseCode.NORMAL) {
    this.removeAllListeners();
    this.socket?.close(closeCode);
  }
}

// Mimics the BitmapFrame struct in rust.
export type BitmapFrame = {
  top: number;
  left: number;
  image_data: ImageData;
};
