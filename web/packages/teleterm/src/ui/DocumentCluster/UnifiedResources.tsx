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

import React, { useState, useCallback, useEffect } from 'react';

import {
  UnifiedResources as SharedUnifiedResources,
  useUnifiedResourcesFetch,
  UnifiedResourcesQueryParams,
  SharedUnifiedResource,
} from 'shared/components/UnifiedResources';
import {
  DbProtocol,
  formatDatabaseInfo,
  DbType,
} from 'shared/services/databases';

import { Flex, ButtonPrimary, Text } from 'design';

import * as icons from 'design/Icon';
import Image from 'design/Image';
import stack from 'design/assets/resources/stack.png';

import SearchPanel from 'teleport/UnifiedResources/SearchPanel';

import { UnifiedResourceResponse } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import * as uri from 'teleterm/ui/uri';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useWorkspaceLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import { useConnectMyComputerContext } from 'teleterm/ui/ConnectMyComputer';

import { retryWithRelogin } from 'teleterm/ui/utils';

import {
  ConnectServerActionButton,
  ConnectKubeActionButton,
  ConnectDatabaseActionButton,
} from './actionButtons';
import { useResourcesContext } from './resourcesContext';

interface UnifiedResourcesProps {
  clusterUri: uri.ClusterUri;
}

export function UnifiedResources(props: UnifiedResourcesProps) {
  const appContext = useAppContext();
  const { onResourcesRefreshRequest } = useResourcesContext();

  const [params, setParams] = useState<UnifiedResourcesQueryParams>({
    sort: { fieldName: 'name', dir: 'ASC' },
  });

  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const loggedInUser = useWorkspaceLoggedInUser();
  const { canUse: hasPermissionsForConnectMyComputer, agentCompatibility } =
    useConnectMyComputerContext();

  const isRootCluster = props.clusterUri === rootClusterUri;
  const canAddResources = isRootCluster && loggedInUser?.acl?.tokens.create;

  const canUseConnectMyComputer =
    isRootCluster &&
    hasPermissionsForConnectMyComputer &&
    agentCompatibility === 'compatible';

  const { fetch, resources, attempt } = useUnifiedResourcesFetch({
    fetchFunc: useCallback(
      async (paginationParams, signal) => {
        const response = await retryWithRelogin(
          appContext,
          props.clusterUri,
          () =>
            appContext.resourcesService.listUnifiedResources(
              {
                clusterUri: props.clusterUri,
                searchAsRoles: false,
                sortBy: {
                  isDesc: params.sort.dir === 'DESC',
                  field: params.sort.fieldName,
                },
                search: params.search,
                kindsList: params.kinds,
                query: params.query,
                pinnedOnly: params.pinnedOnly,
                startKey: paginationParams.startKey,
                limit: paginationParams.limit,
              },
              signal
            )
        );

        return {
          startKey: response.nextKey,
          agents: response.resources,
          totalCount: response.resources.length,
        };
      },
      [appContext, params, props.clusterUri]
    ),
  });

  useEffect(() => {
    const { cleanup } = onResourcesRefreshRequest(() =>
      fetch({
        force: true,
        fromStart: true,
      })
    );
    return cleanup;
  }, [onResourcesRefreshRequest, fetch]);

  return (
    <SharedUnifiedResources
      params={params}
      setParams={setParams}
      updateUnifiedResourcesPreferences={() => alert('Not implemented')}
      onLabelClick={() => alert('Not implemented')}
      pinning={{ kind: 'hidden' }}
      resources={resources.map(mapToSharedResource)}
      resourcesFetchAttempt={attempt}
      fetchResources={fetch}
      availableKinds={['db', 'kube_cluster', 'node']}
      Header={pinAllButton => (
        <Flex alignItems="center" justifyContent="space-between">
          {/*temporary search panel*/}
          <SearchPanel
            params={params}
            pathname={''}
            replaceHistory={() => undefined}
            setParams={setParams}
          />
          {pinAllButton}
        </Flex>
      )}
      NoResources={
        <NoResources
          canCreate={canAddResources}
          canUseConnectMyComputer={canUseConnectMyComputer}
          onConnectMyComputerCtaClick={() => {
            documentsService.openConnectMyComputerDocument({ rootClusterUri });
          }}
        />
      }
    />
  );
}

const mapToSharedResource = (
  resource: UnifiedResourceResponse
): SharedUnifiedResource => {
  switch (resource.kind) {
    case 'server': {
      const { resource: server } = resource;
      return {
        resource: {
          kind: 'node' as const,
          labels: server.labelsList,
          id: server.name,
          hostname: server.hostname,
          addr: server.addr,
          tunnel: server.tunnel,
        },
        ui: {
          ActionButton: <ConnectServerActionButton server={server} />,
        },
      };
    }
    case 'database': {
      const { resource: database } = resource;
      return {
        resource: {
          kind: 'db' as const,
          labels: database.labelsList,
          description: database.desc,
          name: database.name,
          type: formatDatabaseInfo(
            database.type as DbType,
            database.protocol as DbProtocol
          ).title,
          protocol: database.protocol as DbProtocol,
        },
        ui: {
          ActionButton: <ConnectDatabaseActionButton database={database} />,
        },
      };
    }
    case 'kube': {
      const { resource: kube } = resource;

      return {
        resource: {
          kind: 'kube_cluster' as const,
          labels: kube.labelsList,
          name: kube.name,
        },
        ui: {
          ActionButton: <ConnectKubeActionButton kube={kube} />,
        },
      };
    }
  }
};

function NoResources(props: {
  canCreate: boolean;
  canUseConnectMyComputer: boolean;
  onConnectMyComputerCtaClick(): void;
}) {
  let $content: React.ReactElement;
  if (!props.canCreate) {
    $content = (
      <>
        <Text typography="h3" mb="2" fontWeight={600}>
          No Resources Found
        </Text>
        <Text>
          Either there are no resources in the cluster, or your roles don't
          grant you access.
        </Text>
      </>
    );
  } else {
    $content = (
      <>
        <Image src={stack} ml="auto" mr="auto" mb={4} height="100px" />
        <Text typography="h3" mb={2} fontWeight={600}>
          Add your first resource to Teleport
        </Text>
        <Text color="text.slightlyMuted">
          {props.canUseConnectMyComputer
            ? 'You can add it in the Teleport Web UI or by connecting your computer to the cluster.'
            : 'Connect SSH servers, Kubernetes clusters, Databases and more from Teleport Web UI.'}
        </Text>
        {props.canUseConnectMyComputer && (
          <ButtonPrimary
            type="button"
            mt={3}
            gap={2}
            onClick={props.onConnectMyComputerCtaClick}
          >
            <icons.Laptop size={'medium'} />
            Connect My Computer
          </ButtonPrimary>
        )}
      </>
    );
  }

  return (
    <Flex
      maxWidth={600}
      p={8}
      pt={5}
      width="100%"
      mx="auto"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
    >
      {$content}
    </Flex>
  );
}
