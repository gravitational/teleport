/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import {
  ResponseType,
  type BaseEvent,
  type BatchEvent,
  type StreamEvent,
} from './types';

const responseHeaderSize = 17;

export function parseArrayBuffer<
  T extends BaseEvent<TType>,
  TType extends number = number,
>(
  buffer: ArrayBuffer,
  decodeEvent: (buffer: ArrayBuffer) => T
): StreamEvent<T> | T {
  if (buffer.byteLength < responseHeaderSize) {
    throw new Error('Event too short');
  }

  const view = new DataView(buffer);
  const responseType = view.getUint8(0) as ResponseType;
  const timestamp = Number(view.getBigInt64(1));
  const dataLength = view.getUint32(9);

  if (buffer.byteLength < responseHeaderSize + dataLength) {
    throw new Error('Incomplete event data');
  }

  const data = new Uint8Array(buffer, responseHeaderSize, dataLength);

  const requestId = view.getUint32(13);

  switch (responseType) {
    case ResponseType.Batch:
      return parseBatchEvent(buffer, decodeEvent);

    case ResponseType.Error:
      return {
        error: new TextDecoder().decode(data),

        requestId,
        timestamp,
        type: responseType,
      };

    case ResponseType.Start:
      return { requestId, timestamp, type: responseType };

    case ResponseType.Stop:
      const startTime = Number(view.getBigInt64(responseHeaderSize));
      const endTime = Number(view.getBigInt64(25));

      return {
        endTime,
        requestId,
        startTime,
        timestamp,
        type: responseType,
      };

    default:
      return decodeEvent(buffer);
  }
}

function parseBatchEvent<T>(
  buffer: ArrayBuffer,
  decodeEvent: (buffer: ArrayBuffer) => T
): BatchEvent<T> {
  if (buffer.byteLength < responseHeaderSize) {
    throw new Error('Batch event header too short');
  }

  const headerView = new DataView(buffer);
  const batchCount = headerView.getUint32(1);
  const requestId = headerView.getUint32(5);

  const events: T[] = [];

  let offset = responseHeaderSize;

  for (let i = 0; i < batchCount; i++) {
    if (offset + responseHeaderSize > buffer.byteLength) {
      throw new Error(`Incomplete batch event at index ${i}`);
    }

    const eventView = new DataView(buffer, offset);
    const dataLength = eventView.getUint32(9);

    if (offset + responseHeaderSize + dataLength > buffer.byteLength) {
      throw new Error(`Incomplete event data at index ${i}`);
    }

    const eventBuffer = new ArrayBuffer(responseHeaderSize + dataLength);
    const eventBytes = new Uint8Array(eventBuffer);
    eventBytes.set(
      new Uint8Array(buffer, offset, responseHeaderSize + dataLength)
    );

    const parsedEvent = decodeEvent(eventBuffer);
    events.push(parsedEvent);

    offset += responseHeaderSize + dataLength;
  }

  return {
    events,
    requestId,
    timestamp: 0,
    type: ResponseType.Batch,
  };
}
