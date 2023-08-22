/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { renderHook } from '@testing-library/react-hooks';
import { render, screen } from 'design/utils/testing';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';

import { useInfiniteScroll } from './useInfiniteScroll';

const mio = mockIntersectionObserver();

test('calls the callback whenever an element is in view', () => {
  render(<div data-testid="trigger" />);
  const trigger = screen.getByTestId('trigger');
  const cb = jest.fn();
  renderHook(() => useInfiniteScroll(trigger, cb));
  expect(cb).not.toHaveBeenCalled();

  mio.enterNode(trigger);
  expect(cb).toHaveBeenCalledTimes(1);
  mio.leaveNode(trigger);
  expect(cb).toHaveBeenCalledTimes(1);
  mio.enterNode(trigger);
  expect(cb).toHaveBeenCalledTimes(2);
});

test('supports changing nodes', () => {
  render(
    <>
      <div data-testid="trigger1" />
      <div data-testid="trigger2" />
    </>
  );
  const trigger1 = screen.getByTestId('trigger1');
  const trigger2 = screen.getByTestId('trigger2');
  const cb = jest.fn();
  const { rerender } = renderHook(
    props => useInfiniteScroll(props.trigger, props.cb),
    {
      initialProps: { trigger: trigger1, cb },
    }
  );

  mio.enterNode(trigger1);
  expect(cb).toHaveBeenCalledTimes(1);

  rerender({ trigger: trigger2, cb });

  // Should only register entering trigger2.
  mio.leaveNode(trigger1);
  mio.enterNode(trigger1);
  mio.enterNode(trigger2);
  expect(cb).toHaveBeenCalledTimes(2);
});

test('supports changing callbacks', () => {
  render(
    <>
      <div data-testid="trigger1" />
      <div data-testid="trigger2" />
    </>
  );
  const trigger1 = screen.getByTestId('trigger1');
  const trigger2 = screen.getByTestId('trigger2');
  const cb = jest.fn();
  const { rerender } = renderHook(
    props => useInfiniteScroll(props.trigger, props.cb),
    {
      initialProps: { trigger: trigger1, cb },
    }
  );

  mio.enterNode(trigger1);
  expect(cb).toHaveBeenCalledTimes(1);

  rerender({ trigger: trigger2, cb });

  // Should only register entering trigger2.
  mio.leaveNode(trigger1);
  mio.enterNode(trigger1);
  mio.enterNode(trigger2);
  expect(cb).toHaveBeenCalledTimes(2);
});
