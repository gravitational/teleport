// MIT License
//
// Copyright (c) 2019 - 2022 Dominik Geng (domske@github)
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

export class RingBuffer<T> {
  /**
   * Create a new ring buffer from an array.
   * @param data Array to import.
   * @param size Optional size. Otherwise same size as the input data array.
   */
  public static fromArray<T>(data: T[], size = 0): RingBuffer<T> {
    const ringBuffer = new RingBuffer<T>(size);
    ringBuffer.fromArray(data, size === 0);
    return ringBuffer;
  }

  protected buffer: T[] = [];
  protected size: number;
  protected pos = 0;

  /**
   * Creates a new ring buffer.
   * @param size Maximum size of the circular buffer.
   */
  constructor(size: number) {
    if (size < 0) {
      throw new RangeError('Invalid size.');
    }
    this.size = size;
  }

  /**
   * Returns the maximum size.
   */
  public getSize(): number {
    return this.size;
  }

  /**
   * Returns the next internal index position.
   */
  public getPos(): number {
    return this.pos;
  }

  /**
   * Returns the current buffer size.
   * The underlying array length.
   */
  public getBufferLength(): number {
    return this.buffer.length;
  }

  /**
   * Pushes new items to this buffer. (like Array#push)
   * @param items Items to push to this buffer.
   */
  public add(...items: T[]): void {
    items.forEach(item => {
      this.buffer[this.pos] = item;
      this.pos = (this.pos + 1) % this.size;
    });
  }

  /**
   * Returns the item on specific index.
   * @param index Index of item.
   */
  public get(index: number): T | undefined {
    if (index < 0) {
      index += this.buffer.length;
    }

    if (index < 0 || index > this.buffer.length) {
      return undefined;
    }

    if (this.buffer.length < this.size) {
      return this.buffer[index];
    }

    return this.buffer[(this.pos + index) % this.size];
  }

  /**
   * Returns the first item.
   * Same as #get(0).
   */
  public getFirst(): T | undefined {
    return this.get(0);
  }

  /**
   * Returns the last item.
   * Same as #get(-1) or #get(length - 1).
   */
  public getLast(): T | undefined {
    return this.get(-1);
  }

  /**
   * Return the first `n` items as an array.
   * @param n Number of items to return.
   */
  public getFirstN(n: number): T[] {
    if (n === 0) {
      return [];
    }
    if (n < 0) {
      return this.getLastN(-n);
    }
    return this.toArray().slice(0, n);
  }

  /**
   * Return the last `n` items as an array.
   * @param n Number of items to return.
   */
  public getLastN(n: number): T[] {
    if (n === 0) {
      return [];
    }
    if (n < 0) {
      return this.getFirstN(-n);
    }
    return this.toArray().slice(-n);
  }

  /**
   * Removes one or more items form the buffer.
   * @param index Start index of the item to remove.
   * @param count The count of items to remove. (default: 1)
   */
  public remove(index: number, count = 1): T[] {
    if (index < 0) {
      index += this.buffer.length;
    }

    if (index < 0 || index > this.buffer.length) {
      return [];
    }

    const arr = this.toArray();
    const removedItems = arr.splice(index, count);
    this.fromArray(arr);
    return removedItems;
  }

  /**
   * Removes the first item. Like #remove(0).
   */
  public removeFirst(): T {
    return this.remove(0)[0];
  }

  /**
   * Removes the last item. Like #remove(-1).
   */
  public removeLast(): T {
    return this.remove(-1)[0];
  }

  /**
   * Converts the ring buffer to a JavaScript standard array.
   */
  public toArray(): T[] {
    return this.buffer.slice(this.pos).concat(this.buffer.slice(0, this.pos));
  }

  /**
   * Imports an array to this buffer. (overwrites)
   * @param data JavaScript standard array.
   * @param resize If true, sets the maximum size to the input data array length.
   */
  public fromArray(data: T[], resize = false): void {
    if (!Array.isArray(data)) {
      throw new TypeError('Input value is not an array.');
    }

    if (resize) {
      this.resize(data.length);
    }

    if (this.size === 0) {
      return;
    }

    this.buffer = data.slice(-this.size);
    this.pos = this.buffer.length % this.size;
  }

  /**
   * Clears the the buffer. Removes all items.
   */
  public clear(): void {
    this.buffer = [];
    this.pos = 0;
  }

  /**
   * Resizes the circular buffer.
   * @param newSize The new maximum size.
   */
  public resize(newSize: number): void {
    if (newSize < 0) {
      throw new RangeError('The size does not allow negative values.');
    }

    if (newSize === 0) {
      this.clear();
    } else if (newSize !== this.size) {
      const currentBuffer = this.toArray();
      this.fromArray(currentBuffer.slice(-newSize));
      this.pos = this.buffer.length % newSize;
    }

    this.size = newSize;
  }

  /**
   * Returns true if the maximum size is reached.
   */
  public isFull(): boolean {
    return this.buffer.length === this.size;
  }

  /**
   * Returns true if the buffer is empty.
   */
  public isEmpty(): boolean {
    return this.buffer.length === 0;
  }
}
