const { TextEncoder } = require('util');
import Codec, { MessageType, ButtonState, MouseButton } from './codec';

// Use nodejs TextEncoder until jsdom adds support for TextEncoder (https://github.com/jsdom/jsdom/issues/2524)
window.TextEncoder = window.TextEncoder || TextEncoder;
const codec = new Codec();

test('encodes the screen spec', () => {
  const w = 1800;
  const h = 1200;
  const message = codec.encodeScreenSpec(w, h);
  const view = new DataView(message);
  expect(view.getUint8(0)).toEqual(MessageType.CLIENT_SCREEN_SPEC);
  expect(view.getUint32(1)).toEqual(w);
  expect(view.getUint32(5)).toEqual(h);
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
    0x0048,
    0x0065,
    0x006c,
    0x006c,
    0x006f,
    0x0077,
    0x006f,
    0x0072,
    0x006c,
    0x0064,
    0x0021,
    0x002a,
    0x0040,
    0x0031,
    0x0032,
    0x0033,
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
    0x00,
    0x7f,
    0xc2,
    0x80,
    0xdf,
    0xbf,
    0xe0,
    0xa0,
    0x80,
    0xef,
    0xbf,
    0xbf,
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

test('decodes message types', () => {
  const pngFrameBuf = new ArrayBuffer(100);
  const clipboardBuf = new ArrayBuffer(100);
  const cliScreenBuf = new ArrayBuffer(100);
  const pngFrameView = new DataView(pngFrameBuf);
  const clipboardView = new DataView(clipboardBuf);
  const cliScreenView = new DataView(cliScreenBuf);

  pngFrameView.setUint8(0, MessageType.PNG_FRAME);
  expect(codec.decodeMessageType(pngFrameBuf)).toEqual(MessageType.PNG_FRAME);

  clipboardView.setUint8(0, MessageType.CLIPBOARD_DATA);
  expect(codec.decodeMessageType(clipboardBuf)).toEqual(
    MessageType.CLIPBOARD_DATA
  );

  // We only expect to need to decode png frames and clipboard data.
  cliScreenView.setUint8(0, MessageType.CLIENT_SCREEN_SPEC);
  expect(() => {
    codec.decodeMessageType(cliScreenBuf);
  }).toThrow(`invalid message type: ${MessageType.CLIENT_SCREEN_SPEC}`);
});
