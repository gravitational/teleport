/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { createMemoryRouter, RouterProvider } from 'react-router';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import { GitServerAuthorize } from './GitServerAuthorize';

function setup(onRedirect = jest.fn()) {
  jest
    .spyOn(api, 'get')
    .mockResolvedValue({ github: { organization: 'my-org' } });

  const router = createMemoryRouter(
    [
      {
        path: cfg.routes.gitServerAuthorize,
        element: <GitServerAuthorize onRedirect={onRedirect} />,
      },
    ],
    { initialEntries: ['/web/gitservers/my-git-server/authorize'] }
  );
  render(<RouterProvider router={router} />);
}

afterEach(() => {
  jest.restoreAllMocks();
});

test('starts the flow on click', async () => {
  const redirectURL =
    'https://github.com/login/oauth/authorize?client_id=test-client-id&state=test-state';
  jest.spyOn(api, 'post').mockResolvedValue({ redirectURL });
  const onRedirect = jest.fn();
  setup(onRedirect);

  expect(await screen.findByText('my-org')).toBeInTheDocument();
  expect(api.post).not.toHaveBeenCalled();

  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: 'Continue' }));

  await waitFor(() => expect(onRedirect).toHaveBeenCalledWith(redirectURL));
  expect(api.post).toHaveBeenCalledWith(
    cfg.getGitServerUrl(
      { clusterId: cfg.proxyCluster, name: 'my-git-server' },
      'authorize'
    )
  );
});

test('shows error', async () => {
  jest.spyOn(api, 'post').mockRejectedValue(new Error('some error'));
  setup();

  await screen.findByText('my-org');
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: 'Continue' }));

  expect(await screen.findByText('some error')).toBeInTheDocument();
});
