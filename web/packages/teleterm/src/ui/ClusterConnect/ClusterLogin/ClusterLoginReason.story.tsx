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
import { Meta } from '@storybook/react';

import {
  makeApp,
  makeDatabaseGateway,
  makeKubeGateway,
} from 'teleterm/services/tshd/testHelpers';
import { routing } from 'teleterm/ui/uri';

import { ClusterLoginPresentation } from './ClusterLogin';
import {
  compatibilityArgType,
  makeProps,
  StoryProps,
  TestContainer,
} from './storyHelpers';

const meta: Meta<StoryProps> = {
  title: 'Teleterm/ModalsHost/ClusterLogin/Reason',
  argTypes: compatibilityArgType,
  args: {
    compatibility: 'compatible',
  },
};
export default meta;

export const GatewayCertExpiredWithDbGateway = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.gateway-cert-expired',
    targetUri: dbGateway.targetUri,
    gateway: dbGateway,
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const GatewayCertExpiredWithKubeGateway = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.gateway-cert-expired',
    targetUri: kubeGateway.targetUri,
    gateway: kubeGateway,
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const GatewayCertExpiredWithoutGateway = (storyProps: StoryProps) => {
  const props = makeProps(storyProps);
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.gateway-cert-expired',
    targetUri: dbGateway.targetUri,
    gateway: undefined,
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const VnetCertExpired = (storyProps: StoryProps) => {
  const app = makeApp();
  const props = makeProps(storyProps);
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.vnet-cert-expired',
    targetUri: app.uri,
    routeToApp: {
      name: 'tcp-app',
      publicAddr: 'tcp-app.teleport.example.com',
      clusterName: routing.parseAppUri(app.uri).params.rootClusterId,
      uri: app.endpointUri,
      targetPort: 0,
    },
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

export const VnetCertExpiredMultiPort = (storyProps: StoryProps) => {
  const app = makeApp({
    endpointUri: 'tcp://localhost',
    tcpPorts: [{ port: 1337, endPort: 0 }],
  });
  const props = makeProps(storyProps);
  props.initAttempt.data.allowPasswordless = false;
  props.reason = {
    kind: 'reason.vnet-cert-expired',
    targetUri: app.uri,
    routeToApp: {
      name: 'tcp-app',
      publicAddr: 'tcp-app.teleport.example.com',
      clusterName: routing.parseAppUri(app.uri).params.rootClusterId,
      uri: app.endpointUri,
      targetPort: 1337,
    },
  };

  return (
    <TestContainer>
      <ClusterLoginPresentation {...props} />
    </TestContainer>
  );
};

const dbGateway = makeDatabaseGateway({
  uri: '/gateways/gateway1',
  targetName: 'postgres',
  targetUri: '/clusters/teleport-local/dbs/postgres',
  targetUser: 'alice',
  targetSubresourceName: '',
  localAddress: 'localhost',
  localPort: '59116',
  protocol: 'postgres',
});

const kubeGateway = makeKubeGateway({
  uri: '/gateways/gateway2',
  targetName: 'minikube',
  targetUri: '/clusters/teleport-local/kubes/minikube',
  targetSubresourceName: '',
  localAddress: 'localhost',
  localPort: '59117',
});
