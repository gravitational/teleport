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

import { fireEvent, render, screen } from 'design/utils/testing';

import { UploadForm } from './UploadForm';

function getUploadDestinationInput(): HTMLElement {
  return screen.getByLabelText('Upload Destination');
}

function getFileInput(): HTMLElement {
  return screen.getByTestId('file-input');
}

const files = [
  new File(['(⌐□_□)'], 'chuck-norris.png', { type: 'image/png' }),
  new File(['(⌐□_□)'], 'tommy-lee.png', { type: 'image/png' }),
];

test('file input is disabled when path is empty', () => {
  render(<UploadForm onAddUpload={() => {}} />);
  fireEvent.change(getUploadDestinationInput(), {
    target: { value: '' },
  });
  expect(getFileInput()).toBeDisabled();
});

test('files can be selected using input', () => {
  const handleAddUpload = jest.fn();

  render(<UploadForm onAddUpload={handleAddUpload} />);

  fireEvent.change(getFileInput(), {
    target: { files },
  });

  expect(handleAddUpload).toHaveBeenCalledTimes(2);
  expect(handleAddUpload).toHaveBeenCalledWith('~/', files[0]);
  expect(handleAddUpload).toHaveBeenCalledWith('~/', files[1]);
});

test('files can be dropped into upload area', () => {
  const handleAddUpload = jest.fn();

  render(<UploadForm onAddUpload={handleAddUpload} />);

  fireEvent.drop(screen.getByText('Drag your files here'), {
    dataTransfer: { files },
  });

  expect(handleAddUpload).toHaveBeenCalledTimes(2);
  expect(handleAddUpload).toHaveBeenCalledWith('~/', files[0]);
  expect(handleAddUpload).toHaveBeenCalledWith('~/', files[1]);
});
