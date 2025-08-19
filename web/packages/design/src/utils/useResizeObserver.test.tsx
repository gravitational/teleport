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
import { useState } from 'react';

import { useResizeObserver } from './useResizeObserver';

const resizeObserver = mockResizeObserver();

describe('useResizeObserver', () => {
  it('functions when observed element is conditionally rendered', async () => {
    const user = userEvent.setup();
    const onResize = jest.fn();

    render(<ExampleComponent onResize={onResize} />);

    let resizableElement = screen.getByTestId('resizable');

    expect(resizableElement).toBeInTheDocument();
    resizeObserver.mockElementSize(resizableElement, {
      contentBoxSize: { inlineSize: 300, blockSize: 200 },
    });
    expect(onResize).not.toHaveBeenCalled();

    // Verify that ResizeObserver is working as expected.
    resizeObserver.resize(resizableElement);
    expect(onResize).toHaveBeenCalledTimes(1);
    resizeObserver.resize(resizableElement);
    expect(onResize).toHaveBeenCalledTimes(2);

    // Hide element and verify that resizing the old, now unmounted node does not trigger the callback.
    await user.click(screen.getByText('Hide'));
    expect(screen.queryByTestId('resizable')).not.toBeInTheDocument();

    resizeObserver.resize(resizableElement);
    expect(onResize).toHaveBeenCalledTimes(2);

    // Show element again, resize the new node and verify that it triggers the callback.
    await user.click(screen.getByText('Show'));
    resizableElement = screen.getByTestId('resizable');

    resizeObserver.mockElementSize(resizableElement, {
      contentBoxSize: { inlineSize: 300, blockSize: 200 },
    });

    resizeObserver.resize(resizableElement);

    expect(onResize).toHaveBeenCalledTimes(3);
  });
});

const ExampleComponent = (props: { onResize: () => void }) => {
  const [isShown, setIsShown] = useState(true);
  const ref = useResizeObserver(props.onResize);

  return (
    <div>
      <button type="button" onClick={() => setIsShown(!isShown)}>
        {isShown ? 'Hide' : 'Show'}
      </button>
      {isShown && <div data-testid="resizable" ref={ref}></div>}
    </div>
  );
};
