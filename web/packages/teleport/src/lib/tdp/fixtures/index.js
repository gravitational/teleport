// Copyright 2021 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The objects loaded from json below are the results of websocket onmessage calls handled by a TdpClient (packages/teleport/src/lib/tdp/client.ts).
// The messages are each represented by a Uint8Array, and were saved by setting the saveMessages parameter (now deprecated, see commit ece61f56462b94e44d3a2231acb1738e17e10c6a)
// of the TdpClient object (packages/teleport/src/lib/tdp/client.ts) to true, allowing messages to be passed per normal operation of the client, and then saving the object printed in the console upon
// calling "disconnect". Here, they are converted from their json Uint8Array[] back into an ArrayBuffer[], which can then be used to simulate a realistic
// sequence of onmessage calls for development/performance-testing purposes.
//
// The array of Uint8Arrays gets saved to JSON in a somewhat awkward manner, with each Uint8Array saved as an object with {string: number} keypairs
// where the string key represents the index in the array, and its value represents the actual byte data of the message. Fortunately, the keys
// are all arranged in the proper order (from 0 to N), so it's not too much trouble to convert
// [
//   { '0': 2, '1': 45 , ...}, // first message received
//   { '0': 78, '1': 0 , ...}, // second message received, etc.
//   ...
// ]
//
// to
//
// [
//   Uint8Array([2, 45, ...]),
//   Uint8Array([78, 0, ...]),
//   ...
// ]
//
// and ultimately to
//
// [
//   ArrayBuffer(Uint8Array([2, 45, ...])),
//   ArrayBuffer(Uint8Array([78, 0, ...])),
//   ...
// ]

import uint82260x1130 from './2260x1130.json';
import uint8first2pngs from './first2pngs.json';

function convert2ArrayBuf(data) {
  const arrayBuf = [];
  const array = data;
  array.forEach(obj => {
    let uint8array = new Uint8Array(Object.keys(obj).map(key => obj[key]));
    arrayBuf.push(uint8array.buffer);
  });
  return arrayBuf;
}

export const arrayBuf2260x1130 = convert2ArrayBuf(uint82260x1130);
export const arrayBufFirst2Pngs = convert2ArrayBuf(uint8first2pngs);
