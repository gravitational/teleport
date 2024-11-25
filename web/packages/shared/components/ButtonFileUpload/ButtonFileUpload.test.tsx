/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { ButtonFileUpload } from './ButtonFileUpload';

test('buttonFileUpload', async () => {
  const setSelectedFile = jest.fn();
  const file = new File(['dummy'], 'dummy.text', { type: 'plain/text' });
  render(
    <ButtonFileUpload
      onFileSelect={setSelectedFile}
      text="click"
      errorMessage="No files selected."
      accept=".txt"
      disabled={false}
    />
  );

  const inputElement: HTMLInputElement =
    screen.getByTestId('button-file-upload');
  fireEvent.change(inputElement, { target: { files: [] } });
  expect(screen.getByText('No files selected.')).toBeInTheDocument();

  fireEvent.change(inputElement, { target: { files: [file] } });
  expect(screen.getByText('dummy.text')).toBeInTheDocument();
  expect(inputElement.files[0]).toBe(file);
  expect(setSelectedFile).toHaveBeenCalledWith(file);
});
