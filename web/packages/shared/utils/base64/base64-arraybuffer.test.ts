// Modified from https://github.com/niklasvh/base64-arraybuffer/

// Copyright (c) 2012 Niklas von Hertzen

// Permission is hereby granted, free of charge, to any person
// obtaining a copy of this software and associated documentation
// files (the "Software"), to deal in the Software without
// restriction, including without limitation the rights to use,
// copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following
// conditions:

// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
// OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
// HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
// WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.

import { arrayBufferToBase64, base64ToArrayBuffer } from './base64-arraybuffer';

function stringArrayBuffer(str: string) {
  const buffer = new ArrayBuffer(str.length);
  const bytes = new Uint8Array(buffer);

  str.split('').forEach(function (str, i) {
    bytes[i] = str.charCodeAt(0);
  });

  return buffer;
}

function testArrayBuffers(buffer1: ArrayBuffer, buffer2: ArrayBuffer) {
  const len1 = buffer1.byteLength;
  const len2 = buffer2.byteLength;
  const view1 = new Uint8Array(buffer1);
  const view2 = new Uint8Array(buffer2);

  if (len1 !== len2) {
    return false;
  }

  for (let i = 0; i < len1; i++) {
    if (view1[i] === undefined || view1[i] !== view2[i]) {
      return false;
    }
  }
  return true;
}

function rangeArrayBuffer() {
  const buffer = new ArrayBuffer(256);
  const bytes = new Uint8Array(buffer);
  for (let i = 0; i < 256; i++) {
    bytes[i] = i;
  }

  return buffer;
}

describe('encode', () => {
  it('encode "Hello world"', () =>
    expect(arrayBufferToBase64(stringArrayBuffer('Hello world'))).toBe(
      'SGVsbG8gd29ybGQ='
    ));
  it('encode "Man"', () =>
    expect(arrayBufferToBase64(stringArrayBuffer('Man'))).toBe('TWFu'));
  it('encode "Ma"', () =>
    expect(arrayBufferToBase64(stringArrayBuffer('Ma'))).toBe('TWE='));
  it('encode "Hello worlds!"', () =>
    expect(arrayBufferToBase64(stringArrayBuffer('Hello worlds!'))).toBe(
      'SGVsbG8gd29ybGRzIQ=='
    ));
  it('encode all binary characters', () =>
    expect(arrayBufferToBase64(rangeArrayBuffer())).toBe(
      'AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJTVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsbKztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/w=='
    ));
});

describe('decode', () => {
  it('decode "Man"', () =>
    expect(
      testArrayBuffers(base64ToArrayBuffer('TWFu'), stringArrayBuffer('Man'))
    ).toBeTruthy());
  it('decode "Hello world"', () =>
    expect(
      testArrayBuffers(
        base64ToArrayBuffer('SGVsbG8gd29ybGQ='),
        stringArrayBuffer('Hello world')
      )
    ).toBeTruthy());
  it('decode "Hello worlds!"', () =>
    expect(
      testArrayBuffers(
        base64ToArrayBuffer('SGVsbG8gd29ybGRzIQ=='),
        stringArrayBuffer('Hello worlds!')
      )
    ).toBeTruthy());
  it('decode all binary characters', () =>
    expect(
      testArrayBuffers(
        base64ToArrayBuffer(
          'AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJTVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsbKztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/w=='
        ),
        rangeArrayBuffer()
      )
    ).toBeTruthy());
});
