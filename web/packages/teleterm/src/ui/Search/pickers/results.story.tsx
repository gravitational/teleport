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

import { useState } from 'react';

import { Flex } from 'design';
import { App } from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';
import { WindowsDesktop } from 'gen-proto-ts/teleport/lib/teleterm/v1/windows_desktop_pb';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import { getAppAddrWithProtocol } from 'teleterm/services/tshd/app';
import {
  makeApp,
  makeDatabase,
  makeKube,
  makeLabelsList,
  makeRootCluster,
  makeServer,
  makeWindowsDesktop,
} from 'teleterm/services/tshd/testHelpers';
import { getWindowsDesktopAddrWithoutDefaultPort } from 'teleterm/services/tshd/windowsDesktop';
import { ResourceSearchError } from 'teleterm/ui/services/resources';
import { routing } from 'teleterm/ui/uri';
import type * as uri from 'teleterm/ui/uri';

import { SearchResult, SearchResultApp } from '../searchResult';
import { makeResourceResult } from '../testHelpers';
import {
  AdvancedSearchEnabledItem,
  AppItem,
  ComponentMap,
  NoResultsItem,
  ResourceSearchErrorsItem,
  TypeToSearchItem,
} from './ActionPicker';
import { NoSuggestionsAvailable, SuggestionsError } from './ParameterPicker';
import { NonInteractiveItem, ResultList } from './ResultList';

export default {
  title: 'Teleterm/Search',
};

const clusterUri: uri.ClusterUri = '/clusters/teleport-local';
const longClusterUri: uri.ClusterUri =
  '/clusters/teleport-very-long-cluster-name-with-uuid-2f96e498-88ec-442f-a25b-569fa915041c';

export const Results = (props: { maxWidth: string }) => {
  const { maxWidth = '600px' } = props;

  return (
    <Flex gap={4} alignItems="flex-start">
      <div
        css={`
          max-width: ${maxWidth};
          min-width: 0;
          flex: 1;
          background-color: ${props => props.theme.colors.levels.elevated};

          display: flex;
          flex-direction: column;

          > * {
            max-height: unset;
          }
        `}
      >
        <SearchResultItems />
      </div>
      <div
        css={`
          max-width: ${maxWidth};
          min-width: 0;
          flex: 1;
          background-color: ${props => props.theme.colors.levels.elevated};

          display: flex;
          flex-direction: column;

          > * {
            max-height: unset;
          }
        `}
      >
        <AuxiliaryItems />
      </div>
    </Flex>
  );
};

export const ResultsNarrow = () => {
  return <Results maxWidth="300px" />;
};

function makeAppWithAddr(props: Partial<App>) {
  const app = makeApp(props);
  return { ...app, addrWithProtocol: getAppAddrWithProtocol(app) };
}

function makeWindowsDesktopWithoutDefaultPort(props: Partial<WindowsDesktop>) {
  const desktop = makeWindowsDesktop(props);
  return {
    ...desktop,
    addrWithoutDefaultPort: getWindowsDesktopAddrWithoutDefaultPort(desktop),
  };
}

const SearchResultItems = () => {
  const searchResults: SearchResult[] = [
    makeResourceResult({
      kind: 'server',
      resourceMatches: [{ field: 'addr', searchTerm: '10.0.0.175' }],
      labelMatches: [
        {
          kind: 'label-name',
          labelName: 'service',
          searchTerm: 'service',
          score: 40,
        },
        {
          kind: 'label-value',
          labelName: 'service',
          searchTerm: 'ansible',
          score: 60,
        },
        {
          kind: 'label-name',
          labelName: 'kernel',
          searchTerm: 'kernel',
          score: 45,
        },
        {
          kind: 'label-value',
          labelName: 'arch',
          searchTerm: 'aarch64',
          score: 55,
        },
      ],
      resource: makeServer({
        hostname: 'long-label-list',
        addr: '10.0.0.175:3022',
        uri: `${clusterUri}/servers/2f96e498-88ec-442f-a25b-569fa915041c`,
        name: '2f96e498-88ec-442f-a25b-569fa915041c',
        labels: makeLabelsList({
          arch: 'aarch64',
          external: '32.192.113.93',
          internal: '10.0.0.175',
          kernel: '5.13.0-1234-aws',
          service: 'ansible',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'server',
      labelMatches: [
        {
          kind: 'label-value',
          labelName: 'service',
          searchTerm: 'ansible',
          score: 100,
        },
      ],
      resource: makeServer({
        hostname: 'short-label-list',
        addr: '',
        tunnel: true,
        uri: `${clusterUri}/servers/90a29595-aac7-42eb-a484-c6c0e23f1a21`,
        name: '90a29595-aac7-42eb-a484-c6c0e23f1a21',
        labels: makeLabelsList({
          arch: 'aarch64',
          service: 'ansible',
          external: '32.192.113.93',
          internal: '10.0.0.175',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'server',
      resourceMatches: [{ field: 'name', searchTerm: 'bbaaceba-6bd1-4750' }],
      resource: makeServer({
        hostname: 'uuid-match',
        addr: '',
        tunnel: true,
        uri: `${clusterUri}/servers/bbaaceba-6bd1-4750-9d3d-1a80e0cc8a63`,
        name: 'bbaaceba-6bd1-4750-9d3d-1a80e0cc8a63',
        labels: makeLabelsList({
          internal: '10.0.0.175',
          service: 'ansible',
          external: '32.192.113.93',
          arch: 'aarch64',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'server',
      resourceMatches: [{ field: 'hostname', searchTerm: 'super' }],
      resource: makeServer({
        hostname:
          'super-long-server-name-with-uuid-2f96e498-88ec-442f-a25b-569fa915041c',
        uri: `${longClusterUri}/servers/super-long-desc`,
        labels: makeLabelsList({
          internal: '10.0.0.175',
          service: 'ansible',
          external: '32.192.113.93',
          arch: 'aarch64',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'server',
      requiresRequest: true,
      labelMatches: [
        {
          kind: 'label-name',
          labelName: 'service',
          searchTerm: 'service',
          score: 40,
        },
        {
          kind: 'label-value',
          labelName: 'service',
          searchTerm: 'ansible',
          score: 60,
        },
        {
          kind: 'label-name',
          labelName: 'kernel',
          searchTerm: 'kernel',
          score: 45,
        },
        {
          kind: 'label-value',
          labelName: 'arch',
          searchTerm: 'aarch64',
          score: 55,
        },
      ],
      resource: makeServer({
        hostname: 'long-label-list',
        uri: `${clusterUri}/servers/2f96e498-88ec-442f-a25b-569fa915041c`,
        name: '2f96e498-88ec-442f-a25b-569fa915041c',
        labels: makeLabelsList({
          arch: 'aarch64',
          external: '32.192.113.93',
          internal: '10.0.0.175',
          kernel: '5.13.0-1234-aws',
          service: 'ansible',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'app',
      labelMatches: [
        {
          kind: 'label-value',
          labelName: 'env',
          searchTerm: 'dev',
          score: 100,
        },
      ],
      resource: makeAppWithAddr({
        uri: `${clusterUri}/apps/web-app`,
        name: 'web-app',
        endpointUri: 'http://localhost:3000',
        desc: '',
        labels: makeLabelsList({
          access: 'cloudwatch-metrics,ec2,s3,cloudtrail',
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'app',
      resourceMatches: [{ field: 'desc', searchTerm: 'SAML' }],
      labelMatches: [
        {
          kind: 'label-name',
          labelName: 'aws/Owner',
          searchTerm: 'Owner',
          score: 35,
        },
      ],
      resource: makeAppWithAddr({
        uri: `${clusterUri}/apps/saml-app`,
        name: 'saml-app',
        endpointUri: '',
        samlApp: true,
        desc: 'SAML Application',
        labels: makeLabelsList({
          access: 'cloudwatch-metrics,ec2,s3,cloudtrail',
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'app',
      resourceMatches: [{ field: 'addrWithProtocol', searchTerm: 'local' }],
      resource: makeAppWithAddr({
        uri: `${clusterUri}/apps/no-desc`,
        name: 'no-desc',
        desc: '',
        labels: makeLabelsList({
          access: 'cloudwatch-metrics,ec2,s3,cloudtrail',
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'app',
      resourceMatches: [{ field: 'desc', searchTerm: 'Lorem' }],
      resource: makeAppWithAddr({
        uri: `${clusterUri}/apps/short-desc`,
        name: 'short-desc',
        desc: 'Lorem ipsum',
        labels: makeLabelsList({
          access: 'cloudwatch-metrics,ec2,s3,cloudtrail',
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'app',
      resourceMatches: [{ field: 'desc', searchTerm: 'dignissim' }],
      labelMatches: [
        {
          kind: 'label-value',
          labelName: 'aws/Environment',
          searchTerm: 'demo',
          score: 50,
        },
      ],
      resource: makeAppWithAddr({
        uri: `${clusterUri}/apps/long-desc`,
        name: 'long-desc',
        desc: 'Eget dignissim lectus nisi vitae nunc',
        labels: makeLabelsList({
          access: 'cloudwatch-metrics,ec2,s3,cloudtrail',
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'app',
      resourceMatches: [{ field: 'desc', searchTerm: 'Duis' }],
      resource: makeAppWithAddr({
        uri: `${clusterUri}/apps/super-long-desc`,
        name: 'super-long-desc',
        desc: 'Duis id tortor at purus tincidunt finibus. Mauris eu semper orci, non commodo lacus. Praesent sollicitudin magna id laoreet porta. Nunc lobortis varius sem vel fringilla.',
        labels: makeLabelsList({
          access: 'cloudwatch-metrics,ec2,s3,cloudtrail',
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'app',
      resourceMatches: [{ field: 'desc', searchTerm: 's' }],
      labelMatches: [
        {
          kind: 'label-name',
          labelName: 'access',
          searchTerm: 's',
          score: 40,
        },
      ],
      resource: makeAppWithAddr({
        name: 'super-long-app-with-uuid-1f96e498-88ec-442f-a25b-569fa915041c',
        desc: 'short-desc',
        uri: `${longClusterUri}/apps/super-long-desc`,
        labels: makeLabelsList({
          access: 'cloudwatch-metrics,ec2,s3,cloudtrail',
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),

    makeResourceResult({
      kind: 'app',
      requiresRequest: true,
      labelMatches: [
        {
          kind: 'label-name',
          labelName: 'access',
          searchTerm: 's',
          score: 40,
        },
      ],
      resource: makeAppWithAddr({
        uri: `${clusterUri}/apps/web-app`,
        name: 'web-app',
        endpointUri: 'http://localhost:3000',
        desc: '',
        labels: makeLabelsList({
          access: 'cloudwatch-metrics,ec2,s3,cloudtrail',
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resourceMatches: [
        { field: 'type', searchTerm: 'self' },
        { field: 'protocol', searchTerm: 'postgres' },
      ],
      labelMatches: [
        {
          kind: 'label-value',
          labelName: 'env',
          searchTerm: 'dev',
          score: 100,
        },
      ],
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/no-desc`,
        name: 'no-desc',
        desc: '',
        labels: makeLabelsList({
          'aws/Accounting': 'dev-ops',
          'aws/Environment': 'demo-13-biz',
          'aws/Name': 'db-bastion-4-13biz',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          engine: '🐘',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resourceMatches: [{ field: 'desc', searchTerm: 'm' }],
      labelMatches: [
        {
          labelName: 'aws/Environment',
          score: 40,
          kind: 'label-name',
          searchTerm: 'm',
        },
      ],
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/short-desc`,
        name: 'short-desc',
        desc: 'Lorem ipsum',
        labels: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Name': 'db-bastion-4-13biz',
          'aws/Accounting': 'dev-ops',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          engine: '🐘',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resourceMatches: [{ field: 'desc', searchTerm: 'dignissim' }],
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/long-desc`,
        name: 'long-desc',
        desc: 'Eget dignissim lectus nisi vitae nunc',
        labels: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Name': 'db-bastion-4-13biz',
          'aws/Accounting': 'dev-ops',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          engine: '🐘',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resourceMatches: [{ field: 'desc', searchTerm: 'Duis' }],
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/super-long-desc`,
        name: 'super-long-desc',
        desc: 'Duis id tortor at purus tincidunt finibus. Mauris eu semper orci, non commodo lacus. Praesent sollicitudin magna id laoreet porta. Nunc lobortis varius sem vel fringilla.',
        labels: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Accounting': 'dev-ops',
          'aws/Name': 'db-bastion-4-13biz',
          engine: '🐘',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resource: makeDatabase({
        name: 'super-long-server-db-with-uuid-2f96e498-88ec-442f-a25b-569fa915041c',
        uri: `${longClusterUri}/dbs/super-long-desc`,
        labels: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Accounting': 'dev-ops',
          'aws/Name': 'db-bastion-4-13biz',
          engine: '🐘',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      requiresRequest: true,
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/no-desc`,
        name: 'no-desc',
        desc: '',
        labels: makeLabelsList({
          'aws/Accounting': 'dev-ops',
          'aws/Environment': 'demo-13-biz',
          'aws/Name': 'db-bastion-4-13biz',
          'aws/Owner': 'foobar',
          'aws/Service': 'teleport-db',
          engine: '🐘',
          env: 'dev',
          'teleport.dev/origin': 'config-file',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'kube',
      labelMatches: [
        {
          kind: 'label-value',
          labelName: 'kube',
          searchTerm: 'kubersson',
          score: 100,
        },
      ],
      resource: makeKube({
        name: 'short-label-list',
        labels: makeLabelsList({
          'im-just-a-smol': 'kube',
          kube: 'kubersson',
          with: 'little-to-no-labels',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'kube',
      labelMatches: [
        {
          kind: 'label-name',
          labelName: 'aws/Environment',
          searchTerm: 'Environment',
          score: 60,
        },
        {
          kind: 'label-name',
          labelName: 'aws/Owner',
          searchTerm: 'Owner',
          score: 45,
        },
        {
          kind: 'label-value',
          labelName: 'kube',
          searchTerm: 'kubersson',
          score: 70,
        },
      ],
      resource: makeKube({
        name: 'long-label-list',
        uri: `${clusterUri}/kubes/long-label-list`,
        labels: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          'aws/Name': 'db-bastion-4-13biz',
          kube: 'kubersson',
          with: 'little-to-no-labels',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'kube',
      resource: makeKube({
        name: 'super-long-kube-name-with-uuid-2f96e498-88ec-442f-a25b-569fa915041c',
        uri: `/clusters/teleport-very-long-cluster-name-with-uuid-2f96e498-88ec-442f-a25b-569fa915041c/kubes/super-long-desc`,
        labels: makeLabelsList({
          'im-just-a-smol': 'kube',
          kube: 'kubersson',
          with: 'little-to-no-labels',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'kube',
      requiresRequest: true,
      labelMatches: [
        {
          kind: 'label-value',
          labelName: 'im-just-a-smol',
          searchTerm: 'kube',
          score: 40,
        },
      ],
      resource: makeKube({
        name: 'short-label-list',
        labels: makeLabelsList({
          'im-just-a-smol': 'kube',
          kube: 'kubersson',
          with: 'little-to-no-labels',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'windows_desktop',
      requiresRequest: false,
      resource: makeWindowsDesktopWithoutDefaultPort({
        uri: `${clusterUri}/windows_desktops/long-name`,
        name: 'super-long-windows-desktop-name-with-uuid-7a96e498-88ec-442f-a25b-569fa9150123c',
        labels: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          windowsDesktops: 'custom-windows-list',
          with: 'little-to-no-labels',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'windows_desktop',
      resource: makeWindowsDesktopWithoutDefaultPort({
        uri: `${clusterUri}/windows_desktops/long-label-list`,
        name: 'long-label-list',
        labels: makeLabelsList({
          'aws/Environment': 'demo-13-biz',
          'aws/Owner': 'foobar',
          'aws/Name': 'db-bastion-4-13biz',
          windowsDesktops: 'custom-windows-list',
          with: 'little-to-no-labels',
        }),
      }),
      resourceMatches: [
        { field: 'addrWithoutDefaultPort', searchTerm: '192.169.100.50' },
      ],
      labelMatches: [
        {
          kind: 'label-value',
          labelName: 'windowsDesktops',
          searchTerm: 'custom',
          score: 100,
        },
        {
          kind: 'label-name',
          labelName: 'aws/Environment',
          searchTerm: 'Environment',
          score: 60,
        },
        {
          kind: 'label-name',
          labelName: 'aws/Owner',
          searchTerm: 'Owner',
          score: 45,
        },
      ],
    }),
    makeResourceResult({
      kind: 'windows_desktop',
      requiresRequest: true,
      labelMatches: [
        {
          kind: 'label-value',
          labelName: 'im-just-a-smol',
          searchTerm: 'win',
          score: 40,
        },
      ],
      resource: makeWindowsDesktopWithoutDefaultPort({
        uri: `${clusterUri}/windows_desktops/short-label-list`,
        name: 'short-label-list',
        labels: makeLabelsList({
          'im-just-a-smol': 'win',
        }),
      }),
    }),
    {
      kind: 'resource-type-filter',
      resource: 'kube_cluster',
      nameMatch: '',
      score: 0,
    },
    {
      kind: 'cluster-filter',
      resource: makeRootCluster({
        name: 'teleport-local',
        uri: clusterUri,
        proxyHost: 'teleport-local.dev:3090',
      }),
      nameMatch: '',
      score: 0,
    },
    {
      kind: 'cluster-filter',
      resource: makeRootCluster({
        name: 'teleport-very-long-cluster-name-with-uuid-2f96e498-88ec-442f-a25b-569fa915041c',
        uri: longClusterUri,
        proxyHost: 'teleport-local.dev:3090',
      }),
      nameMatch: '',
      score: 0,
    },
    {
      kind: 'display-results',
      clusterUri,
      value: 'abc',
      resourceKinds: ['db'],
      documentUri: '/docs/abc',
    },
    {
      kind: 'display-results',
      clusterUri,
      value: 'abc',
      resourceKinds: ['node'],
      documentUri: undefined,
    },
    {
      kind: 'display-results',
      clusterUri,
      value: 'abc',
      resourceKinds: [],
      documentUri: undefined,
    },
  ];
  const attempt = makeSuccessAttempt(searchResults);

  return (
    <ResultList<SearchResult>
      attempts={[attempt]}
      onPick={() => {}}
      onBack={() => {}}
      addWindowEventListener={() => ({
        cleanup: () => {},
      })}
      render={searchResult => {
        const Component = ComponentMap[searchResult.kind];

        return {
          key: getKey(searchResult),
          Component: (
            <Component
              searchResult={searchResult}
              getOptionalClusterName={routing.parseClusterName}
              isVnetSupported={true}
            />
          ),
        };
      }}
    />
  );
};

function getKey(searchResult: SearchResult): string {
  switch (searchResult.kind) {
    case 'resource-type-filter':
      return searchResult.resource;
    case 'display-results':
      return searchResult.value;
    default:
      return searchResult.resource.uri;
  }
}

const AuxiliaryItems = () => {
  const [advancedSearchEnabled, setAdvancedSearchEnabled] = useState(false);
  const advancedSearch = {
    isToggled: advancedSearchEnabled,
    onToggle: () => setAdvancedSearchEnabled(prevState => !prevState),
  };

  return (
    <ResultList<string>
      onPick={() => {}}
      onBack={() => {}}
      render={() => null}
      attempts={[]}
      addWindowEventListener={() => ({
        cleanup: () => {},
      })}
      ExtraTopComponent={
        <>
          <NonInteractiveItem>
            <AppItem
              searchResult={
                makeResourceResult({
                  kind: 'app',
                  resource: makeAppWithAddr({
                    uri: `${clusterUri}/apps/tcp-app`,
                    name: 'tcp-app-without-vnet',
                    endpointUri: 'tcp://localhost:3001',
                    desc: '',
                    labels: makeLabelsList({
                      access: 'cloudwatch-metrics,ec2,s3,cloudtrail',
                      'aws/Environment': 'demo-13-biz',
                      'aws/Owner': 'foobar',
                      env: 'dev',
                      'teleport.dev/origin': 'config-file',
                    }),
                  }),
                }) as SearchResultApp
              }
              getOptionalClusterName={routing.parseClusterName}
              isVnetSupported={false}
            />
          </NonInteractiveItem>
          <NoResultsItem
            clustersWithExpiredCerts={new Set()}
            getClusterName={routing.parseClusterName}
            advancedSearch={advancedSearch}
          />
          <NoResultsItem
            clustersWithExpiredCerts={new Set([clusterUri])}
            getClusterName={routing.parseClusterName}
            advancedSearch={advancedSearch}
          />
          <NoResultsItem
            clustersWithExpiredCerts={new Set([clusterUri, '/clusters/foobar'])}
            getClusterName={routing.parseClusterName}
            advancedSearch={advancedSearch}
          />
          <ResourceSearchErrorsItem
            getClusterName={routing.parseClusterName}
            showErrorsInModal={() => window.alert('Error details')}
            errors={[
              new ResourceSearchError(
                '/clusters/foo',
                new Error(
                  '14 UNAVAILABLE: connection error: desc = "transport: authentication handshake failed: EOF"'
                )
              ),
            ]}
            advancedSearch={advancedSearch}
          />
          <ResourceSearchErrorsItem
            getClusterName={routing.parseClusterName}
            showErrorsInModal={() => window.alert('Error details')}
            errors={[
              new ResourceSearchError(
                '/clusters/bar',
                new Error(
                  '2 UNKNOWN: Unable to connect to ssh proxy at teleport.local:443. Confirm connectivity and availability.\n	dial tcp: lookup teleport.local: no such host'
                )
              ),
              new ResourceSearchError(
                '/clusters/foo',
                new Error(
                  '14 UNAVAILABLE: connection error: desc = "transport: authentication handshake failed: EOF"'
                )
              ),
            ]}
            advancedSearch={advancedSearch}
          />
          <SuggestionsError
            statusText={
              '2 UNKNOWN: Unable to connect to ssh proxy at teleport.local:443. Confirm connectivity and availability.\n	dial tcp: lookup teleport.local: no such host'
            }
          />
          <NoSuggestionsAvailable message="No roles found." />
          <TypeToSearchItem
            hasNoRemainingFilterActions={false}
            advancedSearch={advancedSearch}
          />
          <TypeToSearchItem
            hasNoRemainingFilterActions={true}
            advancedSearch={advancedSearch}
          />
          <AdvancedSearchEnabledItem advancedSearch={advancedSearch} />
        </>
      }
    />
  );
};
