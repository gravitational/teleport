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

import 'jest-canvas-mock';

import { render, screen } from 'design/utils/testing';
import { makeDeepLinkWithSafeInput } from 'shared/deepLinks';

import { parseDeepLink } from 'teleterm/deepLinks';
import Logger, { NullService } from 'teleterm/logger';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { App } from 'teleterm/ui/App';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

beforeAll(() => {
  Logger.init(new NullService());
});

test('queuing up a deep link launch before the app is rendered', async () => {
  const launchDeepLink = jest.fn().mockResolvedValue(undefined);
  const ctx = new MockAppContext();
  ctx.configService.set('usageReporting.enabled', false);
  const rootCluster = makeRootCluster();

  const deepLinkParseResult = parseDeepLink(
    makeDeepLinkWithSafeInput({
      path: '/vnet',
      proxyHost: rootCluster.proxyHost,
      username: rootCluster.loggedInUser.name,
      searchParams: {},
    })
  );
  expect(deepLinkParseResult.status).toEqual('success');
  // Before the app is rendered, queue up a deep link launch to be sent after the UI is ready.
  const deepLinkLaunchPromise = ctx.mockMainProcessClient
    .whenFrontendAppIsReady()
    .then(() => {
      ctx.mockMainProcessClient.launchDeepLink(deepLinkParseResult);
    });

  render(<App ctx={ctx} launchDeepLink={launchDeepLink} />);

  expect(await screen.findByText('Connect a Cluster')).toBeInTheDocument();

  // Verify that once the UI is ready, launchDeepLink is called.
  await expect(deepLinkLaunchPromise).resolves.toBe(undefined);
  expect(launchDeepLink).toHaveBeenCalledTimes(1);
  expect(launchDeepLink).toHaveBeenCalledWith(
    expect.anything(),
    expect.anything(),
    deepLinkParseResult
  );
});
