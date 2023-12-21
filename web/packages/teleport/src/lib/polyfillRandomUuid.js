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

if (typeof crypto?.randomUUID != 'function') {
  window.crypto.randomUUID = randomUUID;
}

// This polyfill is from https://github.com/uuidjs/randomUUID/tree/0273d4a16e25e05627ff4797739d976f6302bd6c

// Code based on Node.js' `lib/internal/crypto/random.js`, subject
// to Node.js license found at:
// https://raw.githubusercontent.com/nodejs/node/master/LICENSE

//
// internal/errors
//
class ERR_INVALID_ARG_TYPE extends TypeError {
  constructor(name, type, value) {
    super(`${name} variable is not of type ${type} (value: '${value}')`);
  }

  code = 'ERR_INVALID_ARG_TYPE';
}

//
// internal/validators
//

function validateBoolean(value, name) {
  if (typeof value !== 'boolean')
    throw new ERR_INVALID_ARG_TYPE(name, 'boolean', value);
}

function validateObject(value, name) {
  if (value === null || Array.isArray(value) || typeof value !== 'object') {
    throw new ERR_INVALID_ARG_TYPE(name, 'Object', value);
  }
}

//
// crypto
//

const randomFillSync =
  typeof window === 'undefined'
    ? require('crypto').randomFillSync
    : window.crypto.getRandomValues.bind(window.crypto);

// Implements an RFC 4122 version 4 random UUID.
// To improve performance, random data is generated in batches
// large enough to cover kBatchSize UUID's at a time. The uuidData
// and uuid buffers are reused. Each call to randomUUID() consumes
// 16 bytes from the buffer.

const kHexDigits = [
  48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 97, 98, 99, 100, 101, 102,
];

const kBatchSize = 128;
let uuidData;
let uuidNotBuffered;
let uuid;
let uuidBatch = 0;

function getBufferedUUID() {
  if (uuidData === undefined) {
    uuidData = new Uint8Array(16 * kBatchSize);
  }

  if (uuidBatch === 0) randomFillSync(uuidData);
  uuidBatch = (uuidBatch + 1) % kBatchSize;
  return uuidData.slice(uuidBatch * 16, uuidBatch * 16 + 16);
}

function randomUUID(options) {
  if (options !== undefined) validateObject(options, 'options');
  const { disableEntropyCache = false } = { ...options };

  validateBoolean(disableEntropyCache, 'options.disableEntropyCache');

  if (uuid === undefined) {
    uuid = new Uint8Array(36);
    uuid[8] = uuid[13] = uuid[18] = uuid[23] = '-'.charCodeAt(0);
    uuid[14] = 52; // '4', identifies the UUID version
  }

  let uuidBuf;
  if (!disableEntropyCache) {
    uuidBuf = getBufferedUUID();
  } else {
    uuidBuf = uuidNotBuffered;
    if (uuidBuf === undefined) uuidBuf = uuidNotBuffered = new Uint8Array(16);
    randomFillSync(uuidBuf);
  }

  // Variant byte: 10xxxxxx (variant 1)
  uuidBuf[8] = (uuidBuf[8] & 0x3f) | 0x80;

  // This function is structured the way it is for performance.
  // The uuid buffer stores the serialization of the random
  // bytes from uuidData.
  // xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  let n = 0;
  uuid[0] = kHexDigits[uuidBuf[n] >> 4];
  uuid[1] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[2] = kHexDigits[uuidBuf[n] >> 4];
  uuid[3] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[4] = kHexDigits[uuidBuf[n] >> 4];
  uuid[5] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[6] = kHexDigits[uuidBuf[n] >> 4];
  uuid[7] = kHexDigits[uuidBuf[n++] & 0xf];
  // -
  uuid[9] = kHexDigits[uuidBuf[n] >> 4];
  uuid[10] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[11] = kHexDigits[uuidBuf[n] >> 4];
  uuid[12] = kHexDigits[uuidBuf[n++] & 0xf];
  // -
  // 4, uuid[14] is set already...
  uuid[15] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[16] = kHexDigits[uuidBuf[n] >> 4];
  uuid[17] = kHexDigits[uuidBuf[n++] & 0xf];
  // -
  uuid[19] = kHexDigits[uuidBuf[n] >> 4];
  uuid[20] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[21] = kHexDigits[uuidBuf[n] >> 4];
  uuid[22] = kHexDigits[uuidBuf[n++] & 0xf];
  // -
  uuid[24] = kHexDigits[uuidBuf[n] >> 4];
  uuid[25] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[26] = kHexDigits[uuidBuf[n] >> 4];
  uuid[27] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[28] = kHexDigits[uuidBuf[n] >> 4];
  uuid[29] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[30] = kHexDigits[uuidBuf[n] >> 4];
  uuid[31] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[32] = kHexDigits[uuidBuf[n] >> 4];
  uuid[33] = kHexDigits[uuidBuf[n++] & 0xf];
  uuid[34] = kHexDigits[uuidBuf[n] >> 4];
  uuid[35] = kHexDigits[uuidBuf[n] & 0xf];

  return String.fromCharCode.apply(null, uuid);
}
