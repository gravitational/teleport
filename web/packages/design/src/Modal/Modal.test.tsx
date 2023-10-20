/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { screen } from '@testing-library/react';

import { render, fireEvent } from 'design/utils/testing';

import Modal from './Modal';

const renderModal = props => {
  return render(
    <Modal open={true} {...props}>
      <div>Hello</div>
    </Modal>
  );
};

const escapeKey = {
  key: 'Escape',
};

describe('design/Modal', () => {
  it('respects open prop set to false', () => {
    render(
      <Modal open={false}>
        <div>Hello</div>
      </Modal>
    );

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('respects onBackdropClick prop', () => {
    const mockFn = jest.fn();

    renderModal({
      onBackdropClick: mockFn,
    });

    // handlebackdropClick
    fireEvent.click(screen.getByTestId('backdrop'));
    expect(mockFn).toHaveBeenCalled();
  });

  it('respects onEscapeKeyDown props', () => {
    const mockFn = jest.fn();

    const { container } = renderModal({
      onEscapeKeyDown: mockFn,
    });

    // handleDocumentKeyDown
    fireEvent.keyDown(container, escapeKey);
    expect(mockFn).toHaveBeenCalled();
  });

  it('respects onClose prop', () => {
    const mockFn = jest.fn();

    const { container } = renderModal({
      onClose: mockFn,
    });

    // handlebackdropClick
    fireEvent.click(screen.getByTestId('backdrop'));
    expect(mockFn).toHaveBeenCalled();

    // handleDocumentKeyDown
    fireEvent.keyDown(container, escapeKey);
    expect(mockFn).toHaveBeenCalled();
  });

  it('respects hideBackDrop prop', () => {
    renderModal({
      hideBackdrop: true,
    });

    expect(screen.queryByTestId('backdrop')).not.toBeInTheDocument();
  });

  it('respects disableBackdropClick prop', () => {
    const mockFn = jest.fn();
    renderModal({
      disableBackdropClick: true,
      onClose: mockFn,
    });

    // handleBackdropClick
    fireEvent.click(screen.getByTestId('backdrop'));
    expect(mockFn).not.toHaveBeenCalled();
  });

  it('respects disableEscapeKeyDown prop', () => {
    const mockFn = jest.fn();
    const { container } = renderModal({
      disableEscapeKeyDown: true,
      onClose: mockFn,
    });

    // handleDocumentKeyDown
    fireEvent.keyDown(container, escapeKey);
    expect(mockFn).not.toHaveBeenCalled();
  });

  test('unmount cleans up event listeners and closes modal', () => {
    const mockFn = jest.fn();
    const { container, unmount } = renderModal({
      onEscapeKeyDown: mockFn,
    });

    unmount();

    expect(screen.queryByTestId('Modal')).not.toBeInTheDocument();

    fireEvent.keyDown(container, escapeKey);
    expect(mockFn).not.toHaveBeenCalled();
  });

  test('respects backdropProps prop invisible', () => {
    renderModal({
      BackdropProps: { invisible: true },
    });

    expect(screen.getByTestId('backdrop')).toHaveStyle({
      'background-color': 'transparent',
    });
  });
});
