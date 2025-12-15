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
  EventType,
  type SerializedTerminal,
  type TerminalSize,
  type TtyEvent,
} from './types';

const responseHeaderSize = 17;

export function decodeTtyEvent(buffer: ArrayBuffer): TtyEvent {
  if (buffer.byteLength < responseHeaderSize) {
    throw new Error('Event too short');
  }

  const view = new DataView(buffer);
  const eventType = view.getUint8(0);
  const timestamp = Number(view.getBigInt64(1));
  const dataLength = view.getUint32(9);

  if (buffer.byteLength < responseHeaderSize + dataLength) {
    throw new Error('Incomplete event data');
  }

  const data = new Uint8Array(buffer, responseHeaderSize, dataLength);

  const requestId = view.getUint32(13);

  switch (eventType) {
    case EventType.Resize:
      return {
        requestId,
        terminalSize: decodeTerminalSize(data),
        timestamp,
        type: eventType,
      };

    case EventType.Screen:
      return {
        requestId,
        screen: decodeSerializedTerminal(data),
        timestamp,
        type: eventType,
      };

    case EventType.SessionEnd:
      return { requestId, timestamp, type: eventType };

    case EventType.SessionPrint:
      return {
        data,
        requestId,
        timestamp,
        type: eventType,
      };

    case EventType.SessionStart:
      return {
        requestId,
        terminalSize: decodeTerminalSize(data),
        timestamp,
        type: eventType,
      };
  }
}

function decodeSerializedTerminal(data: Uint8Array): SerializedTerminal {
  if (data.length < responseHeaderSize) {
    throw new Error('Serialized terminal data too short');
  }

  const view = new DataView(
    data.buffer,
    data.byteOffset + 1,
    data.byteLength - 1
  );

  const cols = view.getUint32(0, false);
  const rows = view.getUint32(4, false);
  const cursorX = view.getUint32(8, false);
  const cursorY = view.getUint32(12, false);
  const dataLength = view.getUint32(16, false);

  const totalLength = responseHeaderSize + dataLength;

  if (data.length < totalLength) {
    throw new Error('Incomplete serialized terminal data');
  }

  return {
    cols,
    cursorX,
    cursorY,
    data: data.subarray(responseHeaderSize, totalLength),
    rows,
  };
}

function decodeTerminalSize(data: Uint8Array): TerminalSize {
  const decoder = new TextDecoder();

  const size = decoder.decode(data);
  const [cols, rows] = size.split(':').map(Number);

  if (isNaN(cols) || isNaN(rows)) {
    throw new Error('Invalid terminal size format');
  }

  return { cols, rows };
}
