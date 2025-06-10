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

import { EventEmitter } from 'events';

import { useEffect } from 'react';

import init, {
  FastPathProcessor,
  init_wasm_log,
} from 'shared/libs/ironrdp/pkg/ironrdp';
import Logger from 'shared/libs/logger';
import { ensureError, isAbortError } from 'shared/utils/error';

import Codec, {
  FileType,
  LatencyStats,
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
  SharedDirectoryAccess,
  type FileOrDirInfo,
} from './sharedDirectoryAccess';

export enum TdpClientEvent {
  TDP_CLIENT_SCREEN_SPEC = 'tdp client screen spec',
  TDP_PNG_FRAME = 'tdp png frame',
  TDP_BMP_FRAME = 'tdp bmp frame',
  TDP_CLIPBOARD_DATA = 'tdp clipboard data',
  // Represents either a remote TDP error or a client-side error.
  ERROR = 'error',
  // TDP_WARNING corresponds the TDP warning message
  TDP_WARNING = 'tdp warning',
  // CLIENT_WARNING represents a warning event that isn't a TDP_WARNING
  CLIENT_WARNING = 'client warning',
  // TDP_INFO corresponds with the TDP info message
  TDP_INFO = 'tdp info',
  TRANSPORT_OPEN = 'transport open',
  // TRANSPORT_CLOSE is emitted when a connection ends due to the transport layer being closed.
  // This can occur with or without an error.
  // If an error is present, it will be displayed in the UI.
  TRANSPORT_CLOSE = 'transport close',
  RESET = 'reset',
  POINTER = 'pointer',
  LATENCY_STATS = 'latency stats',
}

type EventMap = {
  [TdpClientEvent.TDP_CLIENT_SCREEN_SPEC]: [ClientScreenSpec];
  [TdpClientEvent.TDP_PNG_FRAME]: [PngFrame];
  [TdpClientEvent.TDP_BMP_FRAME]: [BitmapFrame];
  [TdpClientEvent.TDP_CLIPBOARD_DATA]: [ClipboardData];
  [TdpClientEvent.ERROR]: [Error];
  [TdpClientEvent.TDP_WARNING]: [string];
  [TdpClientEvent.CLIENT_WARNING]: [string];
  [TdpClientEvent.TDP_INFO]: [string];
  [TdpClientEvent.TRANSPORT_OPEN]: [void];
  [TdpClientEvent.TRANSPORT_CLOSE]: [Error | undefined];
  [TdpClientEvent.RESET]: [void];
  [TdpClientEvent.POINTER]: [PointerData];
  [TdpClientEvent.LATENCY_STATS]: [LatencyStats];
  'terminal.webauthn': [string];
};

export enum LogType {
  OFF = 'OFF',
  ERROR = 'ERROR',
  WARN = 'WARN',
  INFO = 'INFO',
  DEBUG = 'DEBUG',
  TRACE = 'TRACE',
}

export interface TdpTransport {
  /** Sends a message down the stream. */
  send(data: string | ArrayBufferLike): void;
  /** Adds a callback for every new message. */
  onMessage(callback: (data: ArrayBufferLike) => void): RemoveListenerFn;
  /**
   * Adds a callback for errors.
   * The stream is closed when this callback is called.
   */
  onError(callback: (error: Error) => void): RemoveListenerFn;
  /**
   * Adds a callback for stream completion.
   * The stream is closed when this callback is called.
   */
  onComplete(callback: () => void): RemoveListenerFn;
}

type RemoveListenerFn = () => void;

// WASM IronRDP code can only be initialized once; repeated attempts will cause an error.
// To prevent multiple initializations, we track the initialization status in a global variable.
let wasmReady: Promise<void> | undefined;

// Client is the TDP client. It is responsible for connecting to a websocket serving the tdp server,
// sending client commands, and receiving and processing server messages. Its creator is responsible for
// ensuring the websocket gets closed and all of its event listeners cleaned up when it is no longer in use.
// For convenience, this can be done in one fell swoop by calling Client.shutdown().
export class TdpClient extends EventEmitter<EventMap> {
  protected codec: Codec;
  protected transport: TdpTransport | undefined;
  private transportAbortController: AbortController | undefined;
  private fastPathProcessor: FastPathProcessor | undefined;

  private logger = Logger.create('TDPClient');

  constructor(
    private getTransport: (signal: AbortSignal) => Promise<TdpTransport>,
    private sharedDirectoryAccess: SharedDirectoryAccess
  ) {
    super();
    this.codec = new Codec();
  }

  /** Connects to the transport and registers event handlers. */
  async connect(
    options: {
      /**
       * Client keyboard layout.
       * This should be provided only for a desktop session
       * (desktop player doesn't allow this parameter).
       */
      keyboardLayout?: number;
      /**
       * Client screen size.
       * This should be provided only for a desktop session
       * (desktop player doesn't allow this parameter).
       */
      screenSpec?: ClientScreenSpec;
    } = {}
  ) {
    this.transportAbortController = new AbortController();
    if (!wasmReady) {
      wasmReady = this.initWasm();
    }
    await wasmReady;

    try {
      this.transport = await this.getTransport(
        this.transportAbortController.signal
      );
    } catch (error) {
      this.emit(TdpClientEvent.ERROR, ensureError(error));
      return;
    }

    this.emit(TdpClientEvent.TRANSPORT_OPEN);
    if (options.screenSpec) {
      this.sendClientScreenSpec(options.screenSpec);
    }

    if (options.keyboardLayout !== undefined) {
      this.sendClientKeyboardLayout(options.keyboardLayout);
    }

    let processingError: Error | undefined;
    let connectionError: Error | undefined;
    await new Promise<void>(resolve => {
      const subscribers = new Set<() => void>();
      const unsubscribe = () => {
        subscribers.forEach(unsubscribe => unsubscribe());
        resolve();
      };

      subscribers.add(
        this.transport.onMessage(data => {
          void this.processMessage(data).catch(error => {
            processingError = ensureError(error);
            unsubscribe();
            // All errors are treated as fatal, close the connection.
            this.transportAbortController.abort();
          });
        })
      );
      subscribers.add(
        this.transport.onError(error => {
          connectionError = error;
          unsubscribe();
        })
      );
      subscribers.add(this.transport.onComplete(unsubscribe));
    });

    // 'Processing' errors are the most important.
    if (processingError) {
      this.emit(TdpClientEvent.ERROR, processingError);
      // If the connection was closed intentionally by the user (aborted),
      // do not treat it as an error in the UI.
    } else if (connectionError && !isAbortError(connectionError)) {
      this.emit(TdpClientEvent.TRANSPORT_CLOSE, connectionError);
    } else {
      this.emit(TdpClientEvent.TRANSPORT_CLOSE, undefined);
    }

    this.logger.info('Transport is closed');

    this.transport = undefined;
  }

  onClientWarning = (listener: (warningMessage: string) => void) => {
    this.on(TdpClientEvent.CLIENT_WARNING, listener);
    return () => this.off(TdpClientEvent.CLIENT_WARNING, listener);
  };

  onError = (listener: (error: Error) => void) => {
    this.on(TdpClientEvent.ERROR, listener);
    return () => this.off(TdpClientEvent.ERROR, listener);
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

  onTransportClose = (listener: (error?: Error) => void) => {
    this.on(TdpClientEvent.TRANSPORT_CLOSE, listener);
    return () => this.off(TdpClientEvent.TRANSPORT_CLOSE, listener);
  };

  onTransportOpen = (listener: () => void) => {
    this.on(TdpClientEvent.TRANSPORT_OPEN, listener);
    return () => this.off(TdpClientEvent.TRANSPORT_OPEN, listener);
  };

  onClipboardData = (listener: (clipboardData: ClipboardData) => void) => {
    this.on(TdpClientEvent.TDP_CLIPBOARD_DATA, listener);
    return () => this.off(TdpClientEvent.TDP_CLIPBOARD_DATA, listener);
  };

  onScreenSpec = (listener: (spec: ClientScreenSpec) => void) => {
    this.on(TdpClientEvent.TDP_CLIENT_SCREEN_SPEC, listener);
    return () => this.off(TdpClientEvent.TDP_CLIENT_SCREEN_SPEC, listener);
  };

  onLatencyStats = (listener: (stats: LatencyStats) => void) => {
    this.on(TdpClientEvent.LATENCY_STATS, listener);
    return () => this.off(TdpClientEvent.LATENCY_STATS, listener);
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
  async processMessage(buffer: ArrayBufferLike): Promise<void> {
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
        throw new Error(this.codec.decodeErrorMessage(buffer));
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
        await this.handleSharedDirectoryInfoRequest(buffer);
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
        await this.handleSharedDirectoryDeleteRequest(buffer);
        break;
      case MessageType.SHARED_DIRECTORY_READ_REQUEST:
        await this.handleSharedDirectoryReadRequest(buffer);
        break;
      case MessageType.SHARED_DIRECTORY_WRITE_REQUEST:
        await this.handleSharedDirectoryWriteRequest(buffer);
        break;
      case MessageType.SHARED_DIRECTORY_MOVE_REQUEST:
        this.handleSharedDirectoryMoveRequest(buffer);
        break;
      case MessageType.SHARED_DIRECTORY_LIST_REQUEST:
        await this.handleSharedDirectoryListRequest(buffer);
        break;
      case MessageType.SHARED_DIRECTORY_TRUNCATE_REQUEST:
        await this.handleSharedDirectoryTruncateRequest(buffer);
        break;
      case MessageType.LATENCY_STATS:
        this.handleLatencyStats(buffer);
        break;
      default:
        this.logger.warn(`received unsupported message type ${messageType}`);
    }
  }

  handleLatencyStats(buffer: ArrayBufferLike) {
    const stats = this.codec.decodeLatencyStats(buffer);
    this.emit(TdpClientEvent.LATENCY_STATS, stats);
  }

  handleClientScreenSpec(buffer: ArrayBufferLike) {
    this.logger.warn(
      `received unsupported message type ${this.codec.decodeMessageType(
        buffer
      )}`
    );
  }

  handleMouseButton(buffer: ArrayBufferLike) {
    this.logger.warn(
      `received unsupported message type ${this.codec.decodeMessageType(
        buffer
      )}`
    );
  }

  handleMouseMove(buffer: ArrayBufferLike) {
    this.logger.warn(
      `received unsupported message type ${this.codec.decodeMessageType(
        buffer
      )}`
    );
  }

  handleClipboardData(buffer: ArrayBufferLike) {
    this.emit(
      TdpClientEvent.TDP_CLIPBOARD_DATA,
      this.codec.decodeClipboardData(buffer)
    );
  }

  handleTdpAlert(buffer: ArrayBufferLike) {
    const alert = this.codec.decodeAlert(buffer);
    // TODO(zmb3): info and warning should use the same handler
    if (alert.severity === Severity.Error) {
      throw new TdpError(alert.message);
    } else if (alert.severity === Severity.Warning) {
      this.handleWarning(alert.message, TdpClientEvent.TDP_WARNING);
    } else {
      this.handleInfo(alert.message);
    }
  }

  // Assuming we have a message of type PNG_FRAME, extract its
  // bounds and png bitmap and emit a render event.
  handlePngFrame(buffer: ArrayBufferLike) {
    this.codec.decodePngFrame(buffer, (pngFrame: PngFrame) =>
      this.emit(TdpClientEvent.TDP_PNG_FRAME, pngFrame)
    );
  }

  handlePng2Frame(buffer: ArrayBufferLike) {
    this.codec.decodePng2Frame(buffer, (pngFrame: PngFrame) =>
      this.emit(TdpClientEvent.TDP_PNG_FRAME, pngFrame)
    );
  }

  handleRdpConnectionActivated(buffer: ArrayBufferLike) {
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

  handleRdpFastPathPDU(buffer: ArrayBufferLike) {
    let rdpFastPathPDU = this.codec.decodeRdpFastPathPDU(buffer);

    // This should never happen but let's catch it with an error in case it does.
    if (!this.fastPathProcessor) {
      throw new Error('FastPathProcessor not initialized');
    }

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
  }

  handleMfaChallenge(buffer: ArrayBufferLike) {
    const mfaJson = this.codec.decodeMfaJson(buffer);
    if (mfaJson.mfaType == 'n') {
      // TermEvent.MFA_CHALLENGE
      this.emit('terminal.webauthn', mfaJson.jsonString);
    } else {
      // mfaJson.mfaType === 'u', or else decodeMfaJson would have thrown an error.
      throw new Error(
        'Multifactor authentication is required for accessing this desktop, \
      however the U2F API for hardware keys is not supported for desktop sessions. \
      Please notify your system administrator to update cluster settings \
      to use WebAuthn as the second factor protocol.'
      );
    }
  }

  handleSharedDirectoryAcknowledge(buffer: ArrayBufferLike) {
    const ack = this.codec.decodeSharedDirectoryAcknowledge(buffer);
    if (ack.errCode !== SharedDirectoryErrCode.Nil) {
      // A failure in the acknowledge message means the directory
      // share operation failed (likely due to server side configuration).
      // Since this is not a fatal error, we emit a warning but otherwise
      // keep the sesion alive.
      this.handleWarning(
        `Failed to share directory '${this.sharedDirectoryAccess.getDirectoryName()}', drive redirection may be disabled on the RDP server.`,
        TdpClientEvent.TDP_WARNING
      );
      return;
    }

    this.logger.info(
      `Started sharing directory: ${this.sharedDirectoryAccess.getDirectoryName()}`
    );
  }

  async handleSharedDirectoryInfoRequest(buffer: ArrayBufferLike) {
    const req = this.codec.decodeSharedDirectoryInfoRequest(buffer);
    const path = req.path;
    try {
      const info = await this.sharedDirectoryAccess.stat(path);
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
        throw e;
      }
    }
  }

  async handleSharedDirectoryCreateRequest(buffer: ArrayBufferLike) {
    const req = this.codec.decodeSharedDirectoryCreateRequest(buffer);

    try {
      await this.sharedDirectoryAccess.create(req.path, req.fileType);
      const info = await this.sharedDirectoryAccess.stat(req.path);
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

  async handleSharedDirectoryDeleteRequest(buffer: ArrayBufferLike) {
    const req = this.codec.decodeSharedDirectoryDeleteRequest(buffer);

    try {
      await this.sharedDirectoryAccess.delete(req.path);
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

  async handleSharedDirectoryReadRequest(buffer: ArrayBufferLike) {
    const req = this.codec.decodeSharedDirectoryReadRequest(buffer);
    const readData = await this.sharedDirectoryAccess.read(
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
  }

  async handleSharedDirectoryWriteRequest(buffer: ArrayBufferLike) {
    const req = this.codec.decodeSharedDirectoryWriteRequest(buffer);
    const bytesWritten = await this.sharedDirectoryAccess.write(
      req.path,
      req.offset,
      req.writeData
    );

    this.sendSharedDirectoryWriteResponse({
      completionId: req.completionId,
      errCode: SharedDirectoryErrCode.Nil,
      bytesWritten,
    });
  }

  handleSharedDirectoryMoveRequest(buffer: ArrayBufferLike) {
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

  async handleSharedDirectoryListRequest(buffer: ArrayBufferLike) {
    const req = this.codec.decodeSharedDirectoryListRequest(buffer);
    const path = req.path;

    const infoList: FileOrDirInfo[] =
      await this.sharedDirectoryAccess.readDir(path);
    const fsoList: FileSystemObject[] = infoList.map(info => this.toFso(info));

    this.sendSharedDirectoryListResponse({
      completionId: req.completionId,
      errCode: SharedDirectoryErrCode.Nil,
      fsoList,
    });
  }

  async handleSharedDirectoryTruncateRequest(buffer: ArrayBufferLike) {
    const req = this.codec.decodeSharedDirectoryTruncateRequest(buffer);
    await this.sharedDirectoryAccess.truncate(req.path, req.endOfFile);
    this.sendSharedDirectoryTruncateResponse({
      completionId: req.completionId,
      errCode: SharedDirectoryErrCode.Nil,
    });
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

  protected send(data: string | ArrayBufferLike): void {
    if (!this.transport) {
      this.logger.info('Transport is not ready, discarding message');
      return;
    }
    this.transport.send(data);
  }

  sendClientScreenSpec(spec: ClientScreenSpec) {
    this.logger.info(
      `requesting screen spec from client ${spec.width} x ${spec.height}`
    );
    this.send(this.codec.encodeClientScreenSpec(spec));
  }

  sendClientKeyboardLayout(keyboardLayout: number) {
    this.send(this.codec.encodeClientKeyboardLayout(keyboardLayout));
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

  sendChallengeResponse(data: {
    totp_code?: string;
    webauthn_response?: {
      id: string;
      type: string;
      extensions: {
        appid: boolean;
      };
      rawId: string;
      response: {
        authenticatorData: string;
        clientDataJSON: string;
        signature: string;
        userHandle: string;
      };
    };
    sso_response?: {
      requestId: string;
      token: string;
    };
  }) {
    const msg = this.codec.encodeMfaJson({
      mfaType: 'n',
      jsonString: JSON.stringify(data),
    });
    this.send(msg);
  }

  async shareDirectory() {
    await this.sharedDirectoryAccess.selectDirectory();
    this.sendSharedDirectoryAnnounce();
  }

  sendSharedDirectoryAnnounce() {
    const name = this.sharedDirectoryAccess.getDirectoryName();
    this.send(
      this.codec.encodeSharedDirectoryAnnounce({
        discard: 0, // This is always the first request.
        // Hardcode directoryId for now since we only support sharing 1 directory.
        // We're using 2 because the smartcard device is hardcoded to 1 in the backend.
        directoryId: 2,
        name,
      })
    );
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

  resize = (spec: ClientScreenSpec) => {
    this.sendClientScreenSpec(spec);
  };

  sendRdpResponsePDU(responseFrame: ArrayBufferLike) {
    this.send(this.codec.encodeRdpResponsePDU(responseFrame));
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

  // It's safe to call this multiple times, calls subsequent to the first call
  // will simply do nothing.
  shutdown() {
    this.transportAbortController?.abort();
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

/** Represents an alert emitted by the TDP service with "error" severity. */
export class TdpError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'TdpError';
  }
}
