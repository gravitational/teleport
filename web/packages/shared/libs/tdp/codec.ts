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

import { Logger } from 'design/logger';
import {
  Envelope,
  MFA,
  SharedDirectoryRequest,
  SharedDirectoryResponse,
  ClientHello as tdpbClientHello,
} from 'gen-proto-ts/teleport/desktop/v1/tdpb_pb';
import { CredentialAssertion } from 'gen-proto-ts/teleport/legacy/types/webauthn/webauthn_pb';
import {
  AuthenticateResponse,
  SSOChallenge,
} from 'gen-proto-ts/teleport/mfa/v1/challenge_pb';
import { arrayBufferToBase64, base64urlToBuffer } from 'shared/utils/base64';

export type Message = ArrayBufferLike;

export enum MessageType {
  CLIENT_SCREEN_SPEC = 1,
  PNG_FRAME = 2,
  MOUSE_MOVE = 3,
  MOUSE_BUTTON = 4,
  KEYBOARD_BUTTON = 5,
  CLIPBOARD_DATA = 6,
  CLIENT_USERNAME = 7,
  MOUSE_WHEEL_SCROLL = 8,
  ERROR = 9,
  MFA_JSON = 10,
  SHARED_DIRECTORY_ANNOUNCE = 11,
  SHARED_DIRECTORY_ACKNOWLEDGE = 12,
  SHARED_DIRECTORY_INFO_REQUEST = 13,
  SHARED_DIRECTORY_INFO_RESPONSE = 14,
  SHARED_DIRECTORY_CREATE_REQUEST = 15,
  SHARED_DIRECTORY_CREATE_RESPONSE = 16,
  SHARED_DIRECTORY_DELETE_REQUEST = 17,
  SHARED_DIRECTORY_DELETE_RESPONSE = 18,
  SHARED_DIRECTORY_READ_REQUEST = 19,
  SHARED_DIRECTORY_READ_RESPONSE = 20,
  SHARED_DIRECTORY_WRITE_REQUEST = 21,
  SHARED_DIRECTORY_WRITE_RESPONSE = 22,
  SHARED_DIRECTORY_MOVE_REQUEST = 23,
  SHARED_DIRECTORY_MOVE_RESPONSE = 24,
  SHARED_DIRECTORY_LIST_REQUEST = 25,
  SHARED_DIRECTORY_LIST_RESPONSE = 26,
  PNG2_FRAME = 27,
  ALERT = 28,
  RDP_FASTPATH_PDU = 29,
  RDP_RESPONSE_PDU = 30,
  RDP_CONNECTION_ACTIVATED = 31,
  SYNC_KEYS = 32,
  SHARED_DIRECTORY_TRUNCATE_REQUEST = 33,
  SHARED_DIRECTORY_TRUNCATE_RESPONSE = 34,
  LATENCY_STATS = 35,
  // MessageType 36 is a server-side only Ping message
  CLIENT_KEYBOARD_LAYOUT = 37,
  TDPB_UPGRADE = 38,
  __LAST, // utility value
}

// 0 is left button, 1 is middle button, 2 is right button
export type MouseButton = 0 | 1 | 2;

export type MouseMove = {
  x: number;
  y: number;
};

export type MouseButtonState = {
  button: MouseButton;
  state: ButtonState;
};

export enum ButtonState {
  UP = 0,
  DOWN = 1,
}

export enum ScrollAxis {
  VERTICAL = 0,
  HORIZONTAL = 1,
}

// | message type (1) | width uint32 | height uint32 |
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#1---client-screen-spec
export type ClientScreenSpec = {
  width: number;
  height: number;
};

export type PointerData = {
  data: ImageData | boolean;
  hotspot_x?: number;
  hotspot_y?: number;
};

// | message type (2) | left uint32 | top uint32 | right uint32 | bottom uint32 | data []byte |
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#2---png-frame
export type PngFrame = {
  left: number;
  top: number;
  right: number;
  bottom: number;
  data: HTMLImageElement;
};

/**
 * `| message type (29) | data_length uint32 | data []byte |`
 *
 * `RdpFastPathPdu` is an alias to a `Uint8Array` so that it can
 * be passed into the `FastPathProcessor`'s `process` method and
 * used without copying. See [the wasm-bindgen guide].
 *
 * [the wasm-bindgen guide]: (https://rustwasm.github.io/docs/wasm-bindgen/reference/types/number-slices.html#number-slices-u8-i8-u16-i16-u32-i32-u64-i64-f32-and-f64)
 */
export type RdpFastPathPdu = Uint8Array;

// | message type (6) | length uint32 | data []byte |
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#6---clipboard-data
export type ClipboardData = {
  // TODO(isaiah): store this as a byte array
  // https://github.com/gravitational/webapps/issues/610
  data: string;
};

// | message type (31) | io_channel_id uint16 | user_channel_id uint16 | screen_width uint16 | screen_height uint16 |
export type RdpConnectionActivated = {
  ioChannelId: number;
  userChannelId: number;
  screenWidth: number;
  screenHeight: number;
};

export enum Severity {
  Info = 0,
  Warning = 1,
  Error = 2,
}

/**
 * @throws {Error} if an invalid severity is passed
 */
export function toSeverity(severity: number): Severity {
  if (severity === Severity.Info) {
    return Severity.Info;
  } else if (severity === Severity.Warning) {
    return Severity.Warning;
  } else if (severity === Severity.Error) {
    return Severity.Error;
  }

  throw new Error(`received invalid severity level: ${severity}`);
}

// | message type (28) | message_length uint32 | message []byte | severity byte
export type Alert = {
  message: string;
  severity: Severity;
};

// | message type (10) | mfa_type byte | message_length uint32 | json []byte
export type MfaJson = {
  mfaType: 'u' | 'n';
  jsonString: string;
};

// | message type (32) | scroll_lock_state byte | num_lock_state byte | caps_lock_state byte | kana_lock_state byte |
export type SyncKeys = {
  scrollLockState: ButtonState;
  numLockState: ButtonState;
  capsLockState: ButtonState;
  kanaLockState: ButtonState;
};

// | message type (11) | completion_id uint32 | directory_id uint32 | name_length uint32 | name []byte |
// TODO(isaiah): The discard here is a copy-paste error, but we need to keep it
// for now in order that the proxy stay compatible with previous versions of the wds.
export type SharedDirectoryAnnounce = {
  discard: number;
  directoryId: number;
  name: string;
};

// | message type (12) | err_code error | directory_id uint32 |
export type SharedDirectoryAcknowledge = {
  errCode: SharedDirectoryErrCode;
  directoryId: number;
};

// | message type (13) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte |
export type SharedDirectoryInfoRequest = {
  completionId: number;
  directoryId: number;
  path: string;
};

// | message type (14) | completion_id uint32 | err_code uint32 | file_system_object fso |
export type SharedDirectoryInfoResponse = {
  completionId: number;
  errCode: SharedDirectoryErrCode;
  fso: FileSystemObject;
};

// | message type (15) | completion_id uint32 | directory_id uint32 | file_type uint32 | path_length uint32 | path []byte |
export type SharedDirectoryCreateRequest = {
  completionId: number;
  directoryId: number;
  fileType: FileType;
  path: string;
};

// | message type (16) | completion_id uint32 | err_code uint32 | file_system_object fso |
export type SharedDirectoryCreateResponse = {
  completionId: number;
  errCode: SharedDirectoryErrCode;
  fso: FileSystemObject;
};

// | message type (17) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte |
export type SharedDirectoryDeleteRequest = {
  completionId: number;
  directoryId: number;
  path: string;
};

// | message type (18) | completion_id uint32 | err_code uint32 |
export type SharedDirectoryDeleteResponse = {
  completionId: number;
  errCode: SharedDirectoryErrCode;
};

// | message type (19) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte | offset uint64 | length uint32 |
export type SharedDirectoryReadRequest = {
  completionId: number;
  directoryId: number;
  path: string;
  pathLength: number;
  offset: bigint;
  length: number;
};

// | message type (20) | completion_id uint32 | err_code uint32 | read_data_length uint32 | read_data []byte |
export type SharedDirectoryReadResponse = {
  completionId: number;
  errCode: SharedDirectoryErrCode;
  readDataLength: number;
  readData: Uint8Array;
};

// | message type (21) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte | offset uint64 | write_data_length uint32 | write_data []byte |
export type SharedDirectoryWriteRequest = {
  completionId: number;
  directoryId: number;
  pathLength: number;
  path: string;
  offset: bigint;
  writeData: Uint8Array;
};

// | message type (22) | completion_id uint32 | err_code uint32 | bytes_written uint32 |
export type SharedDirectoryWriteResponse = {
  completionId: number;
  errCode: number;
  bytesWritten: number;
};

// | message type (23) | completion_id uint32 | directory_id uint32 | original_path_length uint32 | original_path []byte | new_path_length uint32 | new_path []byte |
export type SharedDirectoryMoveRequest = {
  completionId: number;
  directoryId: number;
  originalPathLength: number;
  originalPath: string;
  newPathLength: number;
  newPath: string;
};

// | message type (24) | completion_id uint32 | err_code uint32 |
export type SharedDirectoryMoveResponse = {
  completionId: number;
  errCode: SharedDirectoryErrCode;
};

// | message type (25) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte |
export type SharedDirectoryListRequest = {
  completionId: number;
  directoryId: number;
  path: string;
};

// | message type (26) | completion_id uint32 | err_code uint32 | fso_list_length uint32 | fso_list fso[] |
export type SharedDirectoryListResponse = {
  completionId: number;
  errCode: SharedDirectoryErrCode;
  fsoList: FileSystemObject[];
};

// | message type (33) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte | end_of_file uint32 |
export type SharedDirectoryTruncateRequest = {
  completionId: number;
  directoryId: number;
  path: string;
  endOfFile: number;
};

// | message type (34) | completion_id uint32 | err_code uint32 |
export type SharedDirectoryTruncateResponse = {
  completionId: number;
  errCode: SharedDirectoryErrCode;
};

// | last_modified uint64 | size uint64 | file_type uint32 | is_empty bool | path_length uint32 | path byte[] |
export type FileSystemObject = {
  lastModified: bigint;
  size: bigint;
  fileType: FileType;
  isEmpty: boolean;
  path: string;
};

export enum SharedDirectoryErrCode {
  // nil (no error, operation succeeded)
  Nil = 0,
  // operation failed
  Failed = 1,
  // resource does not exist
  DoesNotExist = 2,
  // resource already exists
  AlreadyExists = 3,
}

export enum FileType {
  File = 0,
  Directory = 1,
}

// | message type (35) | client_latency uint32 | server_latency uint32 |
export type LatencyStats = {
  client: number;
  server: number;
};

export type ServerHello = {
  clipboardSupport: boolean;
  activationEvent: RdpConnectionActivated;
};

export type ClientHello = {
  keyboardLayout: number;
  screenSpec: ClientScreenSpec;
};

export type MfaResponse = {
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
};

// We're not allowed to import from teleport/services/mfa/types, so just
// define the MfaSsoChallenge structure here.
type MfaSsoChallenge = {
  channelId: string;
  redirectUrl: string;
  requestId: string;
  device: {
    connectorId: string;
    connectorType: string;
    displayName: string;
  };
};

function toSharedDirectoryErrCode(errCode: number): SharedDirectoryErrCode {
  if (!(errCode in SharedDirectoryErrCode)) {
    throw new Error(`attempted to convert invalid error code ${errCode}`);
  }

  return errCode as SharedDirectoryErrCode;
}

// Implement a set of encoding methods that the client will use
// to send outbound messages
export interface Codec {
  // Convert incoming messages into a common CodecResult
  decodeMessage(buffer: ArrayBufferLike): DecodedMessage;

  // Shared TDP/TDPB Messages
  encodeInitialMessages(
    spec?: ClientScreenSpec,
    keyboardLayout?: number
  ): Message[];
  encodeMouseMove(x: number, y: number): Message;
  encodeMouseButton(button: MouseButton, state: ButtonState): Message;
  encodeSyncKeys(syncKeys: SyncKeys): Message;
  encodeClipboardData(clipboardData: ClipboardData): Message;
  encodeKeyboardInput(code: string, state: ButtonState): Message[];
  encodeMouseWheelScroll(axis: ScrollAxis, delta: number): Message;
  encodeClientScreenSpec(spec: ClientScreenSpec): Message;
  encodeMfaJson(mfaJson: MfaResponse): Message;
  encodeSharedDirectoryInfoResponse(res: SharedDirectoryInfoResponse): Message;
  encodeSharedDirectoryReadResponse(res: SharedDirectoryReadResponse): Message;
  encodeSharedDirectoryMoveResponse(res: SharedDirectoryMoveResponse): Message;
  encodeSharedDirectoryListResponse(res: SharedDirectoryListResponse): Message;
  encodeRdpResponsePdu(responseFrame: ArrayBufferLike): Message;
  encodeSharedDirectoryAnnounce(announce: SharedDirectoryAnnounce): Message;
  encodeSharedDirectoryCreateResponse(
    resp: SharedDirectoryCreateResponse
  ): Message;
  encodeSharedDirectoryDeleteResponse(
    resp: SharedDirectoryDeleteResponse
  ): Message;
  encodeSharedDirectoryWriteResponse(
    resp: SharedDirectoryWriteResponse
  ): Message;
  encodeSharedDirectoryTruncateResponse(
    resp: SharedDirectoryTruncateResponse
  ): Message;
}

export type DecodedMessage =
  | { kind: 'pngFrame'; data: PngFrame }
  | { kind: 'rdpConnectionActivated'; data: RdpConnectionActivated }
  | { kind: 'serverHello'; data: ServerHello }
  | { kind: 'rdpFastPathPdu'; data: RdpFastPathPdu }
  | { kind: 'clipboardData'; data: ClipboardData }
  | { kind: 'tdpAlert'; data: Alert }
  | { kind: 'mfaChallenge'; data: MfaJson }
  | { kind: 'sharedDirectoryAcknowledge'; data: SharedDirectoryAcknowledge }
  | { kind: 'sharedDirectoryInfoRequest'; data: SharedDirectoryInfoRequest }
  | { kind: 'sharedDirectoryCreateRequest'; data: SharedDirectoryCreateRequest }
  | { kind: 'sharedDirectoryDeleteRequest'; data: SharedDirectoryDeleteRequest }
  | { kind: 'sharedDirectoryReadRequest'; data: SharedDirectoryReadRequest }
  | { kind: 'sharedDirectoryWriteRequest'; data: SharedDirectoryWriteRequest }
  | { kind: 'sharedDirectoryMoveRequest'; data: SharedDirectoryMoveRequest }
  | { kind: 'sharedDirectoryListRequest'; data: SharedDirectoryListRequest }
  | {
      kind: 'sharedDirectoryTruncateRequest';
      data: SharedDirectoryTruncateRequest;
    }
  | { kind: 'latencyStats'; data: LatencyStats }
  | { kind: 'tdpbUpgrade'; data: null }
  | { kind: 'clientScreenSpec'; data: ClientScreenSpec }
  | { kind: 'mouseButton'; data: MouseButtonState }
  | { kind: 'mouseMove'; data: MouseMove }
  | { kind: 'unsupported'; data: string }
  | { kind: 'unknown'; data: unknown };

// Assists with type narrowing on SharedDirectory messages by
// excluding the 'undefined' variant of the generated oneof.
type OneofWithValue<T extends { oneofKind?: string }> = Exclude<
  T,
  { oneofKind: undefined }
>;

function hasOneof<T extends { oneofKind?: string }>(
  value: T
): value is OneofWithValue<T> {
  return value.oneofKind !== undefined;
}

export class TdpbCodec implements Codec {
  encoder = new window.TextEncoder();
  decoder = new window.TextDecoder();
  private logger = new Logger('TDPBCodec');

  // asBase64Url creates a data:image uri from the png data part of a PNG_FRAME tdp message.
  private asBase64Url(buffer: ArrayBufferLike, offset: number): string {
    return `data:image/png;base64,${arrayBufferToBase64(buffer.slice(offset))}`;
  }

  protected marshal(msg: Envelope['payload']): Message {
    const marshalledMessage = Envelope.toBinary({ payload: msg });
    const len = marshalledMessage.byteLength;
    let buf = new Uint8Array(len + 4);
    let outbuf = new DataView(buf.buffer);
    outbuf.setUint32(0, len, false /* big endian */);
    buf.set(marshalledMessage, 4);
    return buf.buffer;
  }

  private processSharedDirectoryRequest(
    completionId: number,
    directoryId: number,
    op: SharedDirectoryRequest['operation']
  ): DecodedMessage {
    // Exclude 'oneOfKind: undefined'
    // Possibly due to 'strictNullChecks' being disabled, the compiler can't seem to narrow
    // the discriminated union using control flow analysis alone.
    if (!hasOneof(op)) {
      this.logger.debug('unknown shared directory operation');
      return { kind: 'unknown', data: null };
    }

    switch (op.oneofKind) {
      case 'create':
        return {
          kind: 'sharedDirectoryCreateRequest',
          data: {
            directoryId,
            completionId,
            ...op.create,
          },
        };
      case 'delete':
        return {
          kind: 'sharedDirectoryDeleteRequest',
          data: {
            directoryId,
            completionId,
            ...op.delete,
          },
        };
      case 'info':
        return {
          kind: 'sharedDirectoryInfoRequest',
          data: {
            directoryId,
            completionId,
            ...op.info,
          },
        };
      case 'list':
        return {
          kind: 'sharedDirectoryListRequest',
          data: {
            directoryId,
            completionId,
            ...op.list,
          },
        };
      case 'move':
        return {
          kind: 'sharedDirectoryMoveRequest',
          data: {
            directoryId,
            completionId,
            originalPathLength: op.move.originalPath.length,
            newPathLength: op.move.newPath.length,
            ...op.move,
          },
        };
      case 'read':
        return {
          kind: 'sharedDirectoryReadRequest',
          data: {
            directoryId,
            completionId,
            pathLength: op.read.path.length,
            ...op.read,
          },
        };
      case 'truncate':
        return {
          kind: 'sharedDirectoryTruncateRequest',
          data: {
            directoryId,
            completionId,
            ...op.truncate,
          },
        };
      case 'write':
        return {
          kind: 'sharedDirectoryWriteRequest',
          data: {
            directoryId,
            completionId,
            pathLength: op.write.path.length,
            writeData: op.write.data,
            ...op.write,
          },
        };
      default:
        const exhaustiveCheck: never = op;
        throw new Error(`Unhandled operation: ${exhaustiveCheck}`);
    }
  }

  decodeMessage(buffer: ArrayBufferLike): DecodedMessage {
    // Note: TDPB messages are prefixed with a single big endian uint32 containing their length
    // to act as a simple framing mechanism. This client connects via a websocket which makes this
    // framing header redundant. Still, to avoid leaking TDPB details to the client implementation,
    // we'll quietly discard that header here.
    const envelope = Envelope.fromBinary(
      new Uint8Array(buffer, 4 /* ignore TDPB header */),
      { readUnknownField: true }
    );

    if (!envelope.payload) {
      // Either the server sent an empty Envelope, or the payload contains a
      // new message type that this implementation doesn't understand.
      this.logger.debug('received empty or unknown message payload');
      return { kind: 'unknown', data: null };
    }

    switch (envelope.payload.oneofKind) {
      case 'serverHello':
        return {
          kind: 'rdpConnectionActivated',
          data: envelope.payload.serverHello.activationSpec,
        };
      case 'pngFrame':
        const frame = envelope.payload.pngFrame;
        let data = new Image();
        data.src = this.asBase64Url(frame.data.buffer, 0);
        return {
          kind: 'pngFrame',
          data: {
            top: frame.coordinates.top,
            left: frame.coordinates.left,
            bottom: frame.coordinates.bottom,
            right: frame.coordinates.right,
            data: data,
          },
        };
      case 'fastPathPdu':
        return {
          kind: 'rdpFastPathPdu',
          data: envelope.payload.fastPathPdu.pdu,
        };
      case 'alert':
        let { message, severity } = envelope.payload.alert;
        return {
          kind: 'tdpAlert',
          data: { message: message, severity: severity.valueOf() - 1 },
        };
      case 'clipboardData':
        return {
          kind: 'clipboardData',
          data: {
            data: this.decoder.decode(envelope.payload.clipboardData.data),
          },
        };
      case 'sharedDirectoryAcknowledge':
        const { errorCode: errCode, directoryId } =
          envelope.payload.sharedDirectoryAcknowledge;
        return {
          kind: 'sharedDirectoryAcknowledge',
          data: {
            errCode,
            directoryId,
          },
        };
      case 'sharedDirectoryRequest':
        return this.processSharedDirectoryRequest(
          envelope.payload.sharedDirectoryRequest.completionId,
          envelope.payload.sharedDirectoryRequest.directoryId,
          envelope.payload.sharedDirectoryRequest.operation
        );
      case 'latencyStats':
        const stats = envelope.payload.latencyStats;
        return {
          kind: 'latencyStats',
          data: {
            client: stats.clientLatencyMs,
            server: stats.serverLatencyMs,
          },
        };
      case 'mfa':
        const mfa = envelope.payload.mfa;
        const challenge = mfa.challenge;

        if (!challenge) {
          throw new Error('received empty MFA challenge');
        }

        let challengeData: {
          webauthn_challenge?: {
            publicKey: PublicKeyCredentialRequestOptionsJSON;
          };
          sso_challenge?: MfaSsoChallenge;
        } = {};

        if (challenge.webauthnChallenge) {
          challengeData.webauthn_challenge = this.toMfaWebauthnChallenge(
            challenge.webauthnChallenge
          );
        }

        if (challenge.ssoChallenge) {
          challengeData.sso_challenge = this.toMfaSsoChallenge(
            challenge.ssoChallenge,
            mfa.channelId
          );
        }

        if (
          challengeData.sso_challenge === undefined &&
          challengeData.webauthn_challenge === undefined
        ) {
          throw new Error(
            'Invalid MFA type - Only SSO or Webauthn are supported'
          );
        }

        return {
          kind: 'mfaChallenge',
          data: {
            mfaType: 'n',
            jsonString: JSON.stringify(challengeData),
          },
        };

      default:
        return { kind: 'unsupported', data: envelope.payload.oneofKind };
    }
  }

  toMfaWebauthnChallenge(challenge: CredentialAssertion): {
    publicKey: PublicKeyCredentialRequestOptionsJSON;
  } {
    return {
      publicKey: {
        challenge: btoa(String.fromCharCode(...challenge.publicKey.challenge)),
        rpId: challenge.publicKey.rpId,
        timeout: Number(challenge.publicKey.timeoutMs),
        userVerification: challenge.publicKey.userVerification,
        extensions: challenge.publicKey.extensions,
        allowCredentials: challenge.publicKey.allowCredentials.map(
          (cred): PublicKeyCredentialDescriptorJSON => {
            return {
              id: btoa(String.fromCharCode(...cred.id)),
              type: cred.type,
            };
          }
        ),
      },
    };
  }

  toMfaSsoChallenge(challenge: SSOChallenge, name: string): MfaSsoChallenge {
    const connectorType = challenge.device?.connectorType;
    switch (connectorType) {
      case 'oidc':
      case 'saml':
      case 'github':
        break;
      default:
        throw new Error(
          'invalid MFA connector type: ' + challenge.device?.connectorType
        );
    }

    return {
      channelId: name,
      redirectUrl: challenge.redirectUrl,
      requestId: challenge.requestId,
      device: {
        connectorId: challenge.device.connectorId,
        connectorType: connectorType,
        displayName: challenge.device.displayName,
      },
    };
  }

  encodeInitialMessages(
    spec?: ClientScreenSpec,
    keyboardLayout?: number
  ): Message[] {
    // Send a Hello message regardless of whether or we've been provided a screenspec or keyboardlayout
    const hello = this.marshal({
      oneofKind: 'clientHello',
      clientHello: tdpbClientHello.create({
        screenSpec: spec,
        keyboardLayout: keyboardLayout,
      }),
    });
    return [hello];
  }

  encodeClientScreenSpec(spec: ClientScreenSpec): Message {
    return this.marshal({
      oneofKind: 'clientScreenSpec',
      clientScreenSpec: spec,
    });
  }

  encodeRdpResponsePdu(response: ArrayBufferLike): Message {
    return this.marshal({
      oneofKind: 'rdpResponsePdu',
      rdpResponsePdu: { response: new Uint8Array(response) },
    });
  }

  encodeMouseMove(x: number, y: number): Message {
    return this.marshal({ oneofKind: 'mouseMove', mouseMove: { x, y } });
  }
  encodeMouseButton(button: MouseButton, state: ButtonState): Message {
    return this.marshal({
      oneofKind: 'mouseButton',
      mouseButton: { button: button + 1, pressed: state == ButtonState.DOWN },
    });
  }

  encodeKeyboardInput(code: string, state: ButtonState): Message[] {
    const scancodes = KEY_SCANCODES[code];
    if (!scancodes) {
      this.logger.warn(`unsupported key code: ${code}`);
      return [];
    }
    return scancodes.map(scancode => this.encodeScancode(scancode, state));
  }

  private encodeScancode(scancode: number, state: ButtonState): Message {
    return this.marshal({
      oneofKind: 'keyboardButton',
      keyboardButton: { keyCode: scancode, pressed: state == ButtonState.DOWN },
    });
  }

  encodeSyncKeys(syncKeys: SyncKeys): Message {
    return this.marshal({
      oneofKind: 'syncKeys',
      syncKeys: {
        scrollLockPressed: syncKeys.scrollLockState == ButtonState.DOWN,
        numLockState: syncKeys.numLockState == ButtonState.DOWN,
        capsLockState: syncKeys.capsLockState == ButtonState.DOWN,
        kanaLockState: syncKeys.kanaLockState == ButtonState.DOWN,
      },
    });
  }

  encodeClipboardData(clipboardData: ClipboardData) {
    return this.marshal({
      oneofKind: 'clipboardData',
      clipboardData: { data: this.encoder.encode(clipboardData.data) },
    });
  }

  encodeMouseWheelScroll(axis: ScrollAxis, delta: number): Message {
    return this.marshal({
      oneofKind: 'mouseWheel',
      mouseWheel: { axis: axis.valueOf() + 1, delta: Math.round(delta) },
    });
  }

  encodeMfaJson(mfaJson: MfaResponse): Message {
    if (mfaJson.webauthn_response) {
      const response = AuthenticateResponse.create({
        response: {
          oneofKind: 'webauthn',
          webauthn: {
            type: mfaJson.webauthn_response.type,
            // The MFA emitter base64 url encodes the buffers returned by webauthn APIs.
            // We unfortunately need to reverse that by decoding back to raw buffers.
            rawId: new Uint8Array(
              base64urlToBuffer(mfaJson.webauthn_response.rawId)
            ),
            response: {
              userHandle: new Uint8Array(
                base64urlToBuffer(mfaJson.webauthn_response.response.userHandle)
              ),
              clientDataJson: new Uint8Array(
                base64urlToBuffer(
                  mfaJson.webauthn_response.response.clientDataJSON
                )
              ),
              authenticatorData: new Uint8Array(
                base64urlToBuffer(
                  mfaJson.webauthn_response.response.authenticatorData
                )
              ),
              signature: new Uint8Array(
                base64urlToBuffer(mfaJson.webauthn_response.response.signature)
              ),
            },
            extensions: {
              appId: mfaJson.webauthn_response.extensions.appid,
            },
          },
        },
      });

      return this.marshal({
        oneofKind: 'mfa',
        mfa: MFA.create({ authenticationResponse: response }),
      });
    } else if (mfaJson.sso_response) {
      const response = AuthenticateResponse.create({
        name: '',
        response: {
          oneofKind: 'sso',
          sso: mfaJson.sso_response,
        },
      });
      return this.marshal({
        oneofKind: 'mfa',
        mfa: MFA.create({ authenticationResponse: response }),
      });
    }

    throw new Error('Invalid MFA response. Must be Webauthn or SSO');
  }

  encodeSharedDirectoryInfoResponse(res: SharedDirectoryInfoResponse): Message {
    return this.marshal({
      oneofKind: 'sharedDirectoryResponse',
      sharedDirectoryResponse: SharedDirectoryResponse.create({
        completionId: res.completionId,
        errorCode: res.errCode,
        operation: {
          oneofKind: 'info',
          info: { fso: res.fso },
        },
      }),
    });
  }

  encodeSharedDirectoryReadResponse(res: SharedDirectoryReadResponse): Message {
    return this.marshal({
      oneofKind: 'sharedDirectoryResponse',
      sharedDirectoryResponse: SharedDirectoryResponse.create({
        completionId: res.completionId,
        errorCode: res.errCode,
        operation: {
          oneofKind: 'read',
          read: { data: res.readData },
        },
      }),
    });
  }

  encodeSharedDirectoryMoveResponse(res: SharedDirectoryMoveResponse): Message {
    return this.marshal({
      oneofKind: 'sharedDirectoryResponse',
      sharedDirectoryResponse: SharedDirectoryResponse.create({
        completionId: res.completionId,
        errorCode: res.errCode,
        operation: {
          oneofKind: 'move',
          move: {},
        },
      }),
    });
  }

  encodeSharedDirectoryListResponse(res: SharedDirectoryListResponse): Message {
    return this.marshal({
      oneofKind: 'sharedDirectoryResponse',
      sharedDirectoryResponse: SharedDirectoryResponse.create({
        completionId: res.completionId,
        errorCode: res.errCode,
        operation: {
          oneofKind: 'list',
          list: {
            fsoList: res.fsoList,
          },
        },
      }),
    });
  }

  encodeSharedDirectoryAnnounce(announce: SharedDirectoryAnnounce): Message {
    return this.marshal({
      oneofKind: 'sharedDirectoryAnnounce',
      sharedDirectoryAnnounce: announce,
    });
  }

  encodeSharedDirectoryCreateResponse(
    resp: SharedDirectoryCreateResponse
  ): Message {
    return this.marshal({
      oneofKind: 'sharedDirectoryResponse',
      sharedDirectoryResponse: SharedDirectoryResponse.create({
        completionId: resp.completionId,
        errorCode: resp.errCode,
        operation: {
          oneofKind: 'create',
          create: {
            fso: resp.fso,
          },
        },
      }),
    });
  }

  encodeSharedDirectoryDeleteResponse(
    resp: SharedDirectoryDeleteResponse
  ): Message {
    return this.marshal({
      oneofKind: 'sharedDirectoryResponse',
      sharedDirectoryResponse: SharedDirectoryResponse.create({
        completionId: resp.completionId,
        errorCode: resp.errCode,
        operation: {
          oneofKind: 'delete',
          delete: {},
        },
      }),
    });
  }

  encodeSharedDirectoryWriteResponse(
    resp: SharedDirectoryWriteResponse
  ): Message {
    return this.marshal({
      oneofKind: 'sharedDirectoryResponse',
      sharedDirectoryResponse: SharedDirectoryResponse.create({
        completionId: resp.completionId,
        errorCode: resp.errCode,
        operation: {
          oneofKind: 'write',
          write: {
            bytesWritten: resp.bytesWritten,
          },
        },
      }),
    });
  }

  encodeSharedDirectoryTruncateResponse(
    resp: SharedDirectoryTruncateResponse
  ): Message {
    return this.marshal({
      oneofKind: 'sharedDirectoryResponse',
      sharedDirectoryResponse: SharedDirectoryResponse.create({
        completionId: resp.completionId,
        errorCode: resp.errCode,
        operation: {
          oneofKind: 'truncate',
          truncate: {},
        },
      }),
    });
  }

  encodeClientHello(hello: ClientHello): Message {
    return this.marshal({
      oneofKind: 'clientHello',
      clientHello: tdpbClientHello.create({
        screenSpec: hello.screenSpec,
        keyboardLayout: hello.keyboardLayout,
      }),
    });
  }
}

// TdpCodec provides an api for encoding and decoding teleport desktop access protocol messages
// [1] Buffers in TdpCodec are manipulated as DataView's [2] in order to give us low level control
// of endianness (defaults to big endian, which is what we want), as opposed to using *Array
// objects [3] which use the platform's endianness.
// [1] https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md
// [2] https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/DataView
// [3] https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Int32Array
//
// This legacy protocol is superseded by TDPB (see TdpbCodec) and remains here for backwards
// compatibility.
// TODO(rhammmonds): DELETE IN v20.0.0
export class TdpCodec implements Codec {
  encoder = new window.TextEncoder();
  decoder = new window.TextDecoder();
  private logger = new Logger('TDPCodec');

  decodeMessage(buffer: ArrayBufferLike): DecodedMessage {
    const messageType = this.decodeMessageType(buffer);
    switch (messageType) {
      case MessageType.PNG_FRAME:
        return {
          kind: 'pngFrame',
          data: this.decodePngFrame(buffer, (frame: PngFrame) => {
            void frame;
          }),
        };
      case MessageType.PNG2_FRAME:
        return {
          kind: 'pngFrame',
          data: this.decodePng2Frame(buffer, (frame: PngFrame) => {
            void frame;
          }),
        };
      case MessageType.RDP_CONNECTION_ACTIVATED:
        return {
          kind: 'rdpConnectionActivated',
          data: this.decodeRdpConnectionActivated(buffer),
        };
      case MessageType.RDP_FASTPATH_PDU:
        return {
          kind: 'rdpFastPathPdu',
          data: this.decodeRdpFastPathPdu(buffer),
        };
      case MessageType.CLIPBOARD_DATA:
        return {
          kind: 'clipboardData',
          data: this.decodeClipboardData(buffer),
        };
      case MessageType.ERROR:
        throw new Error(this.decodeErrorMessage(buffer));
      case MessageType.ALERT:
        return { kind: 'tdpAlert', data: this.decodeAlert(buffer) };
      case MessageType.MFA_JSON:
        return { kind: 'mfaChallenge', data: this.decodeMfaJson(buffer) };
      case MessageType.SHARED_DIRECTORY_ACKNOWLEDGE:
        return {
          kind: 'sharedDirectoryAcknowledge',
          data: this.decodeSharedDirectoryAcknowledge(buffer),
        };
      case MessageType.SHARED_DIRECTORY_INFO_REQUEST:
        return {
          kind: 'sharedDirectoryInfoRequest',
          data: this.decodeSharedDirectoryInfoRequest(buffer),
        };
      case MessageType.SHARED_DIRECTORY_CREATE_REQUEST:
        return {
          kind: 'sharedDirectoryCreateRequest',
          data: this.decodeSharedDirectoryCreateRequest(buffer),
        };
      case MessageType.SHARED_DIRECTORY_DELETE_REQUEST:
        return {
          kind: 'sharedDirectoryDeleteRequest',
          data: this.decodeSharedDirectoryDeleteRequest(buffer),
        };
      case MessageType.SHARED_DIRECTORY_READ_REQUEST:
        return {
          kind: 'sharedDirectoryReadRequest',
          data: this.decodeSharedDirectoryReadRequest(buffer),
        };
      case MessageType.SHARED_DIRECTORY_WRITE_REQUEST:
        return {
          kind: 'sharedDirectoryWriteRequest',
          data: this.decodeSharedDirectoryWriteRequest(buffer),
        };
      case MessageType.SHARED_DIRECTORY_MOVE_REQUEST:
        return {
          kind: 'sharedDirectoryMoveRequest',
          data: this.decodeSharedDirectoryMoveRequest(buffer),
        };
      case MessageType.SHARED_DIRECTORY_LIST_REQUEST:
        return {
          kind: 'sharedDirectoryListRequest',
          data: this.decodeSharedDirectoryListRequest(buffer),
        };
      case MessageType.SHARED_DIRECTORY_TRUNCATE_REQUEST:
        return {
          kind: 'sharedDirectoryTruncateRequest',
          data: this.decodeSharedDirectoryTruncateRequest(buffer),
        };
      case MessageType.LATENCY_STATS:
        return { kind: 'latencyStats', data: this.decodeLatencyStats(buffer) };
      case MessageType.TDPB_UPGRADE:
        return { kind: 'tdpbUpgrade', data: null };
      // Needed by the player client
      case MessageType.CLIENT_SCREEN_SPEC:
        return {
          kind: 'clientScreenSpec',
          data: this.decodeClientScreenSpec(buffer),
        };
      case MessageType.MOUSE_BUTTON:
        return { kind: 'mouseButton', data: this.decodeMouseButton(buffer) };
      case MessageType.MOUSE_MOVE:
        return { kind: 'mouseMove', data: this.decodeMouseMove(buffer) };
      default:
        throw new Error(`received unsupported message type", ${messageType}`);
    }
  }

  decodeMouseButton(buffer: ArrayBufferLike): MouseButtonState {
    const view = new DataView(buffer);
    const isMouseButton = (n: number): MouseButton | undefined => {
      if (n === 0 || n === 1 || n === 2) {
        return n;
      }
      return undefined;
    };

    const value = view.getUint8(1);
    const buttonNum = isMouseButton(value);
    if (buttonNum === undefined) {
      throw Error(
        `MouseButton message contains invalid button value: ${value}`
      );
    }
    return {
      button: buttonNum,
      state: view.getUint8(2),
    };
  }

  decodeMouseMove(buffer: ArrayBufferLike): MouseMove {
    const view = new DataView(buffer);
    return {
      x: view.getUint8(1),
      y: view.getUint8(2),
    };
  }

  encodeInitialMessages(
    spec: ClientScreenSpec,
    keyboardLayout?: number
  ): Message[] {
    let messages: Message[] = [];
    if (spec) {
      messages.push(this.encodeClientScreenSpec(spec));
    }

    // 0 represents the default keyboard layout from the point of view of the
    // remote desktop, so there is no need to send this message. Additionally,
    // for clients (Connect) that don't support specifying a keyboard layout
    // and WDS versions that don't support this feature (v17 and earlier), this
    // avoids the connection crashing.
    if (keyboardLayout !== undefined && keyboardLayout !== 0) {
      messages.push(this.encodeClientKeyboardLayout(keyboardLayout));
    } else {
      // The proxy expects two messasges (client screen spec and keyboard layout)
      // before it will initialise the connection to WDS. If no keyboard layout
      // is sent, the proxy will hang waiting for a second message that won't
      // arrive. To get around this we send another client screen spec.
      // TODO (danielashare): Remove this once proxy doesn't block on
      // keyboardLayout.
      if (spec) {
        messages.push(this.encodeClientScreenSpec(spec));
      }
    }
    return messages;
  }

  // encodeClientScreenSpec encodes the client's screen spec.
  // | message type (1) | width uint32 | height uint32 |
  // https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#1---client-screen-spec
  encodeClientScreenSpec(spec: ClientScreenSpec): Message {
    const { width, height } = spec;
    const buffer = new ArrayBuffer(9);
    const view = new DataView(buffer);
    view.setUint8(0, MessageType.CLIENT_SCREEN_SPEC);
    view.setUint32(1, width);
    view.setUint32(5, height);
    return buffer;
  }

  // decodeClientScreenSpec decodes a raw tdp CLIENT_SCREEN_SPEC message
  // | message type (1) | width uint32 | height uint32 |
  // https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#1---client-screen-spec
  decodeClientScreenSpec(buffer: ArrayBufferLike): ClientScreenSpec {
    let dv = new DataView(buffer);
    return {
      width: dv.getUint32(1),
      height: dv.getUint32(5),
    };
  }

  // encodeMouseMove encodes a mouse move event.
  // | message type (3) | x uint32 | y uint32 |
  encodeMouseMove(x: number, y: number): Message {
    const buffer = new ArrayBuffer(9);
    const view = new DataView(buffer);
    view.setUint8(0, MessageType.MOUSE_MOVE);
    view.setUint32(1, x);
    view.setUint32(5, y);
    return buffer;
  }

  // encodeMouseButton encodes a mouse button action.
  // | message type (4) | button byte | state byte |
  encodeMouseButton(button: MouseButton, state: ButtonState): Message {
    const buffer = new ArrayBuffer(3);
    const view = new DataView(buffer);
    view.setUint8(0, MessageType.MOUSE_BUTTON);
    view.setUint8(1, button);
    view.setUint8(2, state);
    return buffer;
  }

  // encodeKeyboardInput encodes a keyboard action.
  // Returns an empty array if an unsupported code is passed.
  // | message type (5) | key_code uint32 | state byte |
  encodeKeyboardInput(code: string, state: ButtonState): Message[] {
    const scancodes = KEY_SCANCODES[code];
    if (!scancodes) {
      this.logger.warn(`unsupported key code: ${code}`);
      return [];
    }

    return scancodes.map(scancode => this.encodeScancode(scancode, state));
  }

  private encodeScancode(scancode: number, state: ButtonState): Message {
    const buffer = new ArrayBuffer(6);
    const view = new DataView(buffer);
    view.setUint8(0, MessageType.KEYBOARD_BUTTON);
    view.setUint32(1, scancode);
    view.setUint8(5, state);
    return buffer;
  }

  // encodeSyncKeys synchronizes the state of keyboard's modifier keys (caps lock)
  // and resets the server key state to all keys up.
  // | message type (32) | scroll_lock_state byte | num_lock_state byte | caps_lock_state byte | kana_lock_state byte |
  encodeSyncKeys(syncKeys: SyncKeys): Message {
    const buffer = new ArrayBuffer(BYTE_LEN * 5);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset++, MessageType.SYNC_KEYS);
    view.setUint8(offset++, syncKeys.scrollLockState);
    view.setUint8(offset++, syncKeys.numLockState);
    view.setUint8(offset++, syncKeys.capsLockState);
    view.setUint8(offset++, syncKeys.kanaLockState);

    return buffer;
  }

  // _encodeStringMessage encodes a message of the form
  // | message type (N) | length uint32 | data []byte |
  _encodeStringMessage(messageType: MessageType, data: string) {
    const dataUtf8array = this.encoder.encode(data);

    // bufLen is 1 byte for the `message type`,
    // 4 bytes for the `length uint32`,
    // and enough bytes for the full `data []byte`
    const bufLen = BYTE_LEN + UINT_32_LEN + dataUtf8array.length;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset++, messageType);
    view.setUint32(offset, dataUtf8array.length);
    offset += UINT_32_LEN;
    dataUtf8array.forEach(byte => {
      view.setUint8(offset++, byte);
    });

    return buffer;
  }

  // encodeClipboardData encodes clipboard data
  // | message type (6) | length uint32 | data []byte |
  // https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#6---clipboard-data
  encodeClipboardData(clipboardData: ClipboardData) {
    return this._encodeStringMessage(
      MessageType.CLIPBOARD_DATA,
      clipboardData.data
    );
  }

  // encodeUsername encodes a username to log in to the remote desktop with.
  // | message type (7) | username_length uint32 | username []byte |
  encodeUsername(username: string): Message {
    return this._encodeStringMessage(MessageType.CLIENT_USERNAME, username);
  }

  // encodeClientKeyboardLayout encodes a keyboard layout to use on the remote desktop.
  // | messsage type (37) | length uint32 | keyboard_layout uint32 |
  encodeClientKeyboardLayout(keyboardLayout: number): Message {
    const buffer = new ArrayBuffer(BYTE_LEN + UINT_32_LEN + UINT_32_LEN);
    const view = new DataView(buffer);
    let offset = 0;
    view.setUint8(offset, MessageType.CLIENT_KEYBOARD_LAYOUT);
    offset += BYTE_LEN;
    view.setUint32(offset, 4); // length of uint32 keyboard layout
    offset += UINT_32_LEN;
    view.setUint32(offset, keyboardLayout);
    return buffer;
  }

  // encodeMouseWheelScroll encodes a mouse wheel scroll event.
  // on vertical axis, positive delta is up, negative delta is down
  // on horizontal axis, positive delta is left, negative delta is right
  // | message type (8) | axis byte | delta int16
  encodeMouseWheelScroll(axis: ScrollAxis, delta: number): Message {
    const buffer = new ArrayBuffer(4);
    const view = new DataView(buffer);
    view.setUint8(0, MessageType.MOUSE_WHEEL_SCROLL);
    view.setUint8(1, axis);
    view.setUint16(2, delta);
    return buffer;
  }

  // | message type (10) | mfa_type byte | message_length uint32 | json []byte
  encodeMfaJson(mfaJson: MfaResponse): Message {
    const dataUtf8array = this.encoder.encode(JSON.stringify(mfaJson));

    const bufLen = BYTE_LEN + BYTE_LEN + UINT_32_LEN + dataUtf8array.length;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset++, MessageType.MFA_JSON);
    view.setUint8(offset++, 'n'.charCodeAt(0));
    view.setUint32(offset, dataUtf8array.length);
    offset += UINT_32_LEN;
    dataUtf8array.forEach(byte => {
      view.setUint8(offset++, byte);
    });

    return buffer;
  }

  // | message type (11) | completion_id uint32 | directory_id uint32 | name_length uint32 | name []byte |
  encodeSharedDirectoryAnnounce(
    sharedDirAnnounce: SharedDirectoryAnnounce
  ): Message {
    const dataUtf8array = this.encoder.encode(sharedDirAnnounce.name);

    const bufLen = BYTE_LEN + 3 * UINT_32_LEN + dataUtf8array.length;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset++, MessageType.SHARED_DIRECTORY_ANNOUNCE);
    // TODO(isaiah): The discard here is a copy-paste error, but we need to keep it
    // for now in order that the proxy stay compatible with previous versions of the wds.
    view.setUint32(offset, sharedDirAnnounce.discard);
    offset += UINT_32_LEN;
    view.setUint32(offset, sharedDirAnnounce.directoryId);
    offset += UINT_32_LEN;
    view.setUint32(offset, dataUtf8array.length);
    offset += UINT_32_LEN;
    dataUtf8array.forEach(byte => {
      view.setUint8(offset++, byte);
    });

    return buffer;
  }

  // | message type (14) | completion_id uint32 | err_code uint32 | file_system_object fso |
  encodeSharedDirectoryInfoResponse(res: SharedDirectoryInfoResponse): Message {
    const bufLenSansFso = BYTE_LEN + 2 * UINT_32_LEN;
    const bufferSansFso = new ArrayBuffer(bufLenSansFso);
    const view = new DataView(bufferSansFso);
    let offset = 0;

    view.setUint8(offset++, MessageType.SHARED_DIRECTORY_INFO_RESPONSE);
    view.setUint32(offset, res.completionId);
    offset += UINT_32_LEN;
    view.setUint32(offset, res.errCode);
    offset += UINT_32_LEN;

    const fsoBuffer = this.encodeFileSystemObject(res.fso);

    // https://gist.github.com/72lions/4528834?permalink_comment_id=2395442#gistcomment-2395442
    return new Uint8Array([
      ...new Uint8Array(bufferSansFso),
      ...new Uint8Array(fsoBuffer),
    ]).buffer;
  }

  // | message type (16) | completion_id uint32 | err_code uint32 | file_system_object fso |
  encodeSharedDirectoryCreateResponse(
    res: SharedDirectoryCreateResponse
  ): Message {
    const bufLenSansFso = BYTE_LEN + 2 * UINT_32_LEN;
    const bufferSansFso = new ArrayBuffer(bufLenSansFso);
    const view = new DataView(bufferSansFso);
    let offset = 0;

    view.setUint8(offset, MessageType.SHARED_DIRECTORY_CREATE_RESPONSE);
    offset += BYTE_LEN;
    view.setUint32(offset, res.completionId);
    offset += UINT_32_LEN;
    view.setUint32(offset, res.errCode);
    offset += UINT_32_LEN;

    const fsoBuffer = this.encodeFileSystemObject(res.fso);

    // https://gist.github.com/72lions/4528834?permalink_comment_id=2395442#gistcomment-2395442
    return new Uint8Array([
      ...new Uint8Array(bufferSansFso),
      ...new Uint8Array(fsoBuffer),
    ]).buffer;
  }

  // | message type (18) | completion_id uint32 | err_code uint32 |
  encodeSharedDirectoryDeleteResponse(
    res: SharedDirectoryDeleteResponse
  ): Message {
    return this.encodeGenericResponse(
      MessageType.SHARED_DIRECTORY_DELETE_RESPONSE,
      res
    );
  }

  // | message type (20) | completion_id uint32 | err_code uint32 | read_data_length uint32 | read_data []byte |
  encodeSharedDirectoryReadResponse(res: SharedDirectoryReadResponse): Message {
    const bufLen = BYTE_LEN + 3 * UINT_32_LEN + BYTE_LEN * res.readDataLength;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset, MessageType.SHARED_DIRECTORY_READ_RESPONSE);
    offset += BYTE_LEN;
    view.setUint32(offset, res.completionId);
    offset += UINT_32_LEN;
    view.setUint32(offset, res.errCode);
    offset += UINT_32_LEN;
    view.setUint32(offset, res.readDataLength);
    offset += UINT_32_LEN;
    res.readData.forEach(byte => {
      view.setUint8(offset++, byte);
    });

    return buffer;
  }

  // | message type (22) | completion_id uint32 | err_code uint32 | bytes_written uint32 |
  encodeSharedDirectoryWriteResponse(
    res: SharedDirectoryWriteResponse
  ): Message {
    const bufLen = BYTE_LEN + 3 * UINT_32_LEN;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset, MessageType.SHARED_DIRECTORY_WRITE_RESPONSE);
    offset += BYTE_LEN;
    view.setUint32(offset, res.completionId);
    offset += UINT_32_LEN;
    view.setUint32(offset, res.errCode);
    offset += UINT_32_LEN;
    view.setUint32(offset, res.bytesWritten);
    offset += UINT_32_LEN;

    return buffer;
  }

  // | message type (24) | completion_id uint32 | err_code uint32 |
  encodeSharedDirectoryMoveResponse(res: SharedDirectoryMoveResponse): Message {
    return this.encodeGenericResponse(
      MessageType.SHARED_DIRECTORY_MOVE_RESPONSE,
      res
    );
  }

  // | message type (26) | completion_id uint32 | err_code uint32 | fso_list_length uint32 | fso_list fso[] |
  encodeSharedDirectoryListResponse(res: SharedDirectoryListResponse): Message {
    const bufLenSansFsoList = BYTE_LEN + 3 * UINT_32_LEN;
    const bufferSansFsoList = new ArrayBuffer(bufLenSansFsoList);
    const view = new DataView(bufferSansFsoList);
    let offset = 0;

    view.setUint8(offset++, MessageType.SHARED_DIRECTORY_LIST_RESPONSE);
    view.setUint32(offset, res.completionId);
    offset += UINT_32_LEN;
    view.setUint32(offset, res.errCode);
    offset += UINT_32_LEN;
    view.setUint32(offset, res.fsoList.length);
    offset += UINT_32_LEN;

    let withFsoList = new Uint8Array(bufferSansFsoList);
    res.fsoList.forEach(fso => {
      const fsoBuffer = this.encodeFileSystemObject(fso);

      // https://gist.github.com/72lions/4528834?permalink_comment_id=2395442#gistcomment-2395442
      withFsoList = new Uint8Array([
        ...withFsoList,
        ...new Uint8Array(fsoBuffer),
      ]);
    });

    return withFsoList.buffer;
  }

  encodeSharedDirectoryTruncateResponse(
    res: SharedDirectoryTruncateResponse
  ): Message {
    return this.encodeGenericResponse(
      MessageType.SHARED_DIRECTORY_TRUNCATE_RESPONSE,
      res
    );
  }

  private encodeGenericResponse(
    type: MessageType,
    res: {
      completionId: number;
      errCode: SharedDirectoryErrCode;
    }
  ): Message {
    const bufLen = BYTE_LEN + 2 * UINT_32_LEN;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset, type);
    offset += BYTE_LEN;
    view.setUint32(offset, res.completionId);
    offset += UINT_32_LEN;
    view.setUint32(offset, res.errCode);
    offset += UINT_32_LEN;

    return buffer;
  }

  // | last_modified uint64 | size uint64 | file_type uint32 | is_empty bool | path_length uint32 | path byte[] |
  encodeFileSystemObject(fso: FileSystemObject): Message {
    const dataUtf8array = this.encoder.encode(fso.path);

    const bufLen =
      BYTE_LEN + 2 * UINT_64_LEN + 2 * UINT_32_LEN + dataUtf8array.length;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;
    view.setBigUint64(offset, fso.lastModified);
    offset += UINT_64_LEN;
    view.setBigUint64(offset, fso.size);
    offset += UINT_64_LEN;
    view.setUint32(offset, fso.fileType);
    offset += UINT_32_LEN;
    view.setUint8(offset, fso.isEmpty ? 1 : 0);
    offset += BYTE_LEN;
    view.setUint32(offset, dataUtf8array.length);
    offset += UINT_32_LEN;
    dataUtf8array.forEach(byte => {
      view.setUint8(offset++, byte);
    });

    return buffer;
  }

  // | message type (30) | data_length uint32 | data []byte |
  encodeRdpResponsePdu(responseFrame: ArrayBufferLike): Message {
    const bufLen = BYTE_LEN + UINT_32_LEN + responseFrame.byteLength;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset, MessageType.RDP_RESPONSE_PDU);
    offset += BYTE_LEN;
    view.setUint32(offset, responseFrame.byteLength);
    offset += UINT_32_LEN;
    new Uint8Array(buffer, offset).set(new Uint8Array(responseFrame));

    return buffer;
  }

  // decodeClipboardData decodes clipboard data
  decodeClipboardData(buffer: ArrayBufferLike): ClipboardData {
    return {
      data: this.decodeStringMessage(buffer),
    };
  }

  /**
   * decodeMessageType decodes the MessageType from a raw tdp message
   * passed in as an ArrayBuffer (this typically would come from a websocket).
   * @throws {Error} on an invalid or unexpected MessageType value
   */
  decodeMessageType(buffer: ArrayBufferLike): MessageType {
    const messageType = new DataView(buffer).getUint8(0);
    if (!(messageType in MessageType) || messageType === MessageType.__LAST) {
      throw new Error(`invalid message type: ${messageType}`);
    }
    return messageType;
  }

  // decodeErrorMessage decodes a raw tdp Error message and returns it as a string
  // | message type (9) | message_length uint32 | message []byte
  decodeErrorMessage(buffer: ArrayBufferLike): string {
    return this.decodeStringMessage(buffer);
  }

  /**
   * decodeAlert decodes a raw TDP alert message
   * | message type (28) | message_length uint32 | message []byte | severity byte
   * @throws {Error} if an invalid severity is passed
   */
  decodeAlert(buffer: ArrayBufferLike): Alert {
    const dv = new DataView(buffer);
    let offset = 0;

    offset += BYTE_LEN; // eat message type

    const messageLength = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat messageLength

    const message = this.decodeStringMessage(buffer);
    offset += messageLength; // eat message

    const severity = dv.getUint8(offset);

    return {
      message,
      severity: toSeverity(severity),
    };
  }

  // decodeMfaChallenge decodes a raw tdp MFA challenge message and returns it as a string (of a json).
  // | message type (10) | mfa_type byte | message_length uint32 | json []byte
  decodeMfaJson(buffer: ArrayBufferLike): MfaJson {
    const dv = new DataView(buffer);
    let offset = 0;
    offset += BYTE_LEN; // eat message type
    const mfaType = String.fromCharCode(dv.getUint8(offset));
    offset += BYTE_LEN; // eat mfa_type
    if (mfaType !== 'n' && mfaType !== 'u') {
      throw new Error(`invalid mfa type ${mfaType}, should be "n" or "u"`);
    }
    let messageLength = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat message_length
    const jsonString = this.decoder.decode(
      new Uint8Array(buffer, offset, messageLength)
    );
    return { mfaType, jsonString };
  }

  // decodeStringMessage decodes a tdp message of the form
  // | message type (N) | message_length uint32 | message []byte
  private decodeStringMessage(buffer: ArrayBufferLike): string {
    const dv = new DataView(buffer);
    let offset = BYTE_LEN; // eat message type
    const msgLength = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat messageLength

    return this.decoder.decode(new Uint8Array(buffer, offset, msgLength));
  }

  // decodePngFrame decodes a raw tdp PNG frame message and returns it as a PngFrame
  // | message type (2) | left uint32 | top uint32 | right uint32 | bottom uint32 | data []byte |
  // https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#2---png-frame
  decodePngFrame(
    buffer: ArrayBufferLike,
    onload: (pngFrame: PngFrame) => any
  ): PngFrame {
    const dv = new DataView(buffer);
    const image = new Image();
    let offset = 0;
    offset += BYTE_LEN; // eat message type
    const left = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat left
    const top = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat top
    const right = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat right
    const bottom = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat bottom
    const pngFrame = { left, top, right, bottom, data: image };
    pngFrame.data.onload = onload(pngFrame);
    pngFrame.data.src = this.asBase64Url(buffer, offset);

    return pngFrame;
  }

  // decodePng2Frame decodes a raw tdp PNG frame message and returns it as a PngFrame
  // | message type (27) | png_length uint32 | left uint32 | top uint32 | right uint32 | bottom uint32 | data []byte |
  decodePng2Frame(
    buffer: ArrayBufferLike,
    onload: (pngFrame: PngFrame) => any
  ): PngFrame {
    const dv = new DataView(buffer);
    const image = new Image();
    let offset = 0;
    offset += BYTE_LEN; // eat message type
    offset += UINT_32_LEN; // eat png_length
    const left = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat left
    const top = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat top
    const right = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat right
    const bottom = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat bottom
    const pngFrame = { left, top, right, bottom, data: image };
    pngFrame.data.onload = onload(pngFrame);
    pngFrame.data.src = this.asBase64Url(buffer, offset);

    return pngFrame;
  }

  // | message type (29) | data_length uint32 | data []byte |
  decodeRdpFastPathPdu(buffer: ArrayBufferLike): RdpFastPathPdu {
    const dv = new DataView(buffer);
    let offset = 0;
    offset += BYTE_LEN; // eat message type
    const dataLength = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat data_length
    return new Uint8Array(buffer, offset, dataLength);
  }

  // | message type (31) | io_channel_id uint16 | user_channel_id uint16 | screen_width uint16 | screen_height uint16 |
  decodeRdpConnectionActivated(
    buffer: ArrayBufferLike
  ): RdpConnectionActivated {
    const dv = new DataView(buffer);
    let offset = 0;
    offset += BYTE_LEN; // eat message type
    const ioChannelId = dv.getUint16(offset);
    offset += UINT_16_LEN;
    const userChannelId = dv.getUint16(offset);
    offset += UINT_16_LEN;

    const screenWidth = dv.getUint16(offset);
    offset += UINT_16_LEN;
    const screenHeight = dv.getUint16(offset);
    offset += UINT_16_LEN;

    return { ioChannelId, userChannelId, screenWidth, screenHeight };
  }

  // | message type (12) | err_code error | directory_id uint32 |
  decodeSharedDirectoryAcknowledge(
    buffer: ArrayBufferLike
  ): SharedDirectoryAcknowledge {
    const dv = new DataView(buffer);
    let offset = 0;
    offset += BYTE_LEN; // eat message type
    const errCode = toSharedDirectoryErrCode(dv.getUint32(offset));
    offset += UINT_32_LEN; // eat err_code
    const directoryId = dv.getUint32(5);

    return {
      errCode,
      directoryId,
    };
  }

  // | message type (13) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte |
  decodeSharedDirectoryInfoRequest(
    buffer: ArrayBufferLike
  ): SharedDirectoryInfoRequest {
    const dv = new DataView(buffer);
    let offset = 0;
    offset += BYTE_LEN; // eat message type
    const completionId = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat completion_id
    const directoryId = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat directory_id
    let pathLength = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat path_length
    const path = this.decoder.decode(
      new Uint8Array(buffer, offset, pathLength)
    );

    return {
      completionId,
      directoryId,
      path,
    };
  }

  // | message type (15) | completion_id uint32 | directory_id uint32 | file_type uint32 | path_length uint32 | path []byte |
  decodeSharedDirectoryCreateRequest(
    buffer: ArrayBufferLike
  ): SharedDirectoryCreateRequest {
    const dv = new DataView(buffer);
    let offset = 0;
    offset += BYTE_LEN; // eat message type
    const completionId = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat completion_id
    const directoryId = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat directory_id
    const fileType = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat directory_id
    let pathLength = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat path_length
    const path = this.decoder.decode(
      new Uint8Array(buffer, offset, pathLength)
    );

    return {
      completionId,
      directoryId,
      fileType,
      path,
    };
  }

  // | message type (17) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte |
  decodeSharedDirectoryDeleteRequest(
    buffer: ArrayBufferLike
  ): SharedDirectoryDeleteRequest {
    const dv = new DataView(buffer);
    let offset = 0;
    offset += BYTE_LEN; // eat message type
    const completionId = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat completion_id
    const directoryId = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat directory_id
    let pathLength = dv.getUint32(offset);
    offset += UINT_32_LEN; // eat path_length
    const path = this.decoder.decode(
      new Uint8Array(buffer, offset, pathLength)
    );

    return {
      completionId,
      directoryId,
      path,
    };
  }

  // | message type (19) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte | offset uint64 | length uint32 |
  decodeSharedDirectoryReadRequest(
    buffer: ArrayBufferLike
  ): SharedDirectoryReadRequest {
    const dv = new DataView(buffer);
    let bufOffset = 0;
    bufOffset += BYTE_LEN; // eat message type
    const completionId = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat completion_id
    const directoryId = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat directory_id
    const pathLength = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat path_length
    const path = this.decoder.decode(
      new Uint8Array(buffer, bufOffset, pathLength)
    );
    bufOffset += pathLength; // eat path
    const offset = dv.getBigUint64(bufOffset);
    bufOffset += UINT_64_LEN; // eat offset
    const length = dv.getUint32(bufOffset);

    return {
      completionId,
      directoryId,
      pathLength,
      path,
      offset,
      length,
    };
  }

  // | message type (21) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte | offset uint64 | write_data_length uint32 | write_data []byte |
  decodeSharedDirectoryWriteRequest(
    buffer: ArrayBufferLike
  ): SharedDirectoryWriteRequest {
    const dv = new DataView(buffer);
    let bufOffset = BYTE_LEN; // eat message type
    const completionId = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat completion_id
    const directoryId = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat directory_id
    const offset = dv.getBigUint64(bufOffset);
    bufOffset += UINT_64_LEN; // eat offset
    const pathLength = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat path_length
    const path = this.decoder.decode(
      new Uint8Array(buffer, bufOffset, pathLength)
    );
    bufOffset += pathLength; // eat path
    const writeDataLength = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat write_data_length
    const writeData = new Uint8Array(buffer, bufOffset, writeDataLength);

    return {
      completionId,
      directoryId,
      pathLength,
      path,
      offset,
      writeData,
    };
  }

  // | message type (23) | completion_id uint32 | directory_id uint32 | original_path_length uint32 | original_path []byte | new_path_length uint32 | new_path []byte |
  decodeSharedDirectoryMoveRequest(
    buffer: ArrayBufferLike
  ): SharedDirectoryMoveRequest {
    const dv = new DataView(buffer);
    let bufOffset = BYTE_LEN; // eat message type
    const completionId = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat completion_id
    const directoryId = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat directory_id
    const originalPathLength = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat original_path_length
    const originalPath = this.decoder.decode(
      new Uint8Array(buffer, bufOffset, originalPathLength)
    );
    bufOffset += originalPathLength; // eat original_path
    const newPathLength = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat new_path_length
    const newPath = this.decoder.decode(
      new Uint8Array(buffer, bufOffset, newPathLength)
    );

    return {
      completionId,
      directoryId,
      originalPathLength,
      originalPath,
      newPathLength,
      newPath,
    };
  }

  // | message type (25) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte |
  decodeSharedDirectoryListRequest(
    buffer: ArrayBufferLike
  ): SharedDirectoryListRequest {
    return this.decodeSharedDirectoryInfoRequest(buffer);
  }

  decodeSharedDirectoryTruncateRequest(
    buffer: ArrayBufferLike
  ): SharedDirectoryTruncateRequest {
    const dv = new DataView(buffer);
    let bufOffset = BYTE_LEN; // eat message type
    const completionId = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat completion_id
    const directoryId = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat directory_id
    const pathLength = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN; // eat path_length
    const path = this.decoder.decode(
      new Uint8Array(buffer, bufOffset, pathLength)
    );
    bufOffset += pathLength; // eat path
    const endOfFile = dv.getUint32(bufOffset);

    return {
      completionId,
      directoryId,
      path,
      endOfFile,
    };
  }

  decodeLatencyStats(buffer: ArrayBufferLike): LatencyStats {
    const dv = new DataView(buffer);
    let bufOffset = BYTE_LEN; // eat message type
    const browserLatency = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN;
    const desktopLatency = dv.getUint32(bufOffset);
    bufOffset += UINT_32_LEN;

    return {
      client: browserLatency,
      server: desktopLatency,
    };
  }

  // asBase64Url creates a data:image uri from the png data part of a PNG_FRAME tdp message.
  private asBase64Url(buffer: ArrayBufferLike, offset: number): string {
    return `data:image/png;base64,${arrayBufferToBase64(buffer.slice(offset))}`;
  }
}

const BYTE_LEN = 1;
const UINT_16_LEN = 2;
const UINT_32_LEN = 4;
const UINT_64_LEN = 8;

/**
 *  Maps from browser KeyboardEvent.code values to Windows hardware keycodes.
 *
 * The latest version of [scancode.h](https://github.com/FreeRDP/FreeRDP/blob/ba8cf8cf2158018fb7abbedb51ab245f369be813/include/freerdp/scancode.h)
 * in FreeRDP should be considered the canonical source of truth for these values.
 */
const KEY_SCANCODES: { [key: string]: number[] } = {
  Escape: [0x0001],
  Digit1: [0x0002],
  Digit2: [0x0003],
  Digit3: [0x0004],
  Digit4: [0x0005],
  Digit5: [0x0006],
  Digit6: [0x0007],
  Digit7: [0x0008],
  Digit8: [0x0009],
  Digit9: [0x000a],
  Digit0: [0x000b],
  Minus: [0x000c],
  Equal: [0x000d],
  Backspace: [0x000e],
  Tab: [0x000f],
  KeyQ: [0x0010],
  KeyW: [0x0011],
  KeyE: [0x0012],
  KeyR: [0x0013],
  KeyT: [0x0014],
  KeyY: [0x0015],
  KeyU: [0x0016],
  KeyI: [0x0017],
  KeyO: [0x0018],
  KeyP: [0x0019],
  BracketLeft: [0x001a],
  BracketRight: [0x001b],
  Enter: [0x001c],
  ControlLeft: [0x001d],
  KeyA: [0x001e],
  KeyS: [0x001f],
  KeyD: [0x0020],
  KeyF: [0x0021],
  KeyG: [0x0022],
  KeyH: [0x0023],
  KeyJ: [0x0024],
  KeyK: [0x0025],
  KeyL: [0x0026],
  Semicolon: [0x0027],
  Quote: [0x0028],
  Backquote: [0x0029],
  ShiftLeft: [0x002a],
  Backslash: [0x002b],
  KeyZ: [0x002c],
  KeyX: [0x002d],
  KeyC: [0x002e],
  KeyV: [0x002f],
  KeyB: [0x0030],
  KeyN: [0x0031],
  KeyM: [0x0032],
  Comma: [0x0033],
  Period: [0x0034],
  Slash: [0x0035],
  ShiftRight: [0x0036],
  NumpadMultiply: [0x0037],
  AltLeft: [0x0038],
  Space: [0x0039],
  CapsLock: [0x003a],
  F1: [0x003b],
  F2: [0x003c],
  F3: [0x003d],
  F4: [0x003e],
  F5: [0x003f],
  F6: [0x0040],
  F7: [0x0041],
  F8: [0x0042],
  F9: [0x0043],
  F10: [0x0044],
  // This must be sent as Ctrl + NumLock, see https://github.com/FreeRDP/FreeRDP/blob/ba8cf8cf2158018fb7abbedb51ab245f369be813/include/freerdp/scancode.h#L115-L116
  Pause: [0x001d, 0x0045],
  ScrollLock: [0x0046],
  Numpad7: [0x0047],
  Numpad8: [0x0048],
  Numpad9: [0x0049],
  NumpadSubtract: [0x004a],
  Numpad4: [0x004b],
  Numpad5: [0x004c],
  Numpad6: [0x004d],
  NumpadAdd: [0x004e],
  Numpad1: [0x004f],
  Numpad2: [0x0050],
  Numpad3: [0x0051],
  Numpad0: [0x0052],
  NumpadDecimal: [0x0053],
  IntlBackslash: [0x0056],
  F11: [0x0057],
  F12: [0x0058],
  NumpadEqual: [0x0059],
  F13: [0x0064],
  F14: [0x0065],
  F15: [0x0066],
  F16: [0x0067],
  F17: [0x0068],
  F18: [0x0069],
  F19: [0x006a],
  F20: [0x006b],
  F21: [0x006c],
  F22: [0x006d],
  F23: [0x006e],
  KanaMode: [0x0070],
  IntlRo: [0x0073],
  F24: [0x0076],
  Lang4: [0x0077],
  Lang3: [0x0077],
  Convert: [0x0079],
  NonConvert: [0x007b],
  IntlYen: [0x007d],
  NumpadComma: [0x007e],
  Undo: [0xe008],
  Paste: [0xe00a],
  MediaTrackPrevious: [0xe010],
  Cut: [0xe017],
  Copy: [0xe018],
  MediaTrackNext: [0xe019],
  NumpadEnter: [0xe01c],
  ControlRight: [0xe01d],
  AudioVolumeMute: [0xe020],
  LaunchApp2: [0xe021],
  MediaPlayPause: [0xe022],
  MediaStop: [0xe024],
  AudioVolumeDown: [0xe02e], // Chromium, Gecko
  VolumeDown: [0xe02e], // Firefox
  AudioVolumeUp: [0xe030], // Chromium, Gecko
  VolumeUp: [0xe030], // Firefox
  BrowserHome: [0xe032],
  NumpadDivide: [0xe035],
  PrintScreen: [0xe037],
  AltRight: [0xe038],
  NumLock: [0x0045],
  Home: [0xe047],
  ArrowUp: [0xe048],
  PageUp: [0xe049],
  ArrowLeft: [0xe04b],
  ArrowRight: [0xe04d],
  End: [0xe04f],
  ArrowDown: [0xe050],
  PageDown: [0xe051],
  Insert: [0xe052],
  Delete: [0xe053],
  MetaLeft: [0xe05b], // Chromium
  OSLeft: [0xe05b], // Firefox, Gecko
  MetaRight: [0xe05c], // Chromium
  OSRight: [0xe05c], // Firefox, Gecko
  ContextMenu: [0xe05d],
  Power: [0xe05e],
  BrowserSearch: [0xe065],
  BrowserFavorites: [0xe066],
  BrowserRefresh: [0xe067],
  BrowserStop: [0xe068],
  BrowserForward: [0xe069],
  BrowserBack: [0xe06a],
  LaunchApp1: [0xe06b],
  LaunchMail: [0xe06c],
  MediaSelect: [0xe06d],
};
