/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { arrayBufferToBase64, base64ToArrayBuffer } from './base64-arraybuffer';

export function base64urlToBuffer(base64url: string): ArrayBuffer {
  // Base64url to Base64string
  const padding = '=='.slice(0, (4 - (base64url.length % 4)) % 4);
  const base64String =
    base64url.replace(/-/g, '+').replace(/_/g, '/') + padding;

  return base64ToArrayBuffer(base64String);
}

export function bufferToBase64url(buffer: ArrayBuffer): string {
  const base64str = arrayBufferToBase64(buffer);

  // Assuming the base64str is a well-formed url.
  return base64str.replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
}
