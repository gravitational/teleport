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

import { connectToApp } from 'teleterm/ui/services/workspacesService';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { makeApp, makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { IAppContext } from 'teleterm/ui/types';

describe('launching an app in the browser for', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  test('web app (if requested)', async () => {
    jest.spyOn(window, 'open').mockImplementation();
    const appContext = new MockAppContext();
    setTestCluster(appContext);
    const app = makeApp({
      endpointUri: 'http://localhost:3000',
    });

    await connectToApp(
      appContext,
      app,
      { origin: 'resource_table' },
      {
        launchInBrowserIfWebApp: true,
      }
    );
    expect(window.open).toHaveBeenCalledWith(
      'https://teleport-local:3080/web/launch/local-app.example.com:3000/teleport-local/local-app.example.com:3000',
      '_blank',
      'noreferrer,noopener'
    );
  });

  test('saml app', async () => {
    jest.spyOn(window, 'open').mockImplementation();
    const appContext = new MockAppContext();
    setTestCluster(appContext);
    const app = makeApp({ samlApp: true });

    await connectToApp(appContext, app, { origin: 'resource_table' });

    expect(window.open).toHaveBeenCalledWith(
      'https://teleport-local:3080/enterprise/saml-idp/login/foo',
      '_blank',
      'noreferrer,noopener'
    );
  });

  test('aws app', async () => {
    jest.spyOn(window, 'open').mockImplementation();
    const appContext = new MockAppContext();
    setTestCluster(appContext);
    const app = makeApp({ awsConsole: true });

    await connectToApp(
      appContext,
      app,
      { origin: 'resource_table' },
      { launchInBrowserIfWebApp: true, arnForAwsApp: 'foo-arn' }
    );
    expect(window.open).toHaveBeenCalledWith(
      'https://teleport-local:3080/web/launch/local-app.example.com:3000/teleport-local/local-app.example.com:3000/foo-arn',
      '_blank',
      'noreferrer,noopener'
    );
  });
});

test.each([
  {
    name: 'creates tunnel for a tcp app',
    app: makeApp({
      endpointUri: 'tcp://localhost:3000',
    }),
  },
  {
    name: 'creates tunnel for a web app by default',
    app: makeApp({
      endpointUri: 'http://localhost:3000',
    }),
  },
])('$name', async ({ app }) => {
  const appContext = new MockAppContext();
  setTestCluster(appContext);

  await connectToApp(appContext, app, { origin: 'resource_table' });
  const documents = appContext.workspacesService
    .getActiveWorkspaceDocumentService()
    .getGatewayDocuments();
  expect(documents).toHaveLength(1);
  expect(documents[0]).toEqual({
    gatewayUri: undefined,
    kind: 'doc.gateway',
    origin: 'resource_table',
    port: undefined,
    status: '',
    targetName: 'foo',
    targetSubresourceName: undefined,
    targetUri: '/clusters/teleport-local/apps/foo',
    targetUser: '',
    title: 'foo',
    uri: expect.any(String),
  });
});

test('cloud app triggers alert', async () => {
  jest.spyOn(window, 'alert').mockImplementation();
  const appContext = new MockAppContext();
  setTestCluster(appContext);
  const app = makeApp({
    endpointUri: 'cloud://localhost:3000',
  });

  await connectToApp(appContext, app, { origin: 'resource_table' });
  expect(window.alert).toHaveBeenCalledWith(
    'Cloud apps are supported only in tsh.'
  );
});

function setTestCluster(appContext: IAppContext): void {
  const testCluster = makeRootCluster();
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });
}
