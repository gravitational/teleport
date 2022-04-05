/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { arrayBufferToBase64 } from 'shared/utils/base64';

// This is needed for tests until jsdom adds support for TextEncoder (https://github.com/jsdom/jsdom/issues/2524)
const {
  TextEncoder: TestTextEncoder,
  TextDecoder: TestTextDecoder,
} = require('util');
window.TextEncoder = window.TextEncoder || TestTextEncoder;
window.TextDecoder = window.TextDecoder || TestTextDecoder;

export type Message = ArrayBuffer;

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
}

// 0 is left button, 1 is middle button, 2 is right button
export type MouseButton = 0 | 1 | 2;

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

// | message type (2) | left uint32 | top uint32 | right uint32 | bottom uint32 | data []byte |
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#2---png-frame
export type PngFrame = {
  left: number;
  top: number;
  right: number;
  bottom: number;
  data: HTMLImageElement;
};

// | message type (6) | length uint32 | data []byte |
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#6---clipboard-data
export type ClipboardData = {
  // TODO(isaiah): store this as a byte array
  // https://github.com/gravitational/webapps/issues/610
  data: string;
};

// | message type (10) | mfa_type byte | message_length uint32 | json []byte
export type MfaJson = {
  // TODO(isaiah) make this a type that ensures it's the same as MessageTypeEnum.U2F_CHALLENGE and MessageTypeEnum.WEBAUTHN_CHALLENGE
  mfaType: 'u' | 'n';
  jsonString: string;
};

// TdaCodec provides an api for encoding and decoding teleport desktop access protocol messages [1]
// Buffers in TdaCodec are manipulated as DataView's [2] in order to give us low level control
// of endianness (defaults to big endian, which is what we want), as opposed to using *Array
// objects [3] which use the platform's endianness.
// [1] https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md
// [2] https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/DataView
// [3] https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Int32Array
export default class Codec {
  encoder = new window.TextEncoder();
  decoder = new window.TextDecoder();

  // Maps from browser KeyboardEvent.code values to Windows hardware keycodes.
  // Currently only supports Chrome keycodes: TODO(isaiah) -- add support for firefox/safari/edge.
  // See https://developer.mozilla.org/en-US/docs/Web/API/KeyboardEvent/code/code_values#code_values_on_windows
  private _keyScancodes = {
    Escape: 0x0001,
    Digit1: 0x0002,
    Digit2: 0x0003,
    Digit3: 0x0004,
    Digit4: 0x0005,
    Digit5: 0x0006,
    Digit6: 0x0007,
    Digit7: 0x0008,
    Digit8: 0x0009,
    Digit9: 0x000a,
    Digit0: 0x000b,
    Minus: 0x000c,
    Equal: 0x000d,
    Backspace: 0x000e,
    Tab: 0x000f,
    KeyQ: 0x0010,
    KeyW: 0x0011,
    KeyE: 0x0012,
    KeyR: 0x0013,
    KeyT: 0x0014,
    KeyY: 0x0015,
    KeyU: 0x0016,
    KeyI: 0x0017,
    KeyO: 0x0018,
    KeyP: 0x0019,
    BracketLeft: 0x001a,
    BracketRight: 0x001b,
    Enter: 0x001c,
    ControlLeft: 0x001d,
    KeyA: 0x001e,
    KeyS: 0x001f,
    KeyD: 0x0020,
    KeyF: 0x0021,
    KeyG: 0x0022,
    KeyH: 0x0023,
    KeyJ: 0x0024,
    KeyK: 0x0025,
    KeyL: 0x0026,
    Semicolon: 0x0027,
    Quote: 0x0028,
    Backquote: 0x0029,
    ShiftLeft: 0x002a,
    Backslash: 0x002b,
    KeyZ: 0x002c,
    KeyX: 0x002d,
    KeyC: 0x002e,
    KeyV: 0x002f,
    KeyB: 0x0030,
    KeyN: 0x0031,
    KeyM: 0x0032,
    Comma: 0x0033,
    Period: 0x0034,
    Slash: 0x0035,
    ShiftRight: 0x0036,
    NumpadMultiply: 0x0037,
    AltLeft: 0x0038,
    Space: 0x0039,
    CapsLock: 0x003a,
    F1: 0x003b,
    F2: 0x003c,
    F3: 0x003d,
    F4: 0x003e,
    F5: 0x003f,
    F6: 0x0040,
    F7: 0x0041,
    F8: 0x0042,
    F9: 0x0043,
    F10: 0x0044,
    Pause: 0x0045,
    ScrollLock: 0x0046,
    Numpad7: 0x0047,
    Numpad8: 0x0048,
    Numpad9: 0x0049,
    NumpadSubtract: 0x004a,
    Numpad4: 0x004b,
    Numpad5: 0x004c,
    Numpad6: 0x004d,
    NumpadAdd: 0x004e,
    Numpad1: 0x004f,
    Numpad2: 0x0050,
    Numpad3: 0x0051,
    Numpad0: 0x0052,
    NumpadDecimal: 0x0053,
    IntlBackslash: 0x0056,
    F11: 0x0057,
    F12: 0x0058,
    NumpadEqual: 0x0059,
    F13: 0x0064,
    F14: 0x0065,
    F15: 0x0066,
    F16: 0x0067,
    F17: 0x0068,
    F18: 0x0069,
    F19: 0x006a,
    F20: 0x006b,
    F21: 0x006c,
    F22: 0x006d,
    F23: 0x006e,
    KanaMode: 0x0070,
    IntlRo: 0x0073,
    F24: 0x0076,
    Lang4: 0x0077,
    Lang3: 0x0077,
    Convert: 0x0079,
    NonConvert: 0x007b,
    IntlYen: 0x007d,
    NumpadComma: 0x007e,
    Undo: 0xe008,
    Paste: 0xe00a,
    MediaTrackPrevious: 0xe010,
    Cut: 0xe017,
    Copy: 0xe018,
    MediaTrackNext: 0xe019,
    NumpadEnter: 0xe01c,
    ControlRight: 0xe01d,
    AudioVolumeMute: 0xe020,
    LaunchApp2: 0xe021,
    MediaPlayPause: 0xe022,
    MediaStop: 0xe024,
    AudioVolumeDown: 0xe02e, // Chromium, Gecko
    VolumeDown: 0xe02e, // Firefox
    AudioVolumeUp: 0xe030, // Chromium, Gecko
    VolumeUp: 0xe030, // Firefox
    BrowserHome: 0xe032,
    NumpadDivide: 0xe035,
    PrintScreen: 0xe037,
    AltRight: 0xe038,
    NumLock: 0xe045,
    Home: 0xe047,
    ArrowUp: 0xe048,
    PageUp: 0xe049,
    ArrowLeft: 0xe04b,
    ArrowRight: 0xe04d,
    End: 0xe04f,
    ArrowDown: 0xe050,
    PageDown: 0xe051,
    Insert: 0xe052,
    Delete: 0xe053,
    MetaLeft: 0xe05b, // Chromium
    OSLeft: 0xe05b, // Firefox, Gecko
    MetaRight: 0xe05c, // Chromium
    OSRight: 0xe05c, // Firefox, Gecko
    ContextMenu: 0xe05d,
    Power: 0xe05e,
    BrowserSearch: 0xe065,
    BrowserFavorites: 0xe066,
    BrowserRefresh: 0xe067,
    BrowserStop: 0xe068,
    BrowserForward: 0xe069,
    BrowserBack: 0xe06a,
    LaunchApp1: 0xe06b,
    LaunchMail: 0xe06c,
    MediaSelect: 0xe06d,
  };

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
  decodeClientScreenSpec(buffer: ArrayBuffer): ClientScreenSpec {
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
  // Returns null if an unsupported code is passed.
  // | message type (5) | key_code uint32 | state byte |
  encodeKeyboardInput(code: string, state: ButtonState): Message {
    const scanCode = this._keyScancodes[code];
    if (!scanCode) {
      return null;
    }
    const buffer = new ArrayBuffer(6);
    const view = new DataView(buffer);
    view.setUint8(0, MessageType.KEYBOARD_BUTTON);
    view.setUint32(1, scanCode);
    view.setUint8(5, state);
    return buffer;
  }

  // _encodeStringMessage encodes a message of the form
  // | message type (N) | length uint32 | data []byte |
  _encodeStringMessage(messageType: MessageType, data: string) {
    const dataUtf8array = this.encoder.encode(data);

    // bufLen is 1 byte for the `message type`,
    // 4 bytes for the `length uint32`,
    // and enough bytes for the full `data []byte`
    const bufLen = byteLength + uint32Length + dataUtf8array.length;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset++, messageType);
    view.setUint32(offset, dataUtf8array.length);
    offset += uint32Length;
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
  encodeMfaJson(mfaJson: MfaJson): Message {
    const dataUtf8array = this.encoder.encode(mfaJson.jsonString);

    const bufLen =
      byteLength + byteLength + uint32Length + dataUtf8array.length;
    const buffer = new ArrayBuffer(bufLen);
    const view = new DataView(buffer);
    let offset = 0;

    view.setUint8(offset++, MessageType.MFA_JSON);
    view.setUint8(offset++, mfaJson.mfaType.charCodeAt(0));
    view.setUint32(offset, dataUtf8array.length);
    offset += uint32Length;
    dataUtf8array.forEach(byte => {
      view.setUint8(offset++, byte);
    });

    return buffer;
  }

  // decodeClipboardData decodes clipboard data
  decodeClipboardData(buffer: ArrayBuffer): ClipboardData {
    return {
      data: this._decodeStringMessage(buffer),
    };
  }

  // decodeMessageType decodes the MessageType from a raw tdp message
  // passed in as an ArrayBuffer (this typically would come from a websocket).
  // Throws an error on an invalid or unexpected MessageType value.
  decodeMessageType(buffer: ArrayBuffer): MessageType {
    const messageType = new DataView(buffer).getUint8(0);
    // TODO(isaiah): this is fragile, instead switch all possibilities here.
    if (messageType > MessageType.MFA_JSON) {
      throw new Error(`invalid message type: ${messageType}`);
    }
    return messageType;
  }

  // decodeError decodes a raw tdp ERROR message and returns it as a string
  // | message type (9) | message_length uint32 | message []byte
  decodeErrorMessage(buffer: ArrayBuffer): string {
    return this._decodeStringMessage(buffer);
  }

  // decodeMfaChallenge decodes a raw tdp MFA challenge message and returns it as a string (of a json).
  // | message type (10) | mfa_type byte | message_length uint32 | json []byte
  decodeMfaJson(buffer: ArrayBuffer): MfaJson {
    let dv = new DataView(buffer);
    const mfaType = String.fromCharCode(dv.getUint8(1));
    if (mfaType !== 'n' && mfaType !== 'u') {
      throw new Error(`invalid mfa type ${mfaType}, should be "n" or "u"`);
    }
    const jsonString = this.decoder.decode(new Uint8Array(buffer.slice(6)));
    return { mfaType, jsonString };
  }

  // _decodeStringMessage decodes a tdp message of the form
  // | message type (N) | message_length uint32 | message []byte
  _decodeStringMessage(buffer: ArrayBuffer): string {
    // slice(5) ensures we skip the message type and message_length
    return this.decoder.decode(new Uint8Array(buffer.slice(5)));
  }

  // decodePngFrame decodes a raw tdp PNG frame message and returns it as a PngFrame
  // | message type (2) | left uint32 | top uint32 | right uint32 | bottom uint32 | data []byte |
  // https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#2---png-frame
  decodePngFrame(buffer: ArrayBuffer, onload: (PngFrame) => any): PngFrame {
    let dv = new DataView(buffer);
    const image = new Image();
    const pngFrame = {
      left: dv.getUint32(1),
      top: dv.getUint32(5),
      right: dv.getUint32(9),
      bottom: dv.getUint32(13),
      data: image,
    };
    pngFrame.data.onload = onload(pngFrame);
    pngFrame.data.src = this._asBase64Url(buffer);

    return pngFrame;
  }

  // _asBase64Url creates a data:image uri from the png data part of a PNG_FRAME tdp message.
  _asBase64Url(buffer: ArrayBuffer): string {
    return `data:image/png;base64,${arrayBufferToBase64(buffer.slice(17))}`;
  }
}

const byteLength = 1;
const uint32Length = 4;
