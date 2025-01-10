/*
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

import * as protobuf from './protobuf';

describe('lib/term/protobuf', () => {
  const pb = new protobuf.Protobuf();

  describe('decoding', () => {
    it('should decode "audit" message', () => {
      // prettier-ignore
      const input = [ 10, 1, 49, 18, 1, 97, 26, 238, 1, 123, 34, 97, 100, 100, 114, 46, 108, 111, 99, 97, 108, 34, 58, 34, 49, 55, 50, 46, 49, 48, 46, 49, 46, 50, 48, 58, 51, 48, 50, 50, 34, 44, 34, 97, 100, 100, 114, 46, 114, 101, 109, 111, 116, 101, 34, 58, 34, 49, 55, 50, 46, 49, 48, 46, 49, 46, 50, 53, 52, 58, 53, 57, 53, 57, 48, 34, 44, 34, 101, 118, 101, 110, 116, 34, 58, 34, 115, 101, 115, 115, 105, 111, 110, 46, 106, 111, 105, 110, 34, 44, 34, 108, 111, 103, 105, 110, 34, 58, 34, 114, 111, 111, 116, 34, 44, 34, 110, 97, 109, 101, 115, 112, 97, 99, 101, 34, 58, 34, 100, 101, 102, 97, 117, 108, 116, 34, 44, 34, 115, 101, 114, 118, 101, 114, 95, 105, 100, 34, 58, 34, 55, 53, 102, 52, 102, 99, 56, 48, 45, 55, 54, 99, 53, 45, 52, 51, 55, 50, 45, 98, 99, 54, 49, 45, 49, 101, 54, 54, 53, 102, 100, 55, 101, 102, 57, 54, 34, 44, 34, 115, 105, 100, 34, 58, 34, 99, 99, 56, 100, 48, 53, 102, 52, 45, 54, 57, 100, 49, 45, 49, 49, 101, 56, 45, 97, 54, 49, 100, 45, 48, 50, 52, 50, 97, 99, 48, 97, 48, 49, 48, 49, 34, 44, 34, 117, 115, 101, 114, 34, 58, 34, 109, 97, 109, 97, 34, 125 ];
      const array = Uint8Array.from(input);
      const msg = pb.decode(array);

      expect(msg.type).toBe('a');
      expect(msg.payload).toBe(
        `{"addr.local":"172.10.1.20:3022","addr.remote":"172.10.1.254:59590","event":"session.join","login":"root","namespace":"default","server_id":"75f4fc80-76c5-4372-bc61-1e665fd7ef96","sid":"cc8d05f4-69d1-11e8-a61d-0242ac0a0101","user":"mama"}`
      );
    });

    it('should decode "close" message', () => {
      const input = [10, 1, 49, 18, 1, 99];
      const array = Uint8Array.from(input);
      const msg = pb.decode(array);
      expect(msg.type).toBe('c');
      expect(msg.payload).toBe(``);
    });

    it('should decode "raw" message', () => {
      // prettier-ignore
      const input = [10, 1, 49, 18, 1, 114, 26, 46, 27, 91, 51, 51, 59, 49, 109, 99, 111, 110, 116, 97, 105, 110, 101, 114, 40, 102, 49, 102, 102, 50, 57, 53, 101, 52, 49, 50, 55, 41, 27, 91, 48, 59, 51, 51, 109, 32, 126, 27, 91, 48, 48, 109, 58, 32];
      const array = Uint8Array.from(input);
      const msg = pb.decode(array);

      expect(msg.version).toBe('1');
      expect(msg.type).toBe('r');
      expect(msg.payload).toBe(`[33;1mcontainer(f1ff295e4127)[0;33m ~[00m: `);
    });
  });

  describe('encoding', () => {
    it('should encode "raw" message', () => {
      const buffer = pb.encodeRawMessage('mama');
      const array = Uint8Array.from(buffer);
      const msg = pb.decode(array);
      expect(msg.version).toBe('1');
      expect(msg.type).toBe('r');
      expect(msg.payload).toBe('mama');
    });

    it('should encode "resize" message', () => {
      const payload = 'test';
      const buffer = pb.encodeResizeMessage(payload);
      const array = Uint8Array.from(buffer);
      const msg = pb.decode(array);
      expect(msg.version).toBe('1');
      expect(msg.type).toBe('w');
      expect(msg.payload).toBe(payload);
    });
  });
});
