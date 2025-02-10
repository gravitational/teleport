/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import '@testing-library/jest-dom';

import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { mockResizeObserver } from 'jsdom-testing-mocks';
import { useRef, useState } from 'react';

import { useResizeObserver } from './useResizeObserver';

const resizeObserver = mockResizeObserver();
it('does not break when observed element is conditionally not rendered', async () => {
  const user = userEvent.setup();
  const onResize = jest.fn();
  // render
  render(
    // <ExampleComponent onResize={onResize} resizeObserver={resizeObserver} />
    <ExampleComponent onResize={onResize} />
  );
  let resizableEl = screen.getByTestId('resizable');
  expect(resizableEl).toBeInTheDocument();
  resizeObserver.mockElementSize(resizableEl, {
    contentBoxSize: { inlineSize: 300, blockSize: 200 },
  });
  expect(onResize).not.toHaveBeenCalled();

  console.log('First resize');
  // Verify that ResizeObserver is working as expected.
  // await user.click(screen.getByText('Resize'));
  resizeObserver.resize(resizableEl);
  expect(onResize).toHaveBeenCalledTimes(1);
  console.log('Second resize');
  // await user.click(screen.getByText('Resize'));
  resizeObserver.resize(resizableEl);
  expect(onResize).toHaveBeenCalledTimes(2);

  // Hide element and verify that resizing the old, now unmounted node does not trigger the callback.
  console.log('Hiding');
  await user.click(screen.getByText('Hide'));
  expect(screen.queryByTestId('resizable')).not.toBeInTheDocument();
  // perform action that changes size
  // await user.click(screen.getByText('Resize'));
  console.log('Third resize');
  resizeObserver.resize(resizableEl);
  // verify callback was not called
  // NOTE: This fails in the current code, but should pass if isShown is passed as enabled OR the
  // new approach is used.
  expect(onResize).toHaveBeenCalledTimes(2);

  // unhide element
  console.log('Showing');
  await user.click(screen.getByText('Show'));
  resizableEl = screen.getByTestId('resizable');
  resizeObserver.mockElementSize(resizableEl, {
    contentBoxSize: { inlineSize: 300, blockSize: 200 },
  });
  // perform action that changes size
  // await user.click(screen.getByText('Resize'));
  console.log('Fourth resize');
  resizeObserver.resize(resizableEl);
  // verify callback was called
  // NOTE: This fails on the current code, because the resize observer still observes the old node,
  // not the new one.
  expect(onResize).toHaveBeenCalledTimes(3);
});

const ExampleComponent = (props: { onResize: () => void }) => {
  const ref = useRef<HTMLDivElement>(null);

  const [isShown, setIsShown] = useState(true);
  useResizeObserver(ref, props.onResize, { enabled: isShown });

  return (
    <div>
      <button type="button" onClick={() => setIsShown(!isShown)}>
        {isShown ? 'Hide' : 'Show'}
      </button>

      {isShown && <div data-testid="resizable" ref={ref}></div>}
    </div>
  );
};
