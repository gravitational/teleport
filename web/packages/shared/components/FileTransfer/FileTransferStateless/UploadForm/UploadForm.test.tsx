/**
 * Copyright 2022 Gravitational, Inc.
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
import { fireEvent, render, screen } from 'design/utils/testing';

import { UploadForm } from './UploadForm';

function getUploadDestinationInput(): HTMLElement {
  return screen.getByLabelText('Upload destination');
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
