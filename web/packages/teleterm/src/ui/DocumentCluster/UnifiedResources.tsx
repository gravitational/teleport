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

import React, { useState, useCallback, useEffect, useRef } from 'react';

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

import { useStore } from 'shared/libs/stores';

import { UnifiedResourceResponse } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import * as uri from 'teleterm/ui/uri';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useWorkspaceLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import { useConnectMyComputerContext } from 'teleterm/ui/ConnectMyComputer';

import { retryWithRelogin } from 'teleterm/ui/utils';
import { SearchConnector } from 'teleterm/ui/services/resourceSearch/resourceSearchService';
import { DocumentClusterQueryParams } from 'teleterm/ui/services/workspacesService';

import {
  ConnectServerActionButton,
  ConnectKubeActionButton,
  ConnectDatabaseActionButton,
} from './actionButtons';
import { useResourcesContext } from './resourcesContext';

interface UnifiedResourcesProps {
  clusterUri: uri.ClusterUri;
  visible: boolean;
  initialQueryParams?: DocumentClusterQueryParams;
}

export function UnifiedResources(props: UnifiedResourcesProps) {
  const appContext = useAppContext();
  const { onResourcesRefreshRequest } = useResourcesContext();

  const cont = useRef<SearchConnector>();
  if (!cont.current) {
    cont.current = new SearchConnector(
      props.clusterUri,
      props.initialQueryParams || {
        search: '',
        kinds: [],
        isAdvancedSearchEnabled: false,
      }
    );
  }

  const {
    state: { search, isAdvancedSearchEnabled, kinds },
  } = useStore(cont.current, () => clear());

  const [params, setParams] = useState<{
    sort?: {
      fieldName: string;
      dir: 'ASC' | 'DESC';
    };
    pinnedOnly?: boolean;
  }>(() => ({
    sort: { fieldName: 'name', dir: 'ASC' },
  }));

  const mergedParams: UnifiedResourcesQueryParams = {
    kinds,
    sort: params.sort,
    pinnedOnly: params.pinnedOnly,
    search: isAdvancedSearchEnabled ? '' : search,
    query: isAdvancedSearchEnabled ? search : '',
  };

  useEffect(() => {
    if (props.visible) {
      appContext.resourceSearchService.setConnector(cont.current);

      return () => {
        if (appContext.resourceSearchService.getConnector() === cont.current) {
          appContext.resourceSearchService.setConnector(undefined);
        }
      };
    }
  }, [appContext.resourceSearchService, props.visible]);

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

  const { fetch, resources, attempt, clear } = useUnifiedResourcesFetch({
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
                  isDesc: mergedParams.sort.dir === 'DESC',
                  field: mergedParams.sort.fieldName,
                },
                search: mergedParams.search,
                kindsList: mergedParams.kinds,
                query: mergedParams.query,
                pinnedOnly: mergedParams.pinnedOnly,
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
      [
        appContext,
        mergedParams.kinds,
        mergedParams.pinnedOnly,
        mergedParams.query,
        mergedParams.search,
        mergedParams.sort.dir,
        mergedParams.sort.fieldName,
        props.clusterUri,
      ]
    ),
  });

  useEffect(() => {
    const { cleanup } = onResourcesRefreshRequest(() => {
      clear();
      fetch({ force: true });
    });
    return cleanup;
  }, [onResourcesRefreshRequest, fetch, clear]);

  function onParamsChange(newParams: UnifiedResourcesQueryParams): void {
    cont.current.update({
      search: newParams.search,
      kinds: newParams.kinds as any,
    });
    clear();
    setParams(prevState => ({
      ...prevState,
      sort: newParams.sort,
      pinnedOnly: newParams.pinnedOnly,
    }));
  }

  return (
    <SharedUnifiedResources
      params={mergedParams}
      setParams={onParamsChange}
      updateUnifiedResourcesPreferences={() => alert('Not implemented')}
      onLabelClick={() => alert('Not implemented')}
      pinning={{ kind: 'hidden' }}
      resources={resources.map(mapToSharedResource)}
      resourcesFetchAttempt={attempt}
      fetchResources={fetch}
      availableKinds={['db', 'kube_cluster', 'node']}
      Header={() => (
        <Flex alignItems="center" justifyContent="space-between">
          {/*/!*temporary search panel*!/*/}
          {/*<SearchPanel*/}
          {/*  params={params}*/}
          {/*  pathname={''}*/}
          {/*  replaceHistory={() => undefined}*/}
          {/*  setParams={onParamsChange}*/}
          {/*/>*/}
          {/*{pinAllButton}*/}
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
