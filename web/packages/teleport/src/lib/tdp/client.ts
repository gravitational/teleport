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

import { useEffect } from 'react';

import Logger from 'shared/libs/logger';

import init, {
  FastPathProcessor,
  init_wasm_log,
} from 'teleport/ironrdp/pkg/ironrdp';
import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';
import { EventEmitterMfaSender } from 'teleport/lib/EventEmitterMfaSender';
import { TermEvent, WebsocketCloseCode } from 'teleport/lib/term/enums';
import { MfaChallengeResponse } from 'teleport/services/mfa';

import Codec, {
  FileType,
  MessageType,
  PointerData,
  Severity,
  SharedDirectoryErrCode,
  type ButtonState,
  type ClientScreenSpec,
  type ClipboardData,
  type FileSystemObject,
  type MouseButton,
  type PngFrame,
  type ScrollAxis,
  type SharedDirectoryCreateResponse,
  type SharedDirectoryDeleteResponse,
  type SharedDirectoryInfoResponse,
  type SharedDirectoryListResponse,
  type SharedDirectoryMoveResponse,
  type SharedDirectoryReadResponse,
  type SharedDirectoryTruncateResponse,
  type SharedDirectoryWriteResponse,
  type SyncKeys,
} from './codec';
import {
  PathDoesNotExistError,
  SharedDirectoryManager,
  type FileOrDirInfo,
} from './sharedDirectoryManager';

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
  // TDP_INFO corresponds with the TDP info message
  TDP_INFO = 'tdp info',
  WS_OPEN = 'ws open',
  WS_CLOSE = 'ws close',
  RESET = 'reset',
  POINTER = 'pointer',
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
// sending client commands, and receiving and processing server messages. Its creator is responsible for
// ensuring the websocket gets closed and all of its event listeners cleaned up when it is no longer in use.
// For convenience, this can be done in one fell swoop by calling Client.shutdown().
export default class Client extends EventEmitterMfaSender {
  protected codec: Codec;
  protected socket: AuthenticatedWebSocket | undefined;
  private socketAddr: string;
  private sdManager: SharedDirectoryManager;
  private fastPathProcessor: FastPathProcessor | undefined;

  private logger = Logger.create('TDPClient');

  constructor(socketAddr: string) {
    super();
    this.socketAddr = socketAddr;
    this.codec = new Codec();
    this.sdManager = new SharedDirectoryManager();
  }

  // Connect to the websocket and register websocket event handlers.
  // Include a screen spec in cases where the client should determine the screen size
  // (e.g. in a desktop session) in order to automatically send it to the server and
  // start the session. Leave the screen spec undefined in cases where the server determines
  // the screen size (e.g. in a recording playback session). In that case, the client will
  // set the internal screen size when it receives the screen spec from the server
  // (see PlayerClient.handleClientScreenSpec).
  async connect(spec?: ClientScreenSpec) {
    await this.initWasm();

    this.socket = new AuthenticatedWebSocket(this.socketAddr);
    this.socket.binaryType = 'arraybuffer';

    this.socket.onopen = () => {
      this.logger.info('websocket is open');
      this.emit(TdpClientEvent.WS_OPEN);
      if (spec) {
        this.sendClientScreenSpec(spec);
      }
    };

    this.socket.onmessage = async (ev: MessageEvent) => {
      await this.processMessage(ev.data as ArrayBuffer);
    };

    // The socket 'error' event will only ever be emitted by the socket
    // prior to a socket 'close' event (https://stackoverflow.com/a/40084550/6277051).
    // Therefore, we can rely on our onclose handler to account for any websocket errors.
    this.socket.onerror = null;
    this.socket.onclose = ev => {
      let message = 'session disconnected';
      if (ev.code !== WebsocketCloseCode.NORMAL) {
        this.logger.error(`websocket closed with error code: ${ev.code}`);
        message = `connection closed with websocket error`;
      }
      this.logger.info('websocket is closed');

      // Clean up all of our socket's listeners and the socket itself.
      this.socket.onopen = null;
      this.socket.onmessage = null;
      this.socket.onclose = null;
      this.socket = null;

      this.emit(TdpClientEvent.WS_CLOSE, message);
    };
  }

  onClientError = (listener: (error: Error) => void) => {
    this.on(TdpClientEvent.CLIENT_ERROR, listener);
    return () => this.off(TdpClientEvent.CLIENT_ERROR, listener);
  };

  onClientWarning = (listener: (warningMessage: string) => void) => {
    this.on(TdpClientEvent.CLIENT_WARNING, listener);
    return () => this.off(TdpClientEvent.CLIENT_WARNING, listener);
  };

  onError = (listener: (error: Error) => void) => {
    this.on(TdpClientEvent.TDP_ERROR, listener);
    return () => this.off(TdpClientEvent.TDP_ERROR, listener);
  };

  onInfo = (listener: (info: string) => void) => {
    this.on(TdpClientEvent.TDP_INFO, listener);
    return () => this.off(TdpClientEvent.TDP_INFO, listener);
  };

  onReset = (listener: () => void) => {
    this.on(TdpClientEvent.RESET, listener);
    return () => this.off(TdpClientEvent.RESET, listener);
  };

  onBmpFrame = (listener: (bmpFrame: BitmapFrame) => void) => {
    this.on(TdpClientEvent.TDP_BMP_FRAME, listener);
    return () => this.off(TdpClientEvent.TDP_BMP_FRAME, listener);
  };

  onPngFrame = (listener: (pngFrame: PngFrame) => void) => {
    this.on(TdpClientEvent.TDP_PNG_FRAME, listener);
    return () => this.off(TdpClientEvent.TDP_PNG_FRAME, listener);
  };

  onPointer = (listener: (pointerData: PointerData) => void) => {
    this.on(TdpClientEvent.POINTER, listener);
    return () => this.off(TdpClientEvent.POINTER, listener);
  };

  onWarning = (listener: (warningMessage: string) => void) => {
    this.on(TdpClientEvent.TDP_WARNING, listener);
    return () => this.off(TdpClientEvent.TDP_WARNING, listener);
  };

  onWsClose = (listener: (message: string) => void) => {
    this.on(TdpClientEvent.WS_CLOSE, listener);
    return () => this.off(TdpClientEvent.WS_CLOSE, listener);
  };

  onWsOpen = (listener: () => void) => {
    this.on(TdpClientEvent.WS_OPEN, listener);
    return () => this.off(TdpClientEvent.WS_OPEN, listener);
  };

  onClipboardData = (listener: (clipboardData: ClipboardData) => void) => {
    this.on(TdpClientEvent.TDP_CLIPBOARD_DATA, listener);
    return () => this.off(TdpClientEvent.TDP_CLIPBOARD_DATA, listener);
  };

  onScreenSpec = (listener: (spec: ClientScreenSpec) => void) => {
    this.on(TdpClientEvent.TDP_CLIENT_SCREEN_SPEC, listener);
    return () => this.off(TdpClientEvent.TDP_CLIENT_SCREEN_SPEC, listener);
  };

  private async initWasm() {
    // select the wasm log level
    let wasmLogLevel = LogType.OFF;
    if (import.meta.env.MODE === 'development') {
      wasmLogLevel = LogType.TRACE;
    }

    await init();
    init_wasm_log(wasmLogLevel);
  }

  private initFastPathProcessor(
    ioChannelId: number,
    userChannelId: number,
    spec: ClientScreenSpec
  ) {
    this.logger.debug(
      `setting up fast path processor with screen spec ${spec.width} x ${spec.height}`
    );

    this.fastPathProcessor = new FastPathProcessor(
      spec.width,
      spec.height,
      ioChannelId,
      userChannelId
    );
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
        case MessageType.RDP_CONNECTION_ACTIVATED:
          this.handleRdpConnectionActivated(buffer);
          break;
        case MessageType.RDP_FASTPATH_PDU:
          this.handleRdpFastPathPDU(buffer);
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
        case MessageType.ALERT:
          this.handleTdpAlert(buffer);
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
        case MessageType.SHARED_DIRECTORY_TRUNCATE_REQUEST:
          this.handleSharedDirectoryTruncateRequest(buffer);
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

  handleTdpAlert(buffer: ArrayBuffer) {
    const alert = this.codec.decodeAlert(buffer);
    // TODO(zmb3): info and warning should use the same handler
    if (alert.severity === Severity.Error) {
      this.handleError(new Error(alert.message), TdpClientEvent.TDP_ERROR);
    } else if (alert.severity === Severity.Warning) {
      this.handleWarning(alert.message, TdpClientEvent.TDP_WARNING);
    } else {
      this.handleInfo(alert.message);
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

  handleRdpConnectionActivated(buffer: ArrayBuffer) {
    const { ioChannelId, userChannelId, screenWidth, screenHeight } =
      this.codec.decodeRdpConnectionActivated(buffer);
    const spec = { width: screenWidth, height: screenHeight };
    this.logger.info(
      `screen spec received from server ${spec.width} x ${spec.height}`
    );

    this.initFastPathProcessor(ioChannelId, userChannelId, {
      width: screenWidth,
      height: screenHeight,
    });

    // Emit the spec to any listeners. Listeners can then resize
    // the canvas to the size we're actually using in this session.
    this.emit(TdpClientEvent.TDP_CLIENT_SCREEN_SPEC, spec);
  }

  handleRdpFastPathPDU(buffer: ArrayBuffer) {
    let rdpFastPathPDU = this.codec.decodeRdpFastPathPDU(buffer);

    // This should never happen but let's catch it with an error in case it does.
    if (!this.fastPathProcessor)
      this.handleError(
        new Error('FastPathProcessor not initialized'),
        TdpClientEvent.CLIENT_ERROR
      );

    try {
      this.fastPathProcessor.process(
        rdpFastPathPDU,
        this,
        (bmpFrame: BitmapFrame) => {
          this.emit(TdpClientEvent.TDP_BMP_FRAME, bmpFrame);
        },
        (responseFrame: ArrayBuffer) => {
          this.sendRdpResponsePDU(responseFrame);
        },
        (data: ImageData | boolean, hotspot_x?: number, hotspot_y?: number) => {
          this.emit(TdpClientEvent.POINTER, { data, hotspot_x, hotspot_y });
        }
      );
    } catch (e) {
      this.handleError(e, TdpClientEvent.CLIENT_ERROR);
    }
  }

  handleMfaChallenge(buffer: ArrayBuffer) {
    try {
      const mfaJson = this.codec.decodeMfaJson(buffer);
      if (mfaJson.mfaType == 'n') {
        this.emit(TermEvent.MFA_CHALLENGE, mfaJson.jsonString);
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

  handleSharedDirectoryAcknowledge(buffer: ArrayBuffer) {
    const ack = this.codec.decodeSharedDirectoryAcknowledge(buffer);
    if (ack.errCode !== SharedDirectoryErrCode.Nil) {
      // A failure in the acknowledge message means the directory
      // share operation failed (likely due to server side configuration).
      // Since this is not a fatal error, we emit a warning but otherwise
      // keep the sesion alive.
      this.handleWarning(
        `Failed to share directory '${this.sdManager.getName()}', drive redirection may be disabled on the RDP server.`,
        TdpClientEvent.TDP_WARNING
      );
      return;
    }

    this.logger.info('Started sharing directory: ' + this.sdManager.getName());
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

  async handleSharedDirectoryTruncateRequest(buffer: ArrayBuffer) {
    const req = this.codec.decodeSharedDirectoryTruncateRequest(buffer);
    try {
      await this.sdManager.truncateFile(req.path, req.endOfFile);
      this.sendSharedDirectoryTruncateResponse({
        completionId: req.completionId,
        errCode: SharedDirectoryErrCode.Nil,
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

    this.logger.warn('websocket is not open');
  }

  sendClientScreenSpec(spec: ClientScreenSpec) {
    this.logger.info(
      `requesting screen spec from client ${spec.width} x ${spec.height}`
    );
    this.send(this.codec.encodeClientScreenSpec(spec));
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
    this.codec.encodeKeyboardInput(code, state).forEach(msg => this.send(msg));
  }

  sendSyncKeys(syncKeys: SyncKeys) {
    this.send(this.codec.encodeSyncKeys(syncKeys));
  }

  sendClipboardData(clipboardData: ClipboardData) {
    this.send(this.codec.encodeClipboardData(clipboardData));
  }

  sendChallengeResponse(data: MfaChallengeResponse) {
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

  sendSharedDirectoryTruncateResponse(
    response: SharedDirectoryTruncateResponse
  ) {
    this.send(this.codec.encodeSharedDirectoryTruncateResponse(response));
  }

  resize(spec: ClientScreenSpec) {
    this.sendClientScreenSpec(spec);
  }

  sendRdpResponsePDU(responseFrame: ArrayBuffer) {
    this.send(this.codec.encodeRdpResponsePDU(responseFrame));
  }

  // Emits an errType event and closes the websocket connection.
  // Should only be used for fatal errors.
  private handleError(
    err: Error,
    errType: TdpClientEvent.TDP_ERROR | TdpClientEvent.CLIENT_ERROR
  ) {
    this.logger.error(err);
    this.emit(errType, err);
    // All errors are fatal, meaning that we are closing the connection after they happen.
    // To prevent overwriting such error with our close handler, remove it before
    // closing the connection.
    if (this.socket) {
      this.socket.onclose = null;
      this.socket.close();
    }
  }

  // Emits a warning event, but keeps the socket open.
  private handleWarning(
    warning: string,
    warnType: TdpClientEvent.TDP_WARNING | TdpClientEvent.CLIENT_WARNING
  ) {
    this.logger.warn(warning);
    this.emit(warnType, warning);
  }

  private handleInfo(info: string) {
    this.logger.info(info);
    this.emit(TdpClientEvent.TDP_INFO, info);
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

export function useListener<T extends any[]>(
  emitter: (callback: (...args: T) => void) => () => void | undefined,
  listener: ((...args: T) => void) | undefined
) {
  useEffect(() => {
    if (!emitter) {
      return;
    }
    const unregister = emitter((...args) => listener?.(...args));
    return () => {
      unregister();
    };
  }, [emitter, listener]);
}
