import Codec, { MessageType, ButtonState, MouseButton } from './codec';

function getRandomInt(min, max) {
  min = Math.ceil(min);
  max = Math.floor(max);
  return Math.floor(Math.random() * (max - min) + min);
}

describe('codec', () => {
  const codec = new Codec();

  describe('encoding', () => {
    it('encodes the screen spec', () => {
      const w = getRandomInt(10, 100);
      const h = getRandomInt(10, 100);
      const message = codec.encScreenSpec(w, h);
      const view = new DataView(message);
      expect(view.getUint8(0)).toEqual(MessageType.CLIENT_SCREEN_SPEC);
      expect(view.getUint32(1)).toEqual(w);
      expect(view.getUint32(5)).toEqual(h);
    });

    it('encodes mouse moves', () => {
      const x = getRandomInt(0, 100);
      const y = getRandomInt(0, 100);
      const message = codec.encMouseMove(x, y);
      const view = new DataView(message);
      expect(view.getUint8(0)).toEqual(MessageType.MOUSE_MOVE);
      expect(view.getUint32(1)).toEqual(x);
      expect(view.getUint32(5)).toEqual(y);
    });

    it('encodes mouse buttons', () => {
      [0, 1, 2].forEach(button => {
        [ButtonState.DOWN, ButtonState.UP].forEach(state => {
          const message = codec.encMouseButton(button as MouseButton, state);
          const view = new DataView(message);
          expect(view.getUint8(0)).toEqual(MessageType.MOUSE_BUTTON);
          expect(view.getUint8(1)).toEqual(button);
          expect(view.getUint8(2)).toEqual(state);
        });
      });
    });

    describe('encodes username and password', () => {
      // Tests for more than we probably need to support for username and password.
      // Tests inspired by https://github.com/google/closure-library/blob/master/closure/goog/crypt/crypt_test.js (Apache License)
      it('encodes typical characters', () => {
        // Create test vals + known UTF8 encodings
        const username = 'Hello';
        const usernameUTF8 = [0x0048, 0x0065, 0x006c, 0x006c, 0x006f];
        const password = 'world!*@123';
        const passwordUTF8 = [
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
        const message = codec.encUsernamePassword(username, password);
        const view = new DataView(message);

        // Walk through output
        let offset = 0;
        expect(view.getUint8(offset++)).toEqual(
          MessageType.USERNAME_PASSWORD_RESPONSE
        );
        expect(view.getUint32(offset)).toEqual(usernameUTF8.length);
        offset += 4;
        usernameUTF8.forEach(byte => {
          expect(view.getUint8(offset++)).toEqual(byte);
        });
        expect(view.getUint32(offset)).toEqual(passwordUTF8.length);
        offset += 4;
        passwordUTF8.forEach(byte => {
          expect(view.getUint8(offset++)).toEqual(byte);
        });
      });

      it('encodes utf8 characters correctly up to 3 bytes', () => {
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
        const message = codec.encUsernamePassword(
          first3RangesString,
          first3RangesString
        );
        const view = new DataView(message);
        let offset = 0;
        expect(view.getUint8(offset++)).toEqual(
          MessageType.USERNAME_PASSWORD_RESPONSE
        );
        expect(view.getUint32(offset)).toEqual(first3RangesUTF8.length);
        offset += 4;
        first3RangesUTF8.forEach(byte => {
          expect(view.getUint8(offset++)).toEqual(byte);
        });
        expect(view.getUint32(offset)).toEqual(first3RangesUTF8.length);
        offset += 4;
        first3RangesUTF8.forEach(byte => {
          expect(view.getUint8(offset++)).toEqual(byte);
        });
      });
    });
  });

  describe('decoding', () => {
    it.todo(`TODO: jest uses jsdom to emulate a browser environment during the tests, but jsdom does not currently
    support Blob.arrayBuffer() (used in our decoding functions) and thus it is difficult to test.
    I think I've come up with a hacky workaround but @awly and I agreed to put this aside
    for the time being; all will be tested manually for now by necessity during development.`);
  });
});
