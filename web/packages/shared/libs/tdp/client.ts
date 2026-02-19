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

import { Logger } from 'design/logger';
import init, {
  FastPathProcessor,
  init_wasm_log,
} from 'shared/libs/ironrdp/pkg/ironrdp';
import { ensureError, isAbortError } from 'shared/utils/error';

import {
  Alert,
  Codec,
  FileType,
  LatencyStats,
  MfaJson,
  MfaResponse,
  MouseButtonState,
  MouseMove,
  PointerData,
  RdpConnectionActivated,
  RdpFastPathPdu,
  ServerHello,
  Severity,
  SharedDirectoryErrCode,
  TdpbCodec,
  TdpCodec,
  type ButtonState,
  type ClientScreenSpec,
  type ClipboardData,
  type FileSystemObject,
  type MouseButton,
  type PngFrame,
  type ScrollAxis,
  type SharedDirectoryAcknowledge,
  type SharedDirectoryCreateRequest,
  type SharedDirectoryCreateResponse,
  type SharedDirectoryDeleteRequest,
  type SharedDirectoryDeleteResponse,
  type SharedDirectoryInfoRequest,
  type SharedDirectoryInfoResponse,
  type SharedDirectoryListRequest,
  type SharedDirectoryListResponse,
  type SharedDirectoryMoveRequest,
  type SharedDirectoryMoveResponse,
  type SharedDirectoryReadRequest,
  type SharedDirectoryReadResponse,
  type SharedDirectoryTruncateRequest,
  type SharedDirectoryTruncateResponse,
  type SharedDirectoryWriteRequest,
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
  CONNECTION_OPEN = 'connection open',
  // CONNECTION_CLOSE is emitted when a connection ends due to the transport layer being closed.
  // This can occur with or without an error.
  // If an error is present, it will be displayed in the UI.
  CONNECTION_CLOSE = 'connection close',
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
  [TdpClientEvent.CONNECTION_OPEN]: [void];
  [TdpClientEvent.CONNECTION_CLOSE]: [Error | undefined];
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

// Defines which protocol the client will start with.
type ConnectPolicy = { mode: 'tdpb' } | { mode: 'tdp' };

// Client is the TDP client. It is responsible for connecting to a websocket serving the tdp server,
// sending client commands, and receiving and processing server messages. Its creator is responsible for
// ensuring the websocket gets closed and all of its event listeners cleaned up when it is no longer in use.
// For convenience, this can be done in one fell swoop by calling Client.shutdown().
export class TdpClient extends EventEmitter<EventMap> {
  protected transport: TdpTransport | undefined;
  private transportAbortController: AbortController | undefined;
  private fastPathProcessor: FastPathProcessor | undefined;
  private sharedDirectory: SharedDirectoryAccess | undefined;
  private keyboardLayout: number | undefined;
  private screenSpec: ClientScreenSpec | undefined;
  private codec: Codec | undefined;

  private logger = new Logger('TDPClient');

  constructor(
    private getTransport: (signal: AbortSignal) => Promise<TdpTransport>,
    private selectSharedDirectory: () => Promise<SharedDirectoryAccess>,
    private policy: ConnectPolicy = { mode: 'tdp' }
  ) {
    super();
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
    // Initialize our codec according to the connection policy.
    switch (this.policy.mode) {
      case 'tdp':
        // tdp policy is capable of upgrading to TDPB.
        this.codec = new TdpCodec();
        break;
      case 'tdpb':
        this.codec = new TdpbCodec();
        break;
      default:
        const exhaustiveCheck: never = this.policy;
        throw new Error(`Unknown connect policy: ${exhaustiveCheck}`);
    }

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

    // Encode and send initial messages
    this.screenSpec = options.screenSpec;
    this.keyboardLayout = options.keyboardLayout;
    this.codec
      .encodeInitialMessages(options.screenSpec, options.keyboardLayout)
      .forEach(msg => this.send(msg));
    this.emit(TdpClientEvent.CONNECTION_OPEN);

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
      this.emit(TdpClientEvent.CONNECTION_CLOSE, connectionError);
    } else {
      this.emit(TdpClientEvent.CONNECTION_CLOSE, undefined);
    }

    this.logger.info('Transport is closed');

    this.sharedDirectory = undefined;
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
    this.on(TdpClientEvent.CONNECTION_CLOSE, listener);
    return () => this.off(TdpClientEvent.CONNECTION_CLOSE, listener);
  };

  onTransportOpen = (listener: () => void) => {
    this.on(TdpClientEvent.CONNECTION_OPEN, listener);
    return () => this.off(TdpClientEvent.CONNECTION_OPEN, listener);
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
  async processMessage(
    buffer: ArrayBufferLike,
    codecOverride?: Codec
  ): Promise<void> {
    let codec = this.codec;
    if (codecOverride) {
      // Allow the caller to override the codec.
      codec = codecOverride;
    }

    const result = codec.decodeMessage(buffer);
    if (!result) {
      // Codec implementations *should* return an 'unknown' result kind
      // instead of undefined, but double check anyway for safety.
      this.logger.warn('message decoder returned undefined result');
      return;
    }

    switch (result.kind) {
      case 'pngFrame':
        this.handlePngFrame(result.data);
        break;
      case 'rdpConnectionActivated':
        // Comes from TDP codec
        this.handleRdpConnectionActivated(result.data);
        break;
      case 'serverHello':
        // Comes from TDPB codec
        this.handleServerHello(result.data);
        break;
      case 'rdpFastPathPdu':
        this.handleRdpFastPathPdu(result.data);
        break;
      case 'clipboardData':
        this.handleClipboardData(result.data);
        break;
      case 'tdpAlert':
        this.handleTdpAlert(result.data);
        break;
      case 'mfaChallenge':
        this.handleMfaChallenge(result.data);
        break;
      case 'sharedDirectoryAcknowledge':
        this.handleSharedDirectoryAcknowledge(result.data);
        break;
      case 'sharedDirectoryInfoRequest':
        await this.handleSharedDirectoryInfoRequest(result.data);
        break;
      case 'sharedDirectoryCreateRequest':
        // A typical sequence is that we receive a SharedDirectoryCreateRequest
        // immediately followed by a SharedDirectoryWriteRequest. It's important
        // that we await here so that this client doesn't field the SharedDirectoryWriteRequest
        // until the create has successfully completed, or else we might get an error
        // trying to write to a file that hasn't been created yet.
        await this.handleSharedDirectoryCreateRequest(result.data);
        break;
      case 'sharedDirectoryDeleteRequest':
        await this.handleSharedDirectoryDeleteRequest(result.data);
        break;
      case 'sharedDirectoryReadRequest':
        await this.handleSharedDirectoryReadRequest(result.data);
        break;
      case 'sharedDirectoryWriteRequest':
        await this.handleSharedDirectoryWriteRequest(result.data);
        break;
      case 'sharedDirectoryMoveRequest':
        this.handleSharedDirectoryMoveRequest(result.data);
        break;
      case 'sharedDirectoryListRequest':
        await this.handleSharedDirectoryListRequest(result.data);
        break;
      case 'sharedDirectoryTruncateRequest':
        await this.handleSharedDirectoryTruncateRequest(result.data);
        break;
      case 'latencyStats':
        this.handleLatencyStats(result.data);
        break;
      case 'tdpbUpgrade':
        this.handleTdpbUpgrade();
        break;
      case 'clientScreenSpec':
        this.handleClientScreenSpec(result.data);
        break;
      case 'mouseButton':
        this.handleMouseButton(result.data);
        break;
      case 'mouseMove':
        this.handleMouseMove(result.data);
        break;
      case 'unknown':
        // Truly unknown message types. The envelope is empty or
        // or the server's schema is ahead of ours.
        this.logger.debug(`received unknown message type`);
        break;
      case 'unsupported':
        // Message types that we know about, but deliberately do no support on the client.
        // 'data' should be the unsupported message kind.
        this.logger.debug(
          `received message type not supported by this client ${result.data}`
        );
        break;
      default:
        const exhaustiveCheck: never = result;
        throw new Error(`Message type: ${exhaustiveCheck}`);
    }
  }

  handleLatencyStats(stats: LatencyStats) {
    this.emit(TdpClientEvent.LATENCY_STATS, stats);
  }

  handleTdpbUpgrade() {
    // Swap our codec to the TDPB codec.
    const tdpbCodec = new TdpbCodec();
    this.codec = tdpbCodec;

    // Send the TDPB client hello
    this.send(
      tdpbCodec.encodeClientHello({
        keyboardLayout: this.keyboardLayout,
        screenSpec: this.screenSpec,
      })
    );
  }

  handleServerHello(hello: ServerHello) {
    // In the future, we may add new server capability advertisements
    // that will affect client configuration.
    // For now we'll just activate the the connection.
    this.handleRdpConnectionActivated(hello.activationEvent);
  }

  handleClientScreenSpec(spec: ClientScreenSpec) {
    void spec;
    this.logger.warn('received unexpected client screen spec message');
  }

  handleMouseButton(button: MouseButtonState) {
    void button;
    this.logger.warn('received unexpected mouse button message');
  }

  handleMouseMove(move: MouseMove) {
    void move;
    this.logger.warn('received unexpected mouse move message');
  }

  handleClipboardData(data: ClipboardData) {
    this.emit(TdpClientEvent.TDP_CLIPBOARD_DATA, data);
  }

  handleTdpAlert(alert: Alert) {
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
  handlePngFrame(frame: PngFrame) {
    this.emit(TdpClientEvent.TDP_PNG_FRAME, frame);
  }

  handleRdpConnectionActivated(activated: RdpConnectionActivated) {
    const { ioChannelId, userChannelId, screenWidth, screenHeight } = activated;
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

  handleRdpFastPathPdu(rdpFastPathPdu: RdpFastPathPdu) {
    // This should never happen but let's catch it with an error in case it does.
    if (!this.fastPathProcessor) {
      throw new Error('FastPathProcessor not initialized');
    }

    this.fastPathProcessor.process(
      rdpFastPathPdu,
      this,
      (bmpFrame: BitmapFrame) => {
        this.emit(TdpClientEvent.TDP_BMP_FRAME, bmpFrame);
      },
      (responseFrame: ArrayBuffer) => {
        this.sendRdpResponsePdu(responseFrame);
      },
      (data: ImageData | boolean, hotspot_x?: number, hotspot_y?: number) => {
        this.emit(TdpClientEvent.POINTER, { data, hotspot_x, hotspot_y });
      }
    );
  }

  handleMfaChallenge(mfaJson: MfaJson) {
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

  handleSharedDirectoryAcknowledge(ack: SharedDirectoryAcknowledge) {
    const sharedDirectory = this.getSharedDirectoryOrThrow();

    if (ack.errCode !== SharedDirectoryErrCode.Nil) {
      // A failure in the acknowledge message means the directory
      // share operation failed (likely due to server side configuration).
      // Since this is not a fatal error, we emit a warning but otherwise
      // keep the sesion alive.
      this.handleWarning(
        `Failed to share directory '${sharedDirectory.getDirectoryName()}', drive redirection may be disabled on the RDP server.`,
        TdpClientEvent.TDP_WARNING
      );
      return;
    }

    this.logger.info(
      `Started sharing directory: ${sharedDirectory.getDirectoryName()}`
    );
  }

  async handleSharedDirectoryInfoRequest(req: SharedDirectoryInfoRequest) {
    const path = req.path;
    const sharedDirectory = this.getSharedDirectoryOrThrow();

    try {
      const info = await sharedDirectory.stat(path);
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

  async handleSharedDirectoryCreateRequest(req: SharedDirectoryCreateRequest) {
    const sharedDirectory = this.getSharedDirectoryOrThrow();
    try {
      await sharedDirectory.create(req.path, req.fileType);
      const info = await sharedDirectory.stat(req.path);
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

  async handleSharedDirectoryDeleteRequest(req: SharedDirectoryDeleteRequest) {
    const sharedDirectory = this.getSharedDirectoryOrThrow();

    try {
      await sharedDirectory.delete(req.path);
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

  async handleSharedDirectoryReadRequest(req: SharedDirectoryReadRequest) {
    const sharedDirectory = this.getSharedDirectoryOrThrow();

    const readData = await sharedDirectory.read(
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

  async handleSharedDirectoryWriteRequest(req: SharedDirectoryWriteRequest) {
    const sharedDirectory = this.getSharedDirectoryOrThrow();

    const bytesWritten = await sharedDirectory.write(
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

  handleSharedDirectoryMoveRequest(req: SharedDirectoryMoveRequest) {
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

  async handleSharedDirectoryListRequest(req: SharedDirectoryListRequest) {
    const path = req.path;
    const sharedDirectory = this.getSharedDirectoryOrThrow();

    const infoList = await sharedDirectory.readDir(path);
    const fsoList = infoList.map(info => this.toFso(info));

    this.sendSharedDirectoryListResponse({
      completionId: req.completionId,
      errCode: SharedDirectoryErrCode.Nil,
      fsoList,
    });
  }

  async handleSharedDirectoryTruncateRequest(
    req: SharedDirectoryTruncateRequest
  ) {
    const sharedDirectory = this.getSharedDirectoryOrThrow();

    await sharedDirectory.truncate(req.path, req.endOfFile);
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

  sendChallengeResponse(data: MfaResponse) {
    const msg = this.codec.encodeMfaJson(data);
    this.send(msg);
  }

  async shareDirectory() {
    if (this.sharedDirectory) {
      throw new Error('Only one shared directory is allowed at a time.');
    }
    this.sharedDirectory = await this.selectSharedDirectory();
    this.sendSharedDirectoryAnnounce();
  }

  sendSharedDirectoryAnnounce() {
    const name = this.sharedDirectory.getDirectoryName();
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

  sendRdpResponsePdu(responseFrame: ArrayBufferLike) {
    this.send(this.codec.encodeRdpResponsePdu(responseFrame));
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

  private getSharedDirectoryOrThrow() {
    if (!this.sharedDirectory) {
      throw new Error('No shared directory has been initialized.');
    }
    return this.sharedDirectory;
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
