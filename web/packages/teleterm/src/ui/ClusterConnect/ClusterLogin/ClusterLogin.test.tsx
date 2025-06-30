/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import userEvent from '@testing-library/user-event';

import { render, screen } from 'design/utils/testing';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { ClusterLogin } from './ClusterLogin';

it('keeps the focus on the password field on submission error', async () => {
  const user = userEvent.setup();
  const cluster = makeRootCluster();
  const appContext = new MockAppContext();
  appContext.addRootCluster(cluster);

  jest
    .spyOn(appContext.tshd, 'login')
    .mockResolvedValue(
      new MockedUnaryCall(undefined, new Error('whoops something went wrong'))
    );

  render(
    <MockAppContextProvider appContext={appContext}>
      <ClusterLogin
        clusterUri={cluster.uri}
        onCancel={() => {}}
        prefill={{ username: 'alice' }}
        reason={undefined}
      />
    </MockAppContextProvider>
  );

  const passwordField = await screen.findByLabelText('Password');
  expect(passwordField).toHaveFocus();

  await user.type(passwordField, 'foo');
  await user.click(screen.getByText('Sign In'));

  await screen.findByText('whoops something went wrong');
  expect(passwordField).toHaveFocus();
});
