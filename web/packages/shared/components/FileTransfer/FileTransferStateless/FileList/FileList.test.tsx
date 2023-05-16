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
