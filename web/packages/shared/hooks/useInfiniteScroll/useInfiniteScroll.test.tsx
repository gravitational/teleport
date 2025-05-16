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

import { act, renderHook } from '@testing-library/react';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';

import { render, screen } from 'design/utils/testing';

import { useInfiniteScroll } from '.';

const mio = mockIntersectionObserver();

function hookProps() {
  return {
    fetch: jest.fn(),
  };
}

test('calls fetch function whenever an element is in view', async () => {
  const props = hookProps();
  const { result } = renderHook(useInfiniteScroll, {
    initialProps: props,
  });
  render(<div ref={result.current.setTrigger} data-testid="trigger" />);
  const trigger = screen.getByTestId('trigger');
  expect(props.fetch).toHaveBeenCalledTimes(0);

  act(() => mio.enterNode(trigger));
  expect(props.fetch).toHaveBeenCalledTimes(1);

  act(() => mio.leaveNode(trigger));
  expect(props.fetch).toHaveBeenCalledTimes(1);

  act(() => mio.enterNode(trigger));
  expect(props.fetch).toHaveBeenCalledTimes(2);
});

test('supports changing nodes', async () => {
  render(
    <>
      <div data-testid="trigger1" />
      <div data-testid="trigger2" />
    </>
  );
  const trigger1 = screen.getByTestId('trigger1');
  const trigger2 = screen.getByTestId('trigger2');
  const props = hookProps();
  const { result, rerender } = renderHook(useInfiniteScroll, {
    initialProps: props,
  });
  result.current.setTrigger(trigger1);

  act(() => mio.enterNode(trigger1));
  expect(props.fetch).toHaveBeenCalledTimes(1);

  rerender(props);
  result.current.setTrigger(trigger2);

  // Should register entering trigger2.
  act(() => mio.enterNode(trigger2));
  expect(props.fetch).toHaveBeenCalledTimes(2);
});

test('when there are multiple entries, only call fetch once on first encountered intersection', async () => {
  const props = hookProps();
  const { result } = renderHook(useInfiniteScroll, {
    initialProps: props,
  });
  render(<div ref={result.current.setTrigger} data-testid="trigger" />);
  const trigger = screen.getByTestId('trigger');
  expect(props.fetch).toHaveBeenCalledTimes(0);

  // Should not call a fetch because nothing has intersected.
  mio.triggerNodes([
    { node: trigger, desc: { isIntersecting: false, intersectionRatio: 0 } },
    { node: trigger, desc: { isIntersecting: false, intersectionRatio: 0 } },
  ]);
  expect(props.fetch).toHaveBeenCalledTimes(0);

  // Should call fetch only once, despite multiple entries being intersected.
  mio.triggerNodes([
    { node: trigger, desc: { isIntersecting: false, intersectionRatio: 0 } },
    { node: trigger, desc: { isIntersecting: true, intersectionRatio: 1 } },
    { node: trigger, desc: { isIntersecting: true, intersectionRatio: 1 } },
  ]);
  expect(props.fetch).toHaveBeenCalledTimes(1);
});
