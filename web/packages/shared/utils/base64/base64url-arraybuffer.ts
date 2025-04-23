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

import { arrayBufferToBase64, base64ToArrayBuffer } from './base64-arraybuffer';

export function base64urlToBuffer(base64url: string): ArrayBuffer {
  // Base64url to Base64string
  const padding = '=='.slice(0, (4 - (base64url.length % 4)) % 4);
  const base64String =
    base64url.replace(/-/g, '+').replace(/_/g, '/') + padding;

  return base64ToArrayBuffer(base64String);
}

export function bufferToBase64url(buffer: ArrayBufferLike): string {
  const base64str = arrayBufferToBase64(buffer);

  // Assuming the base64str is a well-formed url.
  return base64str.replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
}
