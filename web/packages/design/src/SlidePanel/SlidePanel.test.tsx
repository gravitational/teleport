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

import React from 'react';
import userEvent from '@testing-library/user-event';

import { render, screen } from 'design/utils/testing';
import '@testing-library/jest-dom';

import { SlidePanel } from './SlidePanel';

describe('component: SlidePanel', () => {
  it('renders children properly', () => {
    render(
      <SlidePanel position="open">
        <div>I was rendered</div>
      </SlidePanel>
    );
    expect(screen.getByText('I was rendered')).toBeInTheDocument();
  });

  it('calls the close callback when hitting escape', async () => {
    const user = userEvent.setup();
    const cb = jest.fn();
    render(
      <SlidePanel position="open" closePanel={cb}>
        <div>I was rendered</div>
      </SlidePanel>
    );
    expect(screen.getByText('I was rendered')).toBeInTheDocument();
    await user.keyboard('[Escape]');
    expect(cb.mock.calls).toHaveLength(1);
  });

  it('calls the close callback when clicking the mask', async () => {
    const user = userEvent.setup();
    const cb = jest.fn();
    render(
      <SlidePanel position="open" closePanel={cb}>
        <div>I was rendered</div>
      </SlidePanel>
    );
    expect(screen.getByText('I was rendered')).toBeInTheDocument();
    await user.pointer({
      keys: '[MouseLeft]',
      target: screen.getByTestId('mask'),
    });
    expect(cb.mock.calls).toHaveLength(1);
  });

  it('respects the position prop', () => {
    const { rerender } = render(
      <SlidePanel position="open">
        <div>I was rendered</div>
      </SlidePanel>
    );
    expect(screen.getByTestId('panel')).toHaveClass('open');
    expect(screen.getByTestId('panel')).not.toHaveClass('closed');
    rerender(
      <SlidePanel position="closed">
        <div>I was rendered</div>
      </SlidePanel>
    );
    expect(screen.getByTestId('panel')).toHaveClass('closed');
    expect(screen.getByTestId('panel')).not.toHaveClass('open');
  });
});
