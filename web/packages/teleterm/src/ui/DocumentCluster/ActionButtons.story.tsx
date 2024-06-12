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

import { Flex, Text, Box } from 'design';

import {
  makeApp,
  makeRootCluster,
  makeServer,
  makeDatabase,
  makeKube,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import {
  ConnectAppActionButton,
  ConnectServerActionButton,
  ConnectDatabaseActionButton,
  ConnectKubeActionButton,
} from './ActionButtons';

export default {
  title: 'Teleterm/DocumentCluster/ActionButtons',
};

export function ActionButtons() {
  return (
    <Flex gap={4}>
      <Flex gap={3} flexDirection="column">
        <Box>
          <Text>TCP app</Text>
          <TcpApp />
        </Box>
        <Box>
          <Text>Web app</Text>
          <HttpApp />
        </Box>
        <Box>
          <Text>AWS console</Text>
          <AwsConsole />
        </Box>
        <Box>
          <Text>SAML app</Text>
          <SamlApp />
        </Box>
      </Flex>
      <Box>
        <Text>Server</Text>
        <Server />
      </Box>
      <Box>
        <Text>Database</Text>
        <Database />
      </Box>
      <Box>
        <Text>Kube</Text>
        <Kube />
      </Box>
    </Flex>
  );
}

function TcpApp() {
  const appContext = new MockAppContext();
  const testCluster = makeRootCluster();
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <ConnectAppActionButton
        app={makeApp({
          uri: `${testCluster.uri}/apps/bar`,
        })}
      />
    </MockAppContextProvider>
  );
}

function HttpApp() {
  const appContext = new MockAppContext();
  const testCluster = makeRootCluster();
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <ConnectAppActionButton
        app={makeApp({
          endpointUri: 'http://localhost:3000',
          uri: `${testCluster.uri}/apps/bar`,
        })}
      />
    </MockAppContextProvider>
  );
}

function AwsConsole() {
  const appContext = new MockAppContext();
  const testCluster = makeRootCluster();
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <ConnectAppActionButton
        app={makeApp({
          endpointUri: 'https://localhost:3000',
          awsConsole: true,
          awsRoles: [
            {
              arn: 'foo',
              display: 'foo',
              name: 'foo',
              accountId: '123456789012',
            },
            {
              arn: 'bar',
              display: 'bar',
              name: 'bar',
              accountId: '123456789012',
            },
          ],
          uri: `${testCluster.uri}/apps/bar`,
        })}
      />
    </MockAppContextProvider>
  );
}

function SamlApp() {
  const appContext = new MockAppContext();
  const testCluster = makeRootCluster();
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <ConnectAppActionButton
        app={makeApp({
          endpointUri: 'https://localhost:3000',
          samlApp: true,
          uri: `${testCluster.uri}/apps/bar`,
        })}
      />
    </MockAppContextProvider>
  );
}

function Server() {
  const appContext = new MockAppContext();
  const testCluster = makeRootCluster();
  testCluster.loggedInUser.sshLogins = ['ec2-user'];
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <ConnectServerActionButton
        server={makeServer({
          uri: `${testCluster.uri}/servers/bar`,
        })}
      />
    </MockAppContextProvider>
  );
}

function Database() {
  const appContext = new MockAppContext();
  const testCluster = makeRootCluster();
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });
  appContext.resourcesService.getDbUsers = async () => ['postgres-user'];

  return (
    <MockAppContextProvider appContext={appContext}>
      <ConnectDatabaseActionButton
        database={makeDatabase({
          uri: `${testCluster.uri}/dbs/bar`,
        })}
      />
    </MockAppContextProvider>
  );
}

function Kube() {
  const appContext = new MockAppContext();
  const testCluster = makeRootCluster();
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <ConnectKubeActionButton
        kube={makeKube({
          uri: `${testCluster.uri}/kubes/bar`,
        })}
      />
    </MockAppContextProvider>
  );
}
