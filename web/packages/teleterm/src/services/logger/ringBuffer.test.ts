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

import { RingBuffer } from './ringBuffer';

interface TestObject {
  id: number;
  name: string;
}

test('Create ring buffer.', () => {
  const ringBuffer = new RingBuffer(10);
  expect(ringBuffer).toBeTruthy();
  expect(ringBuffer.getSize()).toBe(10);
});

test('Add items.', () => {
  const ringBuffer = new RingBuffer<number>(5);
  expect(ringBuffer.getSize()).toBe(5);
  expect(ringBuffer.getBufferLength()).toBe(0);
  expect(ringBuffer.getPos()).toBe(0);
  expect(ringBuffer.isFull()).toBeFalsy();
  ringBuffer.add(42);
  expect(ringBuffer.getBufferLength()).toBe(1);
  expect(ringBuffer.getPos()).toBe(1);
  expect(ringBuffer.isFull()).toBeFalsy();
  ringBuffer.add(27, 87);
  expect(ringBuffer.getBufferLength()).toBe(3);
  expect(ringBuffer.getPos()).toBe(3);
  expect(ringBuffer.isFull()).toBeFalsy();
  ringBuffer.add(7, 12, 0, 417);
  expect(ringBuffer.getBufferLength()).toBe(5);
  expect(ringBuffer.getPos()).toBe(2);
  expect(ringBuffer.isFull()).toBeTruthy();
});

test('Clear buffer.', () => {
  const ringBuffer = new RingBuffer<number>(7);
  ringBuffer.add(1, 2, 3, 4, 5, 6, 7);
  expect(ringBuffer.getPos()).toBe(0);
  expect(ringBuffer.isFull()).toBeTruthy();
  expect(ringBuffer.toArray()).toEqual([1, 2, 3, 4, 5, 6, 7]);
  ringBuffer.clear();
  expect(ringBuffer.getPos()).toBe(0);
  expect(ringBuffer.isFull()).toBeFalsy();
  expect(ringBuffer.toArray().length).toBe(0);
  ringBuffer.add(1, 2, 3);
  expect(ringBuffer.getPos()).toBe(3);
  expect(ringBuffer.toArray()).toEqual([1, 2, 3]);
  ringBuffer.clear();
  expect(ringBuffer.getPos()).toBe(0);
  expect(ringBuffer.isFull()).toBeFalsy();
  expect(ringBuffer.toArray().length).toBe(0);
});

test('To array.', () => {
  const ringBuffer = new RingBuffer<number>(7);
  ringBuffer.add(1, 2, 3, 4, 5);
  ringBuffer.add(6, 7, 8, 9);
  ringBuffer.add(10);
  expect(ringBuffer.toArray()).toEqual([4, 5, 6, 7, 8, 9, 10]);
});

test('From array.', () => {
  const ringBuffer = new RingBuffer<string>(4);
  ringBuffer.fromArray(['One', 'Two', 'Three', 'Four', 'Five']);
  expect(ringBuffer.toArray()).toEqual(['Two', 'Three', 'Four', 'Five']);
});

test('From array invalid input.', () => {
  const ringBuffer = new RingBuffer<string>(4);
  expect(() => {
    ringBuffer.fromArray(undefined as any);
  }).toThrow(TypeError);

  expect(() => {
    (ringBuffer as any).fromArray(42);
  }).toThrow(TypeError);

  expect(() => {
    (ringBuffer as any).fromArray(null);
  }).toThrow(TypeError);

  expect(() => {
    (ringBuffer as any).fromArray({ one: 1, two: 2, three: 3 });
  }).toThrow(TypeError);

  expect(() => {
    (ringBuffer as any).fromArray(true);
  }).toThrow(TypeError);

  expect(() => {
    (ringBuffer as any).fromArray('hehe');
  }).toThrow(TypeError);
});

test('From array smaller input.', () => {
  const ringBuffer = new RingBuffer<string>(3);
  ringBuffer.fromArray(['One', 'Two']);
  expect(ringBuffer.getBufferLength()).toBe(2);
  expect(ringBuffer.getSize()).toBe(3);
  expect(ringBuffer.getPos()).toBe(2);
  expect(ringBuffer.toArray()).toEqual(['One', 'Two']);
  ringBuffer.add('Three', 'Four');
  expect(ringBuffer.toArray()).toEqual(['Two', 'Three', 'Four']);
});

test('From array with resize.', () => {
  const ringBuffer = new RingBuffer<string>(4);
  ringBuffer.fromArray(['One', 'Two', 'Three', 'Four', 'Five'], true);
  expect(ringBuffer.toArray()).toEqual(['One', 'Two', 'Three', 'Four', 'Five']);
});

test('From array with resize zero size.', () => {
  const ringBuffer = new RingBuffer<string>(4);
  ringBuffer.fromArray([], true);
  expect(ringBuffer.getSize()).toBe(0);
  expect(ringBuffer.getPos()).toBe(0);
  expect(ringBuffer.toArray()).toEqual([]);
});

test('Resize up.', () => {
  const ringBuffer = new RingBuffer<boolean>(3);
  expect(ringBuffer.getSize()).toBe(3);
  expect(ringBuffer.getPos()).toBe(0);
  ringBuffer.add(true, false, false);
  expect(ringBuffer.getPos()).toBe(0);
  ringBuffer.resize(5);
  ringBuffer.add(false, true, false);
  expect(ringBuffer.getSize()).toBe(5);
  expect(ringBuffer.getPos()).toBe(1);
  expect(ringBuffer.toArray()).toEqual([false, false, false, true, false]);
});

test('Resize down.', () => {
  const ringBuffer = new RingBuffer<boolean>(3);
  expect(ringBuffer.getSize()).toBe(3);
  expect(ringBuffer.getPos()).toBe(0);
  ringBuffer.add(true, false, false, true);
  expect(ringBuffer.getPos()).toBe(1);
  expect(ringBuffer.toArray()).toEqual([false, false, true]);
  ringBuffer.resize(2);
  ringBuffer.add(false);
  expect(ringBuffer.getSize()).toBe(2);
  expect(ringBuffer.toArray()).toEqual([true, false]);
});

test('Resize same size.', () => {
  const ringBuffer = new RingBuffer<number>(3);
  ringBuffer.add(1, 2, 3);
  ringBuffer.resize(3);
  expect(ringBuffer.getSize()).toBe(3);
  ringBuffer.add(4);
  expect(ringBuffer.toArray()).toEqual([2, 3, 4]);
});

test('Error on new with negative size.', () => {
  expect(() => {
    return new RingBuffer<any>(-3);
  }).toThrow(RangeError);
});

test('Resize error on negative size.', () => {
  expect(() => {
    const ringBuffer = new RingBuffer<any>(3);
    ringBuffer.resize(-2);
  }).toThrow(RangeError);
});

test('Is full.', () => {
  const ringBuffer = new RingBuffer<number>(3);
  expect(ringBuffer.isFull()).toBeFalsy();
  ringBuffer.add(1);
  expect(ringBuffer.isFull()).toBeFalsy();
  ringBuffer.add(2, 3);
  expect(ringBuffer.isFull()).toBeTruthy();
  ringBuffer.add(4);
  expect(ringBuffer.isFull()).toBeTruthy();
  ringBuffer.clear();
  expect(ringBuffer.isFull()).toBeFalsy();
});

test('Is empty.', () => {
  const ringBuffer = new RingBuffer<number>(3);
  expect(ringBuffer.isEmpty()).toBeTruthy();
  ringBuffer.add(1);
  expect(ringBuffer.isEmpty()).toBeFalsy();
  ringBuffer.add(2, 3);
  expect(ringBuffer.isEmpty()).toBeFalsy();
  ringBuffer.clear();
  expect(ringBuffer.isEmpty()).toBeTruthy();
});

test('Get item.', () => {
  const ringBuffer = new RingBuffer<number>(5);
  ringBuffer.add(27, 42, 87);
  expect(ringBuffer.get(1)).toBe(42);
  ringBuffer.add(417, 7, 66);
  expect(ringBuffer.get(0)).toBe(42);
  expect(ringBuffer.get(1)).toBe(87);
  expect(ringBuffer.get(-1)).toBe(66);
  expect(ringBuffer.get(-2)).toBe(7);
  expect(ringBuffer.get(8)).toBe(undefined);
  expect(ringBuffer.get(-8)).toBe(undefined);
});

test('Get first.', () => {
  const ringBuffer = new RingBuffer<number>(5);
  expect(ringBuffer.getFirst()).toBe(undefined);
  ringBuffer.add(1, 2, 3);
  expect(ringBuffer.getFirst()).toBe(1);
  ringBuffer.add(4, 5);
  expect(ringBuffer.getFirst()).toBe(1);
  ringBuffer.add(6, 7);
  expect(ringBuffer.getFirst()).toBe(3);
});

test('Get last.', () => {
  const ringBuffer = new RingBuffer<number>(5);
  expect(ringBuffer.getLast()).toBe(undefined);
  ringBuffer.add(1, 2, 3);
  expect(ringBuffer.getLast()).toBe(3);
  ringBuffer.add(4, 5);
  expect(ringBuffer.getLast()).toBe(5);
  ringBuffer.add(6, 7);
  expect(ringBuffer.getLast()).toBe(7);
});

test('Create from array.', () => {
  const ringBuffer = RingBuffer.fromArray([10, 20, 30]);
  expect(ringBuffer).toBeTruthy();
  expect(ringBuffer.getSize()).toBe(3);
  expect(ringBuffer.toArray()).toEqual([10, 20, 30]);
});

test('Create from array with fixed size.', () => {
  const ringBuffer = RingBuffer.fromArray([10, 20, 30], 2);
  expect(ringBuffer).toBeTruthy();
  expect(ringBuffer.getSize()).toBe(2);
  expect(ringBuffer.toArray()).toEqual([20, 30]);
});

test('Complete circular test.', () => {
  const ringBuffer = new RingBuffer<TestObject>(5);

  ringBuffer.add({ id: 1, name: 'One' });
  expect(ringBuffer.getFirst()).toEqual({ id: 1, name: 'One' });
  expect(ringBuffer.getLast()).toEqual({ id: 1, name: 'One' });
  expect(ringBuffer.get(0)).toEqual({ id: 1, name: 'One' });
  expect(ringBuffer.get(-1)).toEqual({ id: 1, name: 'One' });
  expect(ringBuffer.get(2)).toEqual(undefined);
  expect(ringBuffer.get(-2)).toEqual(undefined);

  ringBuffer.add({ id: 2, name: 'Two' });
  expect(ringBuffer.getFirst()).toEqual({ id: 1, name: 'One' });
  expect(ringBuffer.getLast()).toEqual({ id: 2, name: 'Two' });

  ringBuffer.add({ id: 3, name: 'Three' });
  expect(ringBuffer.getFirst()).toEqual({ id: 1, name: 'One' });
  expect(ringBuffer.get(1)).toEqual({ id: 2, name: 'Two' });
  expect(ringBuffer.getLast()).toEqual({ id: 3, name: 'Three' });

  ringBuffer.add({ id: 4, name: 'Four' });
  expect(ringBuffer.getFirst()).toEqual({ id: 1, name: 'One' });
  expect(ringBuffer.get(1)).toEqual({ id: 2, name: 'Two' });
  expect(ringBuffer.get(2)).toEqual({ id: 3, name: 'Three' });
  expect(ringBuffer.getLast()).toEqual({ id: 4, name: 'Four' });

  ringBuffer.add({ id: 5, name: 'Five' });
  expect(ringBuffer.getFirst()).toEqual({ id: 1, name: 'One' });
  expect(ringBuffer.get(1)).toEqual({ id: 2, name: 'Two' });
  expect(ringBuffer.get(2)).toEqual({ id: 3, name: 'Three' });
  expect(ringBuffer.get(3)).toEqual({ id: 4, name: 'Four' });
  expect(ringBuffer.getLast()).toEqual({ id: 5, name: 'Five' });

  ringBuffer.add({ id: 6, name: 'Six' });
  expect(ringBuffer.getFirst()).toEqual({ id: 2, name: 'Two' });
  expect(ringBuffer.get(1)).toEqual({ id: 3, name: 'Three' });
  expect(ringBuffer.get(2)).toEqual({ id: 4, name: 'Four' });
  expect(ringBuffer.get(3)).toEqual({ id: 5, name: 'Five' });
  expect(ringBuffer.getLast()).toEqual({ id: 6, name: 'Six' });

  ringBuffer.add({ id: 7, name: 'Seven' });
  expect(ringBuffer.getFirst()).toEqual({ id: 3, name: 'Three' });
  expect(ringBuffer.get(0)).toEqual({ id: 3, name: 'Three' });
  expect(ringBuffer.get(1)).toEqual({ id: 4, name: 'Four' });
  expect(ringBuffer.get(2)).toEqual({ id: 5, name: 'Five' });
  expect(ringBuffer.get(3)).toEqual({ id: 6, name: 'Six' });
  expect(ringBuffer.get(4)).toEqual({ id: 7, name: 'Seven' });
  expect(ringBuffer.getLast()).toEqual({ id: 7, name: 'Seven' });

  const arrayExport = ringBuffer.toArray();

  expect(arrayExport).toEqual([
    { id: 3, name: 'Three' },
    { id: 4, name: 'Four' },
    { id: 5, name: 'Five' },
    { id: 6, name: 'Six' },
    { id: 7, name: 'Seven' },
  ]);

  const anotherRingBuffer = new RingBuffer(5);
  anotherRingBuffer.fromArray(arrayExport);

  expect(anotherRingBuffer.getFirst()).toEqual({ id: 3, name: 'Three' });
  expect(anotherRingBuffer.getLast()).toEqual({ id: 7, name: 'Seven' });

  anotherRingBuffer.resize(3);
  expect(anotherRingBuffer.getFirst()).toEqual({ id: 5, name: 'Five' });
  expect(anotherRingBuffer.getLast()).toEqual({ id: 7, name: 'Seven' });
});

test('Remove items.', () => {
  const ringBuffer = new RingBuffer<number>(5);
  ringBuffer.add(1, 2, 3);
  expect(ringBuffer.remove(1)).toEqual([2]);
  expect(ringBuffer.toArray()).toEqual([1, 3]);

  ringBuffer.add(4, 5, 6, 7, 8, 9);
  expect(ringBuffer.remove(2, 2)).toEqual([7, 8]);
  expect(ringBuffer.toArray()).toEqual([5, 6, 9]);
});

test('Remove first item', () => {
  const ringBuffer = new RingBuffer<number>(3);
  ringBuffer.add(1, 2, 3);
  ringBuffer.removeFirst();
  expect(ringBuffer.toArray()).toEqual([2, 3]);
});

test('Remove last item', () => {
  const ringBuffer = new RingBuffer<number>(3);
  ringBuffer.add(1, 2, 3);
  ringBuffer.removeLast();
  expect(ringBuffer.toArray()).toEqual([1, 2]);
});

test('Remove out of bounds.', () => {
  const ringBuffer = new RingBuffer<number>(3);
  ringBuffer.add(1, 2, 3);
  expect(ringBuffer.remove(42)).toEqual([]);
  expect(ringBuffer.toArray()).toEqual([1, 2, 3]);
});

test('Get first N items.', () => {
  const ringBuffer = new RingBuffer<number>(7);
  ringBuffer.add(1, 2, 3, 4, 5, 6, 7);
  expect(ringBuffer.getFirstN(1)).toEqual([1]);
  expect(ringBuffer.getFirstN(3)).toEqual([1, 2, 3]);
  ringBuffer.add(8, 9, 10);
  expect(ringBuffer.getFirstN(3)).toEqual([4, 5, 6]);
  expect(ringBuffer.getFirstN(6)).toEqual([4, 5, 6, 7, 8, 9]);
  expect(ringBuffer.getFirstN(7)).toEqual([4, 5, 6, 7, 8, 9, 10]);
  ringBuffer.add(11, 12, 13, 14);
  expect(ringBuffer.getFirstN(1)).toEqual([8]);
  expect(ringBuffer.getFirstN(0)).toEqual([]);
  expect(ringBuffer.getFirstN(10)).toEqual([8, 9, 10, 11, 12, 13, 14]);
  expect(ringBuffer.getFirstN(-1)).toEqual([14]);
  expect(ringBuffer.getFirstN(-3)).toEqual([12, 13, 14]);
  expect(ringBuffer.getFirstN(-10)).toEqual([8, 9, 10, 11, 12, 13, 14]);
});

test('Get last N items.', () => {
  const ringBuffer = new RingBuffer<number>(7);
  ringBuffer.add(1, 2, 3, 4, 5, 6, 7);
  expect(ringBuffer.getLastN(1)).toEqual([7]);
  expect(ringBuffer.getLastN(3)).toEqual([5, 6, 7]);
  ringBuffer.add(8, 9, 10);
  expect(ringBuffer.getLastN(3)).toEqual([8, 9, 10]);
  expect(ringBuffer.getLastN(6)).toEqual([5, 6, 7, 8, 9, 10]);
  expect(ringBuffer.getLastN(7)).toEqual([4, 5, 6, 7, 8, 9, 10]);
  ringBuffer.add(11, 12, 13, 14);
  expect(ringBuffer.getLastN(1)).toEqual([14]);
  expect(ringBuffer.getLastN(0)).toEqual([]);
  expect(ringBuffer.getLastN(10)).toEqual([8, 9, 10, 11, 12, 13, 14]);
  expect(ringBuffer.getLastN(-1)).toEqual([8]);
  expect(ringBuffer.getLastN(-3)).toEqual([8, 9, 10]);
  expect(ringBuffer.getLastN(-10)).toEqual([8, 9, 10, 11, 12, 13, 14]);
});

test('Example 1', () => {
  const ringBuffer = new RingBuffer<number>(5);
  ringBuffer.add(1);
  ringBuffer.add(2, 3);
  ringBuffer.add(4, 5, 6);
  expect(ringBuffer.toArray()).toEqual([2, 3, 4, 5, 6]);
});

test('Example 2', () => {
  const ringBuffer = new RingBuffer<number>(5);
  ringBuffer.add(11);
  ringBuffer.add(22);
  ringBuffer.add(33);
  expect(ringBuffer.toArray()).toEqual([11, 22, 33]);
  ringBuffer.remove(1);
  expect(ringBuffer.toArray()).toEqual([11, 33]);
  ringBuffer.add(44, 55);
  expect(ringBuffer.toArray()).toEqual([11, 33, 44, 55]);
  ringBuffer.removeFirst();
  ringBuffer.removeLast();
  expect(ringBuffer.toArray()).toEqual([33, 44]);
  ringBuffer.add(66, 77, 88, 99);
  ringBuffer.remove(2, 2);
  expect(ringBuffer.toArray()).toEqual([44, 66, 99]);
});
