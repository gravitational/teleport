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

/**
 * convenience constant equal to 2^32.
 */
const TWO_TO_32 = 4294967296;

export const MessageTypeEnum = {
  RAW: 'r',
  AUDIT: 'a',
  SESSION_DATA: 's',
  SESSION_END: 'c',
  RESIZE: 'w',
  FILE_TRANSFER_REQUEST: 'f',
  FILE_TRANSFER_DECISION: 't',
  MFA_CHALLENGE: 'n',
  READY_TO_JOIN: 'j',
  ERROR: 'e',
  LATENCY: 'l',
  KUBE_EXEC: 'k',
  DB_CONNECT: 'd',
  SESSION_STATUS: 'g',
  CHAT_MESSAGE: 'h',
};

export const messageFields = {
  payload: {
    code: 0x1a,
  },

  version: {
    code: 10,
    length: 1,
    values: {
      v1: 49,
    },
  },

  type: {
    length: 1,
    code: 0x12,
    values: {
      resize: MessageTypeEnum.RESIZE.charCodeAt(0),
      fileTransferRequest: MessageTypeEnum.FILE_TRANSFER_REQUEST.charCodeAt(0),
      fileTransferDecision:
        MessageTypeEnum.FILE_TRANSFER_DECISION.charCodeAt(0),
      readyToJoin: MessageTypeEnum.READY_TO_JOIN.charCodeAt(0),
      data: MessageTypeEnum.RAW.charCodeAt(0),
      event: MessageTypeEnum.AUDIT.charCodeAt(0),
      close: MessageTypeEnum.SESSION_END.charCodeAt(0),
      challengeResponse: MessageTypeEnum.MFA_CHALLENGE.charCodeAt(0),
      kubeExec: MessageTypeEnum.KUBE_EXEC.charCodeAt(0),
      error: MessageTypeEnum.ERROR.charCodeAt(0),
      dbConnect: MessageTypeEnum.DB_CONNECT.charCodeAt(0),
      chatMessage: MessageTypeEnum.CHAT_MESSAGE.charCodeAt(0),
    },
  },
};

export class Protobuf {
  encode(messageType, message) {
    var buffer = [];
    this.encodeVersion(buffer);
    this.encodeType(buffer, messageType);
    this.encodePayload(buffer, message);
    return buffer;
  }

  encodeResizeMessage(message) {
    return this.encode(messageFields.type.values.resize, message);
  }

  encodeChallengeResponse(message) {
    return this.encode(messageFields.type.values.challengeResponse, message);
  }

  encodeFileTransferRequest(message) {
    return this.encode(messageFields.type.values.fileTransferRequest, message);
  }

  encodeChatMessage(message) {
    return this.encode(messageFields.type.values.chatMessage, message);
  }

  encodeFileTransferDecision(message) {
    return this.encode(messageFields.type.values.fileTransferDecision, message);
  }

  encodeReadyToJoin() {
    return this.encode(messageFields.type.values.readyToJoin, '');
  }

  encodeKubeExecData(message) {
    return this.encode(messageFields.type.values.kubeExec, message);
  }

  encodeDbConnectData(message) {
    return this.encode(messageFields.type.values.dbConnect, message);
  }

  encodeRawMessage(message) {
    return this.encode(messageFields.type.values.data, message);
  }

  encodeCloseMessage() {
    // Close message has no payload
    return this.encode(messageFields.type.values.close, '');
  }

  encodePayload(buffer, text) {
    // set type
    buffer.push(messageFields.payload.code);

    // encode payload
    var uintArray = this.textToUintArray(text);
    this.encodeVarint(buffer, uintArray.length);
    for (var i = 0; i < uintArray.length; i++) {
      buffer.push(uintArray[i]);
    }
  }

  encodeVersion(buffer) {
    buffer[0] = messageFields.version.code;
    buffer[1] = messageFields.version.length;
    buffer[2] = messageFields.version.values.v1;
  }

  encodeType(buffer, typeValue) {
    buffer[3] = messageFields.type.code;
    buffer[4] = messageFields.type.length;
    buffer[5] = typeValue;
  }

  encodeVarint(buffer, value) {
    var lowBits = value >>> 0;
    var highBits = Math.floor((value - lowBits) / TWO_TO_32) >>> 0;
    while (highBits > 0 || lowBits > 127) {
      buffer.push((lowBits & 0x7f) | 0x80);
      lowBits = ((lowBits >>> 7) | (highBits << 25)) >>> 0;
      highBits = highBits >>> 7;
    }

    buffer.push(lowBits);
  }

  decode(uintArray) {
    var version = this.decodeVersion(uintArray);
    var type = this.decodeType(uintArray);
    var payload = this.decodePayload(uintArray);
    return {
      version,
      type,
      payload,
    };
  }

  decodeVersion(uintArray) {
    if (
      uintArray[0] === messageFields.version.code &&
      uintArray[1] === messageFields.version.length
    ) {
      return String.fromCharCode(uintArray[2]);
    }

    throw new Error('invalid version field');
  }

  decodeType(uintArray) {
    if (
      uintArray[3] === messageFields.type.code &&
      uintArray[4] === messageFields.type.length
    ) {
      return String.fromCharCode(uintArray[5]);
    }
    throw new Error('invalid type field');
  }

  decodePayload(uintArray) {
    if (!uintArray[6]) {
      return '';
    }

    if (uintArray[6] !== messageFields.payload.code) {
      throw new Error('invalid payload field');
    }

    const rawPayloadField = uintArray.slice(7);
    const [startsAt, payloadLength] = this.decodeVarint(rawPayloadField);
    const payloadBytes = rawPayloadField.slice(
      startsAt,
      startsAt + payloadLength
    );
    return this.uintArrayToText(payloadBytes);
  }

  decodeVarint(uintArray) {
    let x = 0;
    let s = 0;
    for (let i = 0; i < uintArray.length; i++) {
      var b = uintArray[i];
      if (b < 0x80) {
        if (i > 9 || (i == 9 && b > 1)) {
          throw new Error('unable to decode varint: overflow');
        }
        return [i + 1, x | (b << s)];
      }
      x = x | (b & (0x7f << s));
      s = s + 7;
    }

    throw new Error('unable to decode varint: empty array');
  }

  private textToUintArray(text: string): Uint8Array {
    return new TextEncoder().encode(text);
  }

  private uintArrayToText(uintArray: Uint8Array): string {
    return new TextDecoder('utf-8').decode(uintArray);
  }
}
