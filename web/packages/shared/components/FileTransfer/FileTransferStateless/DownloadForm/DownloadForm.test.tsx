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
