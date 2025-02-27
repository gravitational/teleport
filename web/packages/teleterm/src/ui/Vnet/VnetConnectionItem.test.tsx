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
import { render, screen, userEvent } from 'design/utils/testing';
import { wait } from 'shared/utils/wait';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';

import { VnetSliderStepHeader } from './VnetConnectionItem';
import { VnetContextProvider } from './vnetContext';

describe('VnetSliderStepHeader', () => {
  it('allows to tab through the header itself as well as the buttons', async () => {
    const user = userEvent.setup();
    render(
      <MockAppContextProvider>
        <ConnectionsContextProvider>
          <VnetContextProvider>
            <VnetSliderStepHeader
              goBack={() => {}}
              runDiagnosticsFromVnetPanel={() => Promise.resolve()}
            />
          </VnetContextProvider>
        </ConnectionsContextProvider>
      </MockAppContextProvider>
    );

    const listItem = screen.getByTitle('Go back to Connections');
    const openDocumentationButton = screen.getByTitle(
      'Open VNet documentation'
    );
    const toggleButton = screen.getByTitle('Start VNet');
    expect(document.body).toHaveFocus();

    await user.tab();
    expect(listItem).toHaveFocus();

    await user.tab();
    expect(openDocumentationButton).toHaveFocus();

    await user.tab();
    expect(toggleButton).toHaveFocus();
  });

  it('maintains focus on start/stop toggle when transitioning between states', async () => {
    const appContext = new MockAppContext();
    jest.spyOn(appContext.vnet, 'start').mockImplementation(async () => {
      // An artificial delay so that the test is able to find an element with "Starting VNet" title.
      await wait(50);
      return new MockedUnaryCall({});
    });
    jest.spyOn(appContext.vnet, 'stop').mockImplementation(async () => {
      await wait(50);
      return new MockedUnaryCall({});
    });
    const user = userEvent.setup();
    render(
      <MockAppContextProvider appContext={appContext}>
        <ConnectionsContextProvider>
          <VnetContextProvider>
            <VnetSliderStepHeader
              goBack={() => {}}
              runDiagnosticsFromVnetPanel={() => Promise.resolve()}
            />
          </VnetContextProvider>
        </ConnectionsContextProvider>
      </MockAppContextProvider>
    );

    expect(document.body).toHaveFocus();

    await user.tab();
    await user.tab();
    await user.tab();
    expect(await screen.findByTitle('Start VNet')).toHaveFocus();

    await user.keyboard('{Enter}');

    expect(await screen.findByTitle('Starting VNet')).toHaveFocus();
    expect(await screen.findByTitle('Stop VNet')).toHaveFocus();

    await user.keyboard('{Enter}');

    expect(await screen.findByTitle('Stopping VNet')).toHaveFocus();
    expect(await screen.findByTitle('Start VNet')).toHaveFocus();
  });
});
