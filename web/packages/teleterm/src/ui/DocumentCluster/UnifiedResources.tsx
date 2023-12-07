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

import {
  UnifiedResourcePreferences,
  DefaultTab,
  ViewMode,
} from 'shared/services/unifiedResourcePreferences';

import { UnifiedResourceResponse } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import * as uri from 'teleterm/ui/uri';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useWorkspaceLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import { useConnectMyComputerContext } from 'teleterm/ui/ConnectMyComputer';

import { retryWithRelogin } from 'teleterm/ui/utils';
import {
  DocumentClusterQueryParams,
  DocumentCluster,
  DocumentClusterResourceKind,
} from 'teleterm/ui/services/workspacesService';

import {
  ConnectServerActionButton,
  ConnectKubeActionButton,
  ConnectDatabaseActionButton,
} from './actionButtons';
import { useResourcesContext } from './resourcesContext';

export function UnifiedResources(props: {
  clusterUri: uri.ClusterUri;
  docUri: uri.DocumentUri;
  queryParams: DocumentClusterQueryParams;
}) {
  // TODO: Add user preferences to Connect.
  // Until we add stored user preferences to Connect, store it in the state.
  const [userPreferences, setUserPreferences] =
    useState<UnifiedResourcePreferences>({
      defaultTab: DefaultTab.DEFAULT_TAB_ALL,
      viewMode: ViewMode.VIEW_MODE_CARD,
    });

  return (
    <Resources
      queryParams={props.queryParams}
      docUri={props.docUri}
      clusterUri={props.clusterUri}
      userPreferences={userPreferences}
      setUserPreferences={setUserPreferences}
      // Reset the component state when query params object change.
      // JSON.stringify on the same object will always produce the same string.
      key={JSON.stringify(props.queryParams)}
    />
  );
}

function Resources(props: {
  clusterUri: uri.ClusterUri;
  docUri: uri.DocumentUri;
  queryParams: DocumentClusterQueryParams;
  userPreferences: UnifiedResourcePreferences;
  setUserPreferences(u: UnifiedResourcePreferences): void;
}) {
  const appContext = useAppContext();
  const { onResourcesRefreshRequest } = useResourcesContext();

  const mergedParams: UnifiedResourcesQueryParams = {
    kinds: props.queryParams.resourceKinds,
    sort: props.queryParams.sort,
    pinnedOnly: false, //TODO: add support for pinning
    search: props.queryParams.advancedSearchEnabled
      ? ''
      : props.queryParams.search,
    query: props.queryParams.advancedSearchEnabled
      ? props.queryParams.search
      : '',
  };

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
    const documentService =
      appContext.workspacesService.getWorkspaceDocumentService(
        uri.routing.ensureRootClusterUri(props.clusterUri)
      );
    documentService.update(props.docUri, (draft: DocumentCluster) => {
      const { queryParams } = draft;
      queryParams.sort = newParams.sort;
      queryParams.resourceKinds =
        newParams.kinds as DocumentClusterResourceKind[];
      queryParams.search = newParams.search || newParams.query;
      queryParams.advancedSearchEnabled = !!newParams.query;
    });
  }

  return (
    <SharedUnifiedResources
      params={mergedParams}
      setParams={onParamsChange}
      unifiedResourcePreferences={props.userPreferences}
      updateUnifiedResourcesPreferences={props.setUserPreferences}
      pinning={{ kind: 'hidden' }}
      resources={resources.map(mapToSharedResource)}
      resourcesFetchAttempt={attempt}
      fetchResources={fetch}
      availableKinds={[
        {
          kind: 'node',
          disabled: false,
        },
        {
          kind: 'db',
          disabled: false,
        },
        {
          kind: 'kube_cluster',
          disabled: false,
        },
      ]}
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
          subKind: server.subKind,
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
