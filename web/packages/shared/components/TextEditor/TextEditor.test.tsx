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

import * as copyModule from 'design/utils/copyToClipboard';
import { render, screen, userEvent } from 'design/utils/testing';
import * as downloadsModule from 'shared/utils/download';

import TextEditor from '.';

describe('textEditor tests', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  test('copy content button', async () => {
    render(
      <TextEditor
        copyButton
        data={[
          {
            content: 'my-content',
            type: 'yaml',
          },
        ]}
      />
    );
    const mockedCopyToClipboard = jest.spyOn(copyModule, 'copyToClipboard');

    await userEvent.click(screen.getByTitle('Copy to clipboard'));
    expect(mockedCopyToClipboard).toHaveBeenCalledWith('my-content');
  });

  test('download content button', async () => {
    render(
      <TextEditor
        downloadButton
        downloadFileName="test.yaml"
        data={[
          {
            content: 'my-content',
            type: 'yaml',
          },
        ]}
      />
    );
    const mockedDownload = jest.spyOn(downloadsModule, 'downloadObject');

    await userEvent.click(screen.getByTitle('Download'));
    expect(mockedDownload).toHaveBeenCalledWith('test.yaml', 'my-content');
  });
});
