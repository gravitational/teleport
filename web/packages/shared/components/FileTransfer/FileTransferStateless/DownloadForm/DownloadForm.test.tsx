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

import { DownloadForm } from './DownloadForm';

function getFilePathInput(): HTMLElement {
  return screen.getByLabelText('File Path');
}

test('button is disabled when path does not point to file', () => {
  render(<DownloadForm onAddDownload={() => {}} />);
  fireEvent.change(getFilePathInput(), {
    target: { value: '/Users/' },
  });

  expect(screen.getByTitle('Download')).toBeDisabled();
});

test('button is enabled when path points to file', () => {
  render(<DownloadForm onAddDownload={() => {}} />);
  fireEvent.change(getFilePathInput(), {
    target: { value: '/Users/file.txt' },
  });

  expect(screen.getByTitle('Download')).toBeEnabled();
});

test('onAddDownload is invoked when the from is submitted', () => {
  const handleAddDownload = jest.fn();
  const filePath = '/Users/file.txt';

  render(<DownloadForm onAddDownload={handleAddDownload} />);

  fireEvent.change(getFilePathInput(), {
    target: { value: filePath },
  });
  expect(screen.getByTitle('Download')).toBeEnabled();

  fireEvent.submit(screen.getByRole('form'));
  expect(handleAddDownload).toHaveBeenCalledWith(filePath);
});
