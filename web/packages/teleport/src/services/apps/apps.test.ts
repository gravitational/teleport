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

import cfg from 'teleport/config';
import api from 'teleport/services/api';
import apps from 'teleport/services/apps';

test('correct formatting of apps fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);
  const response = await apps.fetchApps('does-not-matter', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [
      {
        kind: 'app',
        id: 'cluster-id-app-name-app-name.example.com',
        name: 'app-name',
        description: 'some description',
        uri: 'http://localhost:3001',
        publicAddr: 'app-name.example.com',
        labels: [{ name: 'env', value: 'dev' }],
        clusterId: 'cluster-id',
        useAnyProxyPublicAddr: false,
        fqdn: 'app-name.example.com',
        friendlyName: '',
        launchUrl:
          '/web/launch/app-name.example.com/cluster-id/app-name.example.com',
        awsRoles: [],
        awsConsole: false,
        isCloud: false,
        isTcp: false,
        addrWithProtocol: 'https://app-name.example.com',
        userGroups: [],
        samlApp: false,
        samlAppSsoUrl: '',
        integration: '',
        permissionSets: [],
      },
      {
        kind: 'app',
        id: 'cluster-id-cloud-app-cloud://some-addr',
        name: 'cloud-app',
        description: '',
        uri: 'cloud://some-addr',
        publicAddr: '',
        labels: [],
        clusterId: 'cluster-id',
        fqdn: '',
        useAnyProxyPublicAddr: false,
        friendlyName: '',
        launchUrl: '',
        awsRoles: [],
        awsConsole: false,
        isCloud: true,
        isTcp: false,
        addrWithProtocol: 'cloud://some-addr',
        userGroups: [],
        samlApp: false,
        samlAppSsoUrl: '',
        integration: '',
        permissionSets: [],
      },
      {
        kind: 'app',
        id: 'cluster-id-tcp-app-tcp://some-addr',
        name: 'tcp-app',
        description: '',
        uri: 'tcp://some-addr',
        publicAddr: '',
        labels: [],
        clusterId: 'cluster-id',
        fqdn: '',
        useAnyProxyPublicAddr: false,
        friendlyName: '',
        launchUrl: '',
        awsRoles: [],
        awsConsole: false,
        isCloud: false,
        isTcp: true,
        addrWithProtocol: 'tcp://some-addr',
        userGroups: [],
        samlApp: false,
        samlAppSsoUrl: '',
        integration: '',
        permissionSets: [],
      },
      {
        kind: 'app',
        id: 'cluster-id-saml-app-',
        name: 'saml-app',
        description: 'SAML Application',
        uri: '',
        publicAddr: '',
        labels: [],
        clusterId: 'cluster-id',
        fqdn: '',
        useAnyProxyPublicAddr: false,
        friendlyName: '',
        launchUrl: '',
        awsRoles: [],
        awsConsole: false,
        isCloud: false,
        isTcp: false,
        addrWithProtocol: '',
        userGroups: [],
        samlApp: true,
        samlAppSsoUrl: 'http://localhost/enterprise/saml-idp/login/saml-app',
        samlAppPreset: 'gcp-workforce',
        integration: '',
        permissionSets: [],
        samlAppLaunchUrls: [{ url: 'https://example.com' }],
      },
      {
        kind: 'app',
        id: 'cluster-id-app-with-other-proxy-addr-app-with-other-proxy-addr.example.com',
        name: 'app-with-other-proxy-addr',
        useAnyProxyPublicAddr: true,
        description: 'i have a different proxy addr',
        uri: 'http://localhost:3001',
        publicAddr: 'app-with-other-proxy-addr.example.com',
        labels: [],
        clusterId: 'cluster-id',
        fqdn: 'app-with-other-proxy-addr.localhost',
        friendlyName: '',
        launchUrl: '/web/launch/app-with-other-proxy-addr.localhost',
        awsRoles: [],
        awsConsole: false,
        isCloud: false,
        isTcp: false,
        addrWithProtocol: 'https://app-with-other-proxy-addr.localhost',
        userGroups: [],
        samlApp: false,
        samlAppSsoUrl: '',
        integration: '',
        permissionSets: [],
      },
      {
        kind: 'app',
        id: 'cluster-id-mcp-app-mcp-app.example.com',
        name: 'mcp-app',
        useAnyProxyPublicAddr: false,
        description: 'Some MCP app',
        uri: 'mcp+stdio://',
        publicAddr: 'mcp-app.example.com',
        labels: [],
        clusterId: 'cluster-id',
        fqdn: '',
        friendlyName: '',
        launchUrl: '',
        awsRoles: [],
        awsConsole: false,
        isCloud: false,
        isTcp: false,
        addrWithProtocol: 'mcp+stdio://mcp-app.example.com',
        userGroups: [],
        samlApp: false,
        samlAppSsoUrl: '',
        integration: '',
        permissionSets: [],
        mcp: {
          command: 'npx',
          args: ['-y', '@modelcontextprotocol/server-everything'],
          runAsHostUser: 'hostuser',
        },
      },
    ],
    startKey: mockResponse.startKey,
    totalCount: mockResponse.totalCount,
  });
});

test('null response from apps fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const response = await apps.fetchApps('does-not-matter', {
    search: 'does-not-matter',
  });

  expect(response).toEqual({
    agents: [],
    startKey: undefined,
    totalCount: undefined,
  });
});

test('null labels field in apps fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ items: [{ labels: null }] });
  const response = await apps.fetchApps('does-not-matter', {
    search: 'does-not-matter',
  });

  expect(response.agents[0].labels).toEqual([]);
});

test('createAppSession', async () => {
  const backend = jest.spyOn(api, 'post').mockResolvedValue({
    fqdn: 'app-name.example.com',
    cookieValue: 'cookie-value',
    subjectCookieValue: 'subject-cookie-value',
  });

  const response = await apps.createAppSession({
    fqdn: 'app-name.example.com',
    cluster_name: 'example.com',
    public_addr: 'app-name.example.com',
  });

  expect(response.fqdn).toEqual('app-name.example.com');

  expect(backend).toHaveBeenCalledWith(
    cfg.api.appSession,
    expect.objectContaining({
      fqdn: 'app-name.example.com',
      cluster_name: 'example.com',
      public_addr: 'app-name.example.com',
    })
  );
});

const mockResponse = {
  items: [
    {
      awsConsole: false,
      clusterId: 'cluster-id',
      description: 'some description',
      fqdn: 'app-name.example.com',
      labels: [{ name: 'env', value: 'dev' }],
      name: 'app-name',
      publicAddr: 'app-name.example.com',
      uri: 'http://localhost:3001',
    },
    {
      clusterId: 'cluster-id',
      name: 'cloud-app',
      uri: 'cloud://some-addr',
    },
    {
      clusterId: 'cluster-id',
      name: 'tcp-app',
      uri: 'tcp://some-addr',
    },
    {
      clusterId: 'cluster-id',
      name: 'saml-app',
      description: 'SAML Application',
      samlApp: true,
      samlAppPreset: 'gcp-workforce',
      samlAppLaunchUrls: [{ url: 'https://example.com' }],
    },
    {
      clusterId: 'cluster-id',
      name: 'app-with-other-proxy-addr',
      publicAddr: 'app-with-other-proxy-addr.example.com',
      fqdn: 'app-with-other-proxy-addr.localhost',
      description: 'i have a different proxy addr',
      useAnyProxyPublicAddr: true,
      uri: 'http://localhost:3001',
    },
    {
      clusterId: 'cluster-id',
      name: 'mcp-app',
      publicAddr: 'mcp-app.example.com',
      description: 'Some MCP app',
      uri: 'mcp+stdio://',
      mcp: {
        command: 'npx',
        args: ['-y', '@modelcontextprotocol/server-everything'],
        runAsHostUser: 'hostuser',
      },
    },
  ],
  startKey: 'mockKey',
  totalCount: 100,
};
