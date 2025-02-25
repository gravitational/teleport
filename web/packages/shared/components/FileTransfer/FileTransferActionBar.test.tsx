/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { render, screen } from 'design/utils/testing';

import { FileTransferActionBar } from './FileTransferActionBar';
import { FileTransferContextProvider } from './FileTransferContextProvider';

test('file transfer bar is enabled by default', async () => {
  render(
    <FileTransferContextProvider>
      <FileTransferActionBar hasAccess={true} isConnected={true} />
    </FileTransferContextProvider>
  );

  expect(screen.getByTitle('Download files')).toBeEnabled();
  expect(screen.getByTitle('Upload files')).toBeEnabled();
});

test('file transfer is disable if no access', async () => {
  render(
    <FileTransferContextProvider>
      <FileTransferActionBar hasAccess={false} isConnected={true} />
    </FileTransferContextProvider>
  );

  expect(screen.getByTitle('Download files')).toBeDisabled();
  expect(screen.getByTitle('Upload files')).toBeDisabled();
});
