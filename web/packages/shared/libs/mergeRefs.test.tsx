/* MIT License

Copyright (c) 2020 Greg Bergé

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE. */

import React, { useImperativeHandle, forwardRef } from 'react';
import { render } from '@testing-library/react';

import { mergeRefs } from './mergeRefs';

test('mergeRefs', () => {
  const Dummy = forwardRef(function Dummy(_, ref) {
    useImperativeHandle(ref, () => 'refValue');
    return null;
  });
  const refAsFunc = jest.fn();
  const refAsObj = { current: undefined };
  const Example: React.FC<{ visible: boolean }> = ({ visible }) => {
    return visible ? <Dummy ref={mergeRefs([refAsObj, refAsFunc])} /> : null;
  };
  const { rerender } = render(<Example visible />);
  expect(refAsFunc).toHaveBeenCalledTimes(1);
  expect(refAsFunc).toHaveBeenCalledWith('refValue');
  expect(refAsObj.current).toBe('refValue');
  rerender(<Example visible={false} />);
  expect(refAsFunc).toHaveBeenCalledTimes(2);
  expect(refAsFunc).toHaveBeenCalledWith(null);
  expect(refAsObj.current).toBe(null);
});

test('mergeRefs with undefined and null refs', () => {
  const Dummy = forwardRef(function Dummy(_, ref) {
    useImperativeHandle(ref, () => 'refValue');
    return null;
  });
  const refAsFunc = jest.fn();
  const refAsObj = { current: undefined };
  const Example: React.FC<{ visible: boolean }> = ({ visible }) => {
    return visible ? (
      <Dummy ref={mergeRefs([null, undefined, refAsFunc, refAsObj])} />
    ) : null;
  };
  const { rerender } = render(<Example visible />);
  expect(refAsFunc).toHaveBeenCalledTimes(1);
  expect(refAsFunc).toHaveBeenCalledWith('refValue');
  expect(refAsObj.current).toBe('refValue');
  rerender(<Example visible={false} />);
  expect(refAsFunc).toHaveBeenCalledTimes(2);
  expect(refAsFunc).toHaveBeenCalledWith(null);
  expect(refAsObj.current).toBe(null);
});
