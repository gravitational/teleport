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

import { within } from '@testing-library/react';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';

import { act, render, screen, userEvent } from 'design/utils/testing';
import * as proto from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';

import Logger, { NullService } from 'teleterm/logger';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { proxyHostname } from 'teleterm/services/tshd/cluster';
import { makeApp, makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { App } from 'teleterm/ui/App';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

beforeAll(() => {
  Logger.init(new NullService());
});

const mio = mockIntersectionObserver();

const tests: Array<{ name: string; app: proto.App; targetPort?: string }> = [
  { name: 'single-port', app: makeApp() },
  {
    name: 'multi-port',
    app: makeApp({
      endpointUri: 'tcp://localhost',
      tcpPorts: [
        { port: 1337, endPort: 0 },
        { port: 4242, endPort: 0 },
      ],
    }),
  },
  {
    name: 'multi-port (specific port)',
    targetPort: '4242',
    app: makeApp({
      endpointUri: 'tcp://localhost',
      tcpPorts: [
        { port: 1337, endPort: 0 },
        { port: 4242, endPort: 0 },
      ],
    }),
  },
];

test.each(tests)(
  'launching VNet for the first time through the Connect button next to a $name TCP app copies the address of the app to the clipboard',
  async ({ app, targetPort }) => {
    const expectedPublicAddr = targetPort
      ? `${app.publicAddr}:${targetPort}`
      : app.publicAddr;
    const user = userEvent.setup();
    const ctx = new MockAppContext();
    const rootCluster = makeRootCluster();
    ctx.configService.set('usageReporting.enabled', false);

    jest.spyOn(ctx.tshd, 'listUnifiedResources').mockReturnValue(
      new MockedUnaryCall({
        nextKey: '',
        resources: [
          {
            resource: { oneofKind: 'app', app },
            requiresRequest: false,
          },
        ],
      })
    );
    jest.spyOn(ctx.tshd, 'listRootClusters').mockReturnValue(
      new MockedUnaryCall({
        clusters: [rootCluster],
      })
    );
    jest.spyOn(ctx.vnet, 'getServiceInfo').mockReturnValue(
      new MockedUnaryCall({
        appDnsZones: [proxyHostname(rootCluster.proxyHost)],
        clusters: [rootCluster.name],
        sshConfigured: true,
      })
    );

    render(<App ctx={ctx} />);

    await user.click(await screen.findByText(rootCluster.name));
    act(mio.enterAll);

    expect(
      await screen.findByText(new RegExp(app.publicAddr))
    ).toBeInTheDocument();

    if (targetPort) {
      // Click the three dots menu and then select targetPort from it.
      const visibleDoc = screen.getByTestId('visible-doc');
      await user.click(within(visibleDoc).getByTitle('Open menu'));
      await user.click(await screen.findByText(targetPort));
    } else {
      // Click "Connect" in the TCP app card.
      await user.click(screen.getByText('Connect'));
    }

    // Verify that the info tab was opened, start VNet from it and wait for it to be running.
    const docVnetInfo = screen.getByTestId('visible-doc');
    expect(
      within(docVnetInfo).getByText(/VNet automatically proxies connections/)
    ).toBeInTheDocument();
    await user.click(within(docVnetInfo).getByText('Start VNet'));
    expect(
      await screen.findByText(/Proxying TCP and SSH connections/)
    ).toBeInTheDocument();

    // Verify that a notification is shown and that the address is in the clipboard.
    expect(
      await screen.findByText(
        app.tcpPorts.length
          ? /copied to clipboard/
          : /\(copied to clipboard\) and any port/
      )
    ).toBeInTheDocument();
    await user.click(screen.getByTitle('Close Notification'));
    expect(await window.navigator.clipboard.readText()).toEqual(
      expectedPublicAddr
    );

    // Clear clipboard, stop VNet and start it again.
    await window.navigator.clipboard.write([]);
    await user.click(within(docVnetInfo).getByText('Stop VNet'));
    await user.click(await within(docVnetInfo).findByText('Start VNet'));
    expect(
      await screen.findByText(/Proxying TCP and SSH connections/)
    ).toBeInTheDocument();

    // Verify that the address was not copied to the clipboard after the second start from the "Start
    // VNet" button in the info tab.
    expect(screen.queryByText(/copied to clipboard/)).not.toBeInTheDocument();
    expect(await window.navigator.clipboard.read()).toEqual([]);
  }
);

test.each(tests)(
  'launching VNet for the second time through the Connect button next to a $name TCP app starts VNet immediately',
  async ({ app, targetPort }) => {
    const expectedPublicAddr = targetPort
      ? `${app.publicAddr}:${targetPort}`
      : app.publicAddr;
    const user = userEvent.setup();
    const ctx = new MockAppContext();
    const rootCluster = makeRootCluster();
    ctx.configService.set('usageReporting.enabled', false);
    ctx.statePersistenceService.putState({
      ...ctx.statePersistenceService.getState(),
      vnet: { autoStart: false, hasEverStarted: true },
    });

    jest.spyOn(ctx.tshd, 'listUnifiedResources').mockReturnValue(
      new MockedUnaryCall({
        nextKey: '',
        resources: [
          {
            resource: { oneofKind: 'app', app },
            requiresRequest: false,
          },
        ],
      })
    );
    jest.spyOn(ctx.tshd, 'listRootClusters').mockReturnValue(
      new MockedUnaryCall({
        clusters: [rootCluster],
      })
    );
    jest.spyOn(ctx.vnet, 'getServiceInfo').mockReturnValue(
      new MockedUnaryCall({
        appDnsZones: [proxyHostname(rootCluster.proxyHost)],
        clusters: [rootCluster.name],
        sshConfigured: true,
      })
    );

    render(<App ctx={ctx} />);

    await user.click(await screen.findByText(rootCluster.name));
    act(mio.enterAll);

    expect(
      await screen.findByText(new RegExp(app.publicAddr))
    ).toBeInTheDocument();

    if (targetPort) {
      // Click the three dots menu and then select targetPort from it.
      const visibleDoc = screen.getByTestId('visible-doc');
      await user.click(within(visibleDoc).getByTitle('Open menu'));
      await user.click(await screen.findByText(targetPort));
    } else {
      // Click "Connect" in the TCP app card.
      await user.click(screen.getByText('Connect'));
    }

    // Verify that VNet is running and that the public address was copied to the clipboard.
    expect(
      await screen.findByText(/Proxying TCP and SSH connections/)
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        app.tcpPorts.length
          ? /copied to clipboard/
          : /\(copied to clipboard\) and any port/
      )
    ).toBeInTheDocument();
    expect(await window.navigator.clipboard.readText()).toEqual(
      expectedPublicAddr
    );

    // Verify that the info tab wasn't opened.
    const visibleDoc = screen.getByTestId('visible-doc');
    expect(
      within(visibleDoc).queryByText(/VNet automatically proxies connections/)
    ).not.toBeInTheDocument();
  }
);

test('launching VNet for the first time from the connections panel does not open info tab', async () => {
  const user = userEvent.setup();
  const ctx = new MockAppContext();
  const rootCluster = makeRootCluster();
  ctx.configService.set('usageReporting.enabled', false);

  jest.spyOn(ctx.tshd, 'listRootClusters').mockReturnValue(
    new MockedUnaryCall({
      clusters: [rootCluster],
    })
  );
  jest.spyOn(ctx.vnet, 'getServiceInfo').mockReturnValue(
    new MockedUnaryCall({
      appDnsZones: [proxyHostname(rootCluster.proxyHost)],
      clusters: [rootCluster.name],
      sshConfigured: true,
    })
  );

  render(<App ctx={ctx} />);

  await user.click(await screen.findByText(rootCluster.name));
  act(mio.enterAll);

  // Start VNet.
  await user.click(await screen.findByTitle(/Open Connections/));
  await user.click(await screen.findByTitle('Start VNet'));
  expect(await screen.findByTitle('Stop VNet')).toBeInTheDocument();

  // Verify that the info tab wasn't opened.
  const visibleDoc = screen.getByTestId('visible-doc');
  expect(
    within(visibleDoc).queryByText(/VNet automatically proxies connections/)
  ).not.toBeInTheDocument();
});
