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

import { act, render, screen, userEvent } from 'design/utils/testing';

import Logger, { NullService } from 'teleterm/logger';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import {
  DocumentTshNode,
  Workspace,
} from 'teleterm/ui/services/workspacesService';
import { routing } from 'teleterm/ui/uri';
import { unique } from 'teleterm/ui/utils';
import { VnetContextProvider } from 'teleterm/ui/Vnet';

import { Connections } from './Connections';
import { ConnectionsContextProvider } from './connectionsContext';

beforeAll(() => {
  Logger.init(new NullService());
});

describe('opening VNet panel', () => {
  const tests: Array<{
    name: string;
    open: (user: ReturnType<typeof userEvent.setup>) => Promise<void>;
  }> = [
    {
      name: 'with keyboard shortcuts',
      open: user => user.keyboard('{ArrowDown}{Enter}'),
    },
    {
      name: 'with clicks',
      open: user => user.click(screen.getByTitle(/Open VNet/)),
    },
    {
      name: 'through search',
      open: user => user.keyboard('vnet{Enter}'),
    },
  ];
  test.each(tests)('$name', async ({ open }) => {
    const user = userEvent.setup();

    render(
      <MockAppContextProvider>
        <ConnectionsContextProvider>
          <VnetContextProvider>
            <Connections />
          </VnetContextProvider>
        </ConnectionsContextProvider>
      </MockAppContextProvider>
    );

    await user.click(screen.getByTitle(/Open Connections/));

    expect(
      screen.queryByTitle('Open VNet documentation')
    ).not.toBeInTheDocument();

    await open(user);

    expect(
      await screen.findByTitle('Open VNet documentation')
    ).toBeInTheDocument();
  });
});

describe('opening a connection', () => {
  const tests: Array<{
    name: string;
    open: (user: ReturnType<typeof userEvent.setup>) => Promise<void>;
  }> = [
    {
      name: 'with clicks',
      open: user => user.click(screen.getByText('alice@foo')),
    },
    {
      name: 'with keyboard',
      open: user => user.keyboard('{ArrowDown}{ArrowDown}{Enter}'),
    },
    {
      name: 'with search',
      open: async user => user.keyboard('foo{Enter}'),
    },
  ];
  test.each(tests)('$name', async ({ open }) => {
    const user = userEvent.setup();
    const appContext = new MockAppContext();
    const cluster = makeRootCluster();
    appContext.clustersService.setState(draft => {
      draft.clusters.set(cluster.uri, cluster);
    });
    const doc = {
      kind: 'doc.terminal_tsh_node',
      origin: 'search_bar',
      rootClusterId: cluster.name,
      serverUri: `${cluster.uri}/servers/foo`,
      serverId: 'foo',
      login: 'alice',
      title: 'alice@foo',
      uri: routing.getDocUri({ docId: unique() }),
    };
    appContext.workspacesService.setState(draft => {
      draft.workspaces[cluster.uri] = {
        documents: [doc],
        location: undefined,
      } as Workspace;
    });

    render(
      <MockAppContextProvider appContext={appContext}>
        <ConnectionsContextProvider>
          <VnetContextProvider>
            <Connections />
          </VnetContextProvider>
        </ConnectionsContextProvider>
      </MockAppContextProvider>
    );

    await user.click(screen.getByTitle(/Open Connections/));

    await open(user);

    // The popover got closed.
    expect(screen.queryByText('alice@foo')).not.toBeInTheDocument();
    // Doc with the connection got opened
    const workspace = appContext.workspacesService.getWorkspace(cluster.uri);
    expect(workspace.location).toEqual(doc.uri);
  });
});

test('adding a new conn while the list is open puts the new conn at the top', async () => {
  const user = userEvent.setup();
  const appContext = new MockAppContext();
  const cluster = makeRootCluster();
  appContext.clustersService.setState(draft => {
    draft.clusters.set(cluster.uri, cluster);
  });
  const doc: DocumentTshNode = {
    kind: 'doc.terminal_tsh_node',
    origin: 'search_bar',
    rootClusterId: cluster.name,
    leafClusterId: undefined,
    serverUri: `${cluster.uri}/servers/foo`,
    serverId: 'foo',
    login: 'alice',
    title: 'alice@foo',
    uri: routing.getDocUri({ docId: unique() }),
    status: 'connected',
  };
  appContext.workspacesService.setState(draft => {
    draft.workspaces[cluster.uri] = {
      documents: [doc],
      location: undefined,
    } as Workspace;
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Connections />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  await user.click(screen.getByTitle(/Open Connections/));
  expect(screen.getByText(doc.title)).toBeInTheDocument();

  const newDoc: DocumentTshNode = {
    kind: 'doc.terminal_tsh_node',
    origin: 'search_bar',
    rootClusterId: cluster.name,
    leafClusterId: undefined,
    serverUri: `${cluster.uri}/servers/bar`,
    serverId: 'bar',
    login: 'alice',
    title: 'alice@bar',
    uri: routing.getDocUri({ docId: unique() }),
    status: 'connected',
  };

  act(() => {
    appContext.workspacesService.setState(draft => {
      draft.workspaces[cluster.uri].documents.push(newDoc);
    });
  });

  const oldConn = screen.getByText(doc.title);
  const newConn = screen.getByText(newDoc.title);
  // https://developer.mozilla.org/en-US/docs/Web/API/Node/compareDocumentPosition
  expect(
    newConn.compareDocumentPosition(oldConn) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy();
});

test('disconnecting a conn does not update its position in the list', async () => {
  const user = userEvent.setup();
  const appContext = new MockAppContext();
  const cluster = makeRootCluster();
  appContext.clustersService.setState(draft => {
    draft.clusters.set(cluster.uri, cluster);
  });
  const doc: DocumentTshNode = {
    kind: 'doc.terminal_tsh_node',
    origin: 'search_bar',
    rootClusterId: cluster.name,
    leafClusterId: undefined,
    serverUri: `${cluster.uri}/servers/foo`,
    serverId: 'foo',
    login: 'alice',
    title: 'alice@foo',
    uri: routing.getDocUri({ docId: unique() }),
    status: 'connected',
  };
  const docToBeClosed: DocumentTshNode = {
    kind: 'doc.terminal_tsh_node',
    origin: 'search_bar',
    rootClusterId: cluster.name,
    leafClusterId: undefined,
    serverUri: `${cluster.uri}/servers/bar`,
    serverId: 'bar',
    login: 'alice',
    title: 'alice@bar',
    uri: routing.getDocUri({ docId: unique() }),
    status: 'connected',
  };
  appContext.workspacesService.setState(draft => {
    draft.workspaces[cluster.uri] = {
      documents: [docToBeClosed, doc],
      location: undefined,
    } as Workspace;
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Connections />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );

  await user.click(screen.getByTitle(/Open Connections/));
  const conn = () => screen.getByText(doc.title);
  const connToBeClosed = () => screen.getByText(docToBeClosed.title);

  // https://developer.mozilla.org/en-US/docs/Web/API/Node/compareDocumentPosition
  expect(
    connToBeClosed().compareDocumentPosition(conn()) &
      Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy();

  const disconnectButton = screen.getByTitle(
    `Disconnect ${docToBeClosed.title}`
  );
  await user.click(disconnectButton);

  expect(
    await screen.findByTitle(`Remove ${docToBeClosed.title}`)
  ).toBeInTheDocument();

  expect(
    connToBeClosed().compareDocumentPosition(conn()) &
      Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy();
});
