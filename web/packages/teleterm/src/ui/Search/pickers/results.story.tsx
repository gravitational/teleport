/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import { Flex } from 'design';

import { routing } from 'teleterm/ui/uri';
import {
  makeDatabase,
  makeKube,
  makeServer,
  makeLabelsList,
} from 'teleterm/services/tshd/testHelpers';
import { ResourceSearchError } from 'teleterm/ui/services/resources';

import { SearchResult } from '../searchResult';
import { makeResourceResult } from '../testHelpers';

import {
  ComponentMap,
  NoResultsItem,
  ResourceSearchErrorsItem,
  TypeToSearchItem,
} from './ActionPicker';
import { SuggestionsError } from './ParameterPicker';
import { ResultList } from './ResultList';

import type * as uri from 'teleterm/ui/uri';

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

const SearchResultItems = () => {
  const searchResults: SearchResult[] = [
    makeResourceResult({
      kind: 'server',
      resource: makeServer({
        hostname: 'long-label-list',
        uri: `${clusterUri}/servers/2f96e498-88ec-442f-a25b-569fa915041c`,
        name: '2f96e498-88ec-442f-a25b-569fa915041c',
        labelsList: makeLabelsList({
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
      resource: makeServer({
        hostname: 'short-label-list',
        addr: '',
        tunnel: true,
        uri: `${clusterUri}/servers/90a29595-aac7-42eb-a484-c6c0e23f1a21`,
        name: '90a29595-aac7-42eb-a484-c6c0e23f1a21',
        labelsList: makeLabelsList({
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
        labelsList: makeLabelsList({
          internal: '10.0.0.175',
          service: 'ansible',
          external: '32.192.113.93',
          arch: 'aarch64',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'server',
      resource: makeServer({
        hostname:
          'super-long-server-name-with-uuid-2f96e498-88ec-442f-a25b-569fa915041c',
        uri: `${longClusterUri}/servers/super-long-desc`,
        labelsList: makeLabelsList({
          internal: '10.0.0.175',
          service: 'ansible',
          external: '32.192.113.93',
          arch: 'aarch64',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'database',
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/no-desc`,
        name: 'no-desc',
        desc: '',
        labelsList: makeLabelsList({
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
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/short-desc`,
        name: 'short-desc',
        desc: 'Lorem ipsum',
        labelsList: makeLabelsList({
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
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/long-desc`,
        name: 'long-desc',
        desc: 'Eget dignissim lectus nisi vitae nunc',
        labelsList: makeLabelsList({
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
      resource: makeDatabase({
        uri: `${clusterUri}/dbs/super-long-desc`,
        name: 'super-long-desc',
        desc: 'Duis id tortor at purus tincidunt finibus. Mauris eu semper orci, non commodo lacus. Praesent sollicitudin magna id laoreet porta. Nunc lobortis varius sem vel fringilla.',
        labelsList: makeLabelsList({
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
        labelsList: makeLabelsList({
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
      kind: 'kube',
      resource: makeKube({
        name: 'short-label-list',
        labelsList: makeLabelsList({
          'im-just-a-smol': 'kube',
          kube: 'kubersson',
          with: 'little-to-no-labels',
        }),
      }),
    }),
    makeResourceResult({
      kind: 'kube',
      resource: makeKube({
        name: 'long-label-list',
        uri: `${clusterUri}/kubes/long-label-list`,
        labelsList: makeLabelsList({
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
        labelsList: makeLabelsList({
          'im-just-a-smol': 'kube',
          kube: 'kubersson',
          with: 'little-to-no-labels',
        }),
      }),
    }),
    {
      kind: 'resource-type-filter',
      resource: 'kubes',
      nameMatch: '',
      score: 0,
    },
    {
      kind: 'cluster-filter',
      resource: {
        name: 'teleport-local',
        uri: clusterUri,
        authClusterId: '',
        connected: true,
        leaf: false,
        proxyHost: 'teleport-local.dev:3090',
      },
      nameMatch: '',
      score: 0,
    },
    {
      kind: 'cluster-filter',
      resource: {
        name: 'teleport-very-long-cluster-name-with-uuid-2f96e498-88ec-442f-a25b-569fa915041c',
        uri: longClusterUri,
        authClusterId: '',
        connected: true,
        leaf: false,
        proxyHost: 'teleport-local.dev:3090',
      },
      nameMatch: '',
      score: 0,
    },
  ];
  const attempt = makeSuccessAttempt(searchResults);

  return (
    <ResultList<SearchResult>
      attempts={[attempt]}
      onPick={() => {}}
      onBack={() => {}}
      addWindowEventListener={() => ({ cleanup: () => {} })}
      render={searchResult => {
        const Component = ComponentMap[searchResult.kind];

        return {
          key:
            searchResult.kind !== 'resource-type-filter'
              ? searchResult.resource.uri
              : searchResult.resource,
          Component: (
            <Component
              searchResult={searchResult}
              getOptionalClusterName={routing.parseClusterName}
            />
          ),
        };
      }}
    />
  );
};

const AuxiliaryItems = () => (
  <ResultList<string>
    onPick={() => {}}
    onBack={() => {}}
    render={() => null}
    attempts={[]}
    addWindowEventListener={() => ({ cleanup: () => {} })}
    ExtraTopComponent={
      <>
        <NoResultsItem
          clustersWithExpiredCerts={new Set()}
          getClusterName={routing.parseClusterName}
        />
        <NoResultsItem
          clustersWithExpiredCerts={new Set([clusterUri])}
          getClusterName={routing.parseClusterName}
        />
        <NoResultsItem
          clustersWithExpiredCerts={new Set([clusterUri, '/clusters/foobar'])}
          getClusterName={routing.parseClusterName}
        />
        <ResourceSearchErrorsItem
          getClusterName={routing.parseClusterName}
          showErrorsInModal={() => window.alert('Error details')}
          errors={[
            new ResourceSearchError(
              '/clusters/foo',
              'server',
              new Error(
                '14 UNAVAILABLE: connection error: desc = "transport: authentication handshake failed: EOF"'
              )
            ),
          ]}
        />
        <ResourceSearchErrorsItem
          getClusterName={routing.parseClusterName}
          showErrorsInModal={() => window.alert('Error details')}
          errors={[
            new ResourceSearchError(
              '/clusters/bar',
              'database',
              new Error(
                '2 UNKNOWN: Unable to connect to ssh proxy at teleport.local:443. Confirm connectivity and availability.\n	dial tcp: lookup teleport.local: no such host'
              )
            ),
            new ResourceSearchError(
              '/clusters/foo',
              'server',
              new Error(
                '14 UNAVAILABLE: connection error: desc = "transport: authentication handshake failed: EOF"'
              )
            ),
          ]}
        />
        <SuggestionsError
          statusText={
            '2 UNKNOWN: Unable to connect to ssh proxy at teleport.local:443. Confirm connectivity and availability.\n	dial tcp: lookup teleport.local: no such host'
          }
        />
        <TypeToSearchItem hasNoRemainingFilterActions={false} />
        <TypeToSearchItem hasNoRemainingFilterActions={true} />
      </>
    }
  />
);
