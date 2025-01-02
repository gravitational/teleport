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

import { TransferredFile } from '../types';
import { FileList } from './FileList';

const files: TransferredFile[] = [
  {
    id: '1',
    name: '~/mona-lisa.jpg',
    transferState: { type: 'processing', progress: 0 },
  },
  {
    id: '2',
    name: '~/time.jpg',
    transferState: { type: 'processing', progress: 1 },
  },
];

test('list items are rendered', () => {
  render(<FileList files={files} onCancel={jest.fn()} />);
  const [listItem] = screen.getAllByRole('listitem');
  expect(listItem).toHaveTextContent('~/mona-lisa.jpg');
  expect(listItem).toHaveTextContent('0%');
});

test('transfer is cancelled when component unmounts', () => {
  const handleCancel = jest.fn();
  const { unmount } = render(
    <FileList files={[files[0]]} onCancel={handleCancel} />
  );
  unmount();
  expect(handleCancel).toHaveBeenCalledTimes(1);
});

test('transfer is cancelled when user clicks Cancel button', () => {
  const handleCancel = jest.fn();
  render(<FileList files={[files[0]]} onCancel={handleCancel} />);
  fireEvent.click(screen.getByTitle('Cancel'));
  expect(handleCancel).toHaveBeenCalledTimes(1);
});
