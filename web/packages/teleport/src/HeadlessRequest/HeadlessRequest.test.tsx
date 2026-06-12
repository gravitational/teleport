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

import { createMemoryHistory } from 'history';
import { Route, Router } from 'react-router';

import { render, screen } from 'design/utils/testing';

import cfg from 'teleport/config';
import { HeadlessRequest } from 'teleport/HeadlessRequest/HeadlessRequest';
import { shouldShowMfaPrompt } from 'teleport/lib/useMfa';
import auth from 'teleport/services/auth';

const mockGetChallengeResponse = jest.fn();

jest.mock('teleport/lib/useMfa', () => ({
  useMfa: () => ({
    getChallengeResponse: mockGetChallengeResponse,
    attempt: { status: '' },
  }),
  shouldShowMfaPrompt: jest.fn(),
}));

function setup({ mfaPrompt = false, path = '/web/headless/123' } = {}) {
  (shouldShowMfaPrompt as jest.Mock).mockReturnValue(mfaPrompt);

  mockGetChallengeResponse.mockResolvedValue({ webauthn_response: {} });

  jest
    .spyOn(auth, 'headlessSsoGet')
    .mockResolvedValue({ clientIpAddress: '1.2.3.4' });

  const mockHistory = createMemoryHistory({ initialEntries: [path] });

  render(
    <Router history={mockHistory}>
      <Route path={cfg.routes.headlessSso}>
        <HeadlessRequest />
      </Route>
    </Router>
  );
}

describe('HeadlessRequest', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('shows the headless request approve/reject dialog', async () => {
    setup({ mfaPrompt: false, path: '/web/headless/abc' });

    await expect(
      screen.findByText(/Someone has initiated a command from 1.2.3.4/i)
    ).resolves.toBeInTheDocument();

    expect(
      await screen.findByRole('button', { name: /Approve/i })
    ).toBeInTheDocument();
    expect(
      await screen.findByRole('button', { name: /Reject/i })
    ).toBeInTheDocument();
  });

  test('shows MFA prompt after user approves the request', async () => {
    setup({ mfaPrompt: true, path: '/web/headless/abc' });

    expect(await screen.findAllByText(/Verify Your Identity/i)).toHaveLength(2);
  });
});
