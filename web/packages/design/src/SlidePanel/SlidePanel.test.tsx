/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
