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
import { mockResizeObserver } from 'jsdom-testing-mocks';

import { act } from 'design/utils/testing';

import { useElementSize } from './useElementSize';

const resizeObserver = mockResizeObserver();

describe('useElementSize', () => {
  it('tracks element size correctly with default initial values', () => {
    render(<TestComponent />);

    const resizableElement = screen.getByTestId('resizable');

    expect(screen.getByTestId('width')).toHaveTextContent('0');
    expect(screen.getByTestId('height')).toHaveTextContent('0');

    resizeObserver.mockElementSize(resizableElement, {
      contentBoxSize: { inlineSize: 300, blockSize: 200 },
    });

    act(() => {
      resizeObserver.resize(resizableElement);
    });

    expect(screen.getByTestId('width')).toHaveTextContent('300');
    expect(screen.getByTestId('height')).toHaveTextContent('200');

    resizeObserver.mockElementSize(resizableElement, {
      contentBoxSize: { inlineSize: 400, blockSize: 250 },
    });

    act(() => {
      resizeObserver.resize(resizableElement);
    });

    expect(screen.getByTestId('width')).toHaveTextContent('400');
    expect(screen.getByTestId('height')).toHaveTextContent('250');
  });

  it('allows undefined as initial dimension value', () => {
    render(<TestComponentWithUndefinedHeight />);

    expect(screen.getByTestId('width')).toHaveTextContent('0');
    expect(screen.getByTestId('height')).toHaveTextContent('undefined');

    const resizableElement = screen.getByTestId('resizable');

    resizeObserver.mockElementSize(resizableElement, {
      contentBoxSize: { inlineSize: 300, blockSize: 200 },
    });

    act(() => {
      resizeObserver.resize(resizableElement);
    });

    expect(screen.getByTestId('width')).toHaveTextContent('300');
    expect(screen.getByTestId('height')).toHaveTextContent('200');
  });
});

const TestComponent = () => {
  const [ref, { width, height }] = useElementSize<HTMLDivElement>();

  return (
    <div>
      <div data-testid="resizable" ref={ref}>
        This element&#39;s size is being observed
      </div>
      <p>
        Width: <span data-testid="width">{width}</span>px, Height:{' '}
        <span data-testid="height">{height}</span>px
      </p>
    </div>
  );
};

const TestComponentWithUndefinedHeight = () => {
  const [ref, { width, height }] = useElementSize<HTMLDivElement>({
    height: undefined,
  });

  return (
    <div>
      <div data-testid="resizable" ref={ref}>
        This element has an undefined initial height
      </div>
      <p>
        Width: <span data-testid="width">{width}</span>px, Height:{' '}
        <span data-testid="height">
          {height === undefined ? 'undefined' : height}
        </span>
        px
      </p>
    </div>
  );
};
