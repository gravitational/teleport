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

import { RequestType, type FetchRequest } from './types';

const requestHeaderSize = 21;

export function encodeFetchRequest(request: FetchRequest): ArrayBuffer {
  const buffer = new ArrayBuffer(22);
  const view = new DataView(buffer);

  view.setUint8(0, RequestType.Fetch);
  view.setBigInt64(1, BigInt(Math.floor(request.startTime)));
  view.setBigInt64(9, BigInt(Math.floor(request.endTime)));
  view.setUint32(17, request.requestId);

  if (request.requestCurrentScreen !== undefined) {
    view.setUint8(requestHeaderSize, request.requestCurrentScreen ? 1 : 0);
  }

  return buffer;
}
