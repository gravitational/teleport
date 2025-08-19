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

import Codec, {
  ButtonState,
  MessageType,
  MouseButton,
  ScrollAxis,
} from './codec';

const codec = new Codec();

test('encodes and decodes the screen spec', () => {
  const spec = {
    width: 1800,
    height: 1200,
  };
  const message = codec.encodeClientScreenSpec(spec);
  const view = new DataView(message);
  expect(view.getUint8(0)).toEqual(MessageType.CLIENT_SCREEN_SPEC);
  expect(view.getUint32(1)).toEqual(spec.width);
  expect(view.getUint32(5)).toEqual(spec.height);

  const decodedSpec = codec.decodeClientScreenSpec(message);
  expect(decodedSpec).toEqual(spec);
});

test('encodes mouse moves', () => {
  const x = 0;
  const y = Math.pow(2, 32) - 1;
  const message = codec.encodeMouseMove(x, y);
  const view = new DataView(message);
  expect(view.getUint8(0)).toEqual(MessageType.MOUSE_MOVE);
  expect(view.getUint32(1)).toEqual(x);
  expect(view.getUint32(5)).toEqual(y);
});

test('encodes mouse buttons', () => {
  [0, 1, 2].forEach(button => {
    [ButtonState.DOWN, ButtonState.UP].forEach(state => {
      const message = codec.encodeMouseButton(button as MouseButton, state);
      const view = new DataView(message);
      expect(view.getUint8(0)).toEqual(MessageType.MOUSE_BUTTON);
      expect(view.getUint8(1)).toEqual(button);
      expect(view.getUint8(2)).toEqual(state);
    });
  });
});

// Username/password tests inspired by https://github.com/google/closure-library/blob/master/closure/goog/crypt/crypt_test.js (Apache License)
test('encodes typical characters for username and password', () => {
  // Create a test value with letters, symbols, and numbers and its known UTF8 encodings
  const username = 'Helloworld!*@123';
  const usernameUTF8 = [
    0x0048, 0x0065, 0x006c, 0x006c, 0x006f, 0x0077, 0x006f, 0x0072, 0x006c,
    0x0064, 0x0021, 0x002a, 0x0040, 0x0031, 0x0032, 0x0033,
  ];

  // Encode test vals
  const message = codec.encodeUsername(username);
  const view = new DataView(message);

  // Walk through output
  let offset = 0;
  expect(view.getUint8(offset++)).toEqual(MessageType.CLIENT_USERNAME);
  expect(view.getUint32(offset)).toEqual(usernameUTF8.length);
  offset += 4;
  usernameUTF8.forEach(byte => {
    expect(view.getUint8(offset++)).toEqual(byte);
  });
});

test('encodes utf8 characters correctly up to 3 bytes for username and password', () => {
  const first3RangesString = '\u0000\u007F\u0080\u07FF\u0800\uFFFF';
  const first3RangesUTF8 = [
    0x00, 0x7f, 0xc2, 0x80, 0xdf, 0xbf, 0xe0, 0xa0, 0x80, 0xef, 0xbf, 0xbf,
  ];
  const message = codec.encodeUsername(first3RangesString);
  const view = new DataView(message);
  let offset = 0;
  expect(view.getUint8(offset++)).toEqual(MessageType.CLIENT_USERNAME);
  expect(view.getUint32(offset)).toEqual(first3RangesUTF8.length);
  offset += 4;
  first3RangesUTF8.forEach(byte => {
    expect(view.getUint8(offset++)).toEqual(byte);
  });
});

test('encodes mouse wheel scroll event', () => {
  const axis = ScrollAxis.VERTICAL;
  const delta = 860;
  const message = codec.encodeMouseWheelScroll(axis, delta);
  const view = new DataView(message);
  expect(view.getUint8(0)).toEqual(MessageType.MOUSE_WHEEL_SCROLL);
  expect(view.getUint8(1)).toEqual(axis);
  expect(view.getUint16(2)).toEqual(delta);
});

function makeBuf(type: MessageType, size = 100) {
  const buffer = new ArrayBuffer(size);
  const view = new DataView(buffer);
  view.setUint8(0, type);
  return { buffer };
}

test('decodes message types', () => {
  const { buffer: pngFrameBuf } = makeBuf(MessageType.PNG_FRAME);
  const { buffer: clipboardBuf } = makeBuf(MessageType.CLIPBOARD_DATA);
  const { buffer: errorBuf } = makeBuf(MessageType.ERROR);
  let invalid = MessageType.__LAST;
  const { buffer: invalidBuf } = makeBuf(invalid);

  expect(codec.decodeMessageType(pngFrameBuf)).toEqual(MessageType.PNG_FRAME);
  expect(codec.decodeMessageType(clipboardBuf)).toEqual(
    MessageType.CLIPBOARD_DATA
  );
  expect(codec.decodeMessageType(errorBuf)).toEqual(MessageType.ERROR);
  expect(() => {
    codec.decodeMessageType(invalidBuf);
  }).toThrow(`invalid message type: ${invalid}`);
});

test('decodes errors', () => {
  // First encode an error
  const encoder = new TextEncoder();
  const message = encoder.encode('An error occured');
  const bufLen = 1 + 4 + message.length;
  const tdpErrorBuffer = new ArrayBuffer(bufLen);
  const view = new DataView(tdpErrorBuffer);
  let offset = 0;
  view.setUint8(offset++, MessageType.ERROR);
  view.setUint32(offset, message.length);
  offset += 4; // 4 bytes to offset 32-bit uint
  message.forEach(byte => {
    view.setUint8(offset++, byte);
  });

  const error = codec.decodeErrorMessage(tdpErrorBuffer);
  expect(error).toBe('An error occured');
});

// Username/password tests inspired by https://github.com/google/closure-library/blob/master/closure/goog/crypt/crypt_test.js (Apache License)
test('encodes and decodes clipboard data', () => {
  // Create a test value with letters, symbols, and numbers and its known UTF8 encodings
  const clipboardData = 'Helloworld!*@123';
  const clipboardDataUTF8 = [
    0x0048, 0x0065, 0x006c, 0x006c, 0x006f, 0x0077, 0x006f, 0x0072, 0x006c,
    0x0064, 0x0021, 0x002a, 0x0040, 0x0031, 0x0032, 0x0033,
  ];

  // Encode test vals
  const encodedData = codec.encodeClipboardData({
    data: clipboardData,
  });
  const view = new DataView(encodedData);

  // Walk through output
  let offset = 0;
  expect(view.getUint8(offset++)).toEqual(MessageType.CLIPBOARD_DATA);
  expect(view.getUint32(offset)).toEqual(clipboardDataUTF8.length);
  offset += 4;
  clipboardDataUTF8.forEach(byte => {
    expect(view.getUint8(offset++)).toEqual(byte);
  });

  const decoded = codec.decodeClipboardData(encodedData);
  expect(decoded.data).toEqual(clipboardData);
});
