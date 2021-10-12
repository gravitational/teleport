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

export type Base64urlString = string;

// Sourced from https://github.com/github/webauthn-json
export function base64urlToBuffer(
  baseurl64String: Base64urlString
): ArrayBuffer {
  // Base64url to Base64
  const padding = '=='.slice(0, (4 - (baseurl64String.length % 4)) % 4);
  const base64String =
    baseurl64String.replace(/-/g, '+').replace(/_/g, '/') + padding;

  // Base64 to binary string
  const str = atob(base64String);

  // Binary string to buffer
  const buffer = new ArrayBuffer(str.length);
  const byteView = new Uint8Array(buffer);
  for (let i = 0; i < str.length; i++) {
    byteView[i] = str.charCodeAt(i);
  }
  return buffer;
}

export function bufferToBase64url(buffer: ArrayBuffer): Base64urlString {
  // Buffer to binary string
  const str = String.fromCharCode.apply(null, new Uint8Array(buffer));

  // Binary string to base64
  const base64String = btoa(str);

  // Base64 to base64url
  // We assume that the base64url string is well-formed.
  const base64urlString = base64String
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
  return base64urlString;
}
