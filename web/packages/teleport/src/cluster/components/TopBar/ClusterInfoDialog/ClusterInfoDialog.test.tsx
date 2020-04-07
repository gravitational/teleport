/**
 * Copyright 2020 Gravitational, Inc.
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
import ClusterInfoDialog from './ClusterInfoDialog';
import { ClusterInfoDialog as SbClusterInfoDialog } from './ClusterInfoDialog.story';
import { render, fireEvent, wait } from 'design/utils/testing';

test('rendering of static texts', () => {
  const { getByTestId } = render(<SbClusterInfoDialog />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('button clicks', async () => {
  Object.defineProperty(document, 'execCommand', {
    value: jest.fn(() => true),
  });

  const onClose = jest.fn();
  const { getByText, queryByText } = render(
    <ClusterInfoDialog
      clusterId=""
      publicURL=""
      proxyVersion=""
      authVersion=""
      onClose={onClose}
    />
  );

  await wait(() => fireEvent.click(getByText(/copy/i)));
  expect(queryByText(/copied/i)).not.toBeNull();

  fireEvent.click(getByText(/close/i));
  expect(onClose).toHaveBeenCalledTimes(1);
});
