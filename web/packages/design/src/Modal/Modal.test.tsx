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

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';

import { fireEvent, render } from 'design/utils/testing';

import Modal, { ModalProps } from './Modal';

const renderModal = (props: Omit<ModalProps, 'open'>) => {
  return render(
    <Modal open={true} {...props}>
      <div>Hello</div>
    </Modal>
  );
};

const escapeKey = {
  key: 'Escape',
};

test('respects open prop set to false', () => {
  render(
    <Modal open={false}>
      <div>Hello</div>
    </Modal>
  );

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});

test('respects onBackdropClick prop', () => {
  const mockFn = jest.fn();

  renderModal({
    onBackdropClick: mockFn,
  });

  // handlebackdropClick
  fireEvent.click(screen.getByTestId('backdrop'));
  expect(mockFn).toHaveBeenCalled();
});

test('respects onEscapeKeyDown props', () => {
  const mockFn = jest.fn();

  const { container } = renderModal({
    onEscapeKeyDown: mockFn,
  });

  // handleDocumentKeyDown
  fireEvent.keyDown(container, escapeKey);
  expect(mockFn).toHaveBeenCalled();
});

test('respects onClose prop', () => {
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

test('respects hideBackDrop prop', () => {
  renderModal({
    hideBackdrop: true,
  });

  expect(screen.queryByTestId('backdrop')).not.toBeInTheDocument();
});

test('respects disableBackdropClick prop', () => {
  const mockFn = jest.fn();
  renderModal({
    disableBackdropClick: true,
    onClose: mockFn,
  });

  // handleBackdropClick
  fireEvent.click(screen.getByTestId('backdrop'));
  expect(mockFn).not.toHaveBeenCalled();
});

test('respects disableEscapeKeyDown prop', () => {
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

test('restores focus on close', async () => {
  const user = userEvent.setup();
  render(<ToggleableModal />);
  const toggleModalButton = screen.getByRole('button', { name: 'Toggle' });

  await user.click(toggleModalButton);
  // Type in the input inside the modal to shift focus into another element.
  const input = screen.getByLabelText('Input in modal');
  await user.type(input, 'a');

  const closeModal = screen.getByRole('button', { name: 'Close modal' });
  await user.click(closeModal);

  expect(toggleModalButton).toHaveFocus();
});

test('closing dialog when attachCustomKeyEventHandler is set only hides it with DOM', async () => {
  const { rerender } = render(
    <Modal open={true} keepInDOMAfterClose>
      <div>Hello</div>
    </Modal>
  );

  expect(screen.getByText('Hello')).toBeVisible();

  rerender(
    <Modal open={false} keepInDOMAfterClose>
      <div>Hello</div>
    </Modal>
  );

  expect(screen.getByText('Hello')).not.toBeVisible();
});

const ToggleableModal = () => {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <>
      <button type="button" onClick={() => setIsOpen(!isOpen)}>
        Toggle
      </button>
      <Modal open={isOpen}>
        <form>
          <label>
            Input in modal
            <input />
          </label>
          <button type="button" onClick={() => setIsOpen(false)}>
            Close modal
          </button>
        </form>
      </Modal>
    </>
  );
};
