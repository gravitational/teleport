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

import { useCallback, useEffect, useMemo, memo } from 'react';

import {
  UnifiedResources as SharedUnifiedResources,
  useUnifiedResourcesFetch,
  UnifiedResourcesQueryParams,
  SharedUnifiedResource,
  UnifiedResourcesPinning,
} from 'shared/components/UnifiedResources';
import {
  DbProtocol,
  formatDatabaseInfo,
  DbType,
} from 'shared/services/databases';

import { Flex, ButtonPrimary, Text, Link } from 'design';

import * as icons from 'design/Icon';
import Image from 'design/Image';
import stack from 'design/assets/resources/stack.png';

import { Attempt } from 'shared/hooks/useAsync';

import { DefaultTab } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

import { NodeSubKind } from 'shared/services';

import { UserPreferences } from 'teleterm/services/tshd/types';
import { UnifiedResourceResponse } from 'teleterm/ui/services/resources';
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
import { getAppAddrWithProtocol } from 'teleterm/services/tshd/app';

import {
  ConnectServerActionButton,
  ConnectKubeActionButton,
  ConnectDatabaseActionButton,
  ConnectAppActionButton,
} from './ActionButtons';
import { useResourcesContext, ResourcesContext } from './resourcesContext';
import { useUserPreferences } from './useUserPreferences';

export function UnifiedResources(props: {
  clusterUri: uri.ClusterUri;
  docUri: uri.DocumentUri;
  queryParams: DocumentClusterQueryParams;
}) {
  const { clustersService } = useAppContext();
  const { userPreferencesAttempt, updateUserPreferences, userPreferences } =
    useUserPreferences(props.clusterUri);
  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const { onResourcesRefreshRequest } = useResourcesContext();
  const loggedInUser = useWorkspaceLoggedInUser();

  const { unifiedResourcePreferences } = userPreferences;

  const mergedParams: UnifiedResourcesQueryParams = useMemo(
    () => ({
      kinds: props.queryParams.resourceKinds,
      sort: props.queryParams.sort,
      pinnedOnly: unifiedResourcePreferences.defaultTab === DefaultTab.PINNED,
      search: props.queryParams.advancedSearchEnabled
        ? ''
        : props.queryParams.search,
      query: props.queryParams.advancedSearchEnabled
        ? props.queryParams.search
        : '',
    }),
    [
      props.queryParams.advancedSearchEnabled,
      props.queryParams.resourceKinds,
      props.queryParams.search,
      props.queryParams.sort,
      unifiedResourcePreferences.defaultTab,
    ]
  );

  const { canUse: hasPermissionsForConnectMyComputer, agentCompatibility } =
    useConnectMyComputerContext();

  const isRootCluster = props.clusterUri === rootClusterUri;
  const canAddResources = isRootCluster && loggedInUser?.acl?.tokens.create;
  let discoverUrl: string;
  if (isRootCluster) {
    const rootCluster = clustersService.findCluster(rootClusterUri);
    discoverUrl = `https://${rootCluster.proxyHost}/web/discover`;
  }

  const canUseConnectMyComputer =
    isRootCluster &&
    hasPermissionsForConnectMyComputer &&
    agentCompatibility === 'compatible';

  const openConnectMyComputerDocument = useCallback(() => {
    documentsService.openConnectMyComputerDocument({ rootClusterUri });
  }, [documentsService, rootClusterUri]);

  const onParamsChange = useCallback(
    (newParams: UnifiedResourcesQueryParams): void => {
      documentsService.update(props.docUri, (draft: DocumentCluster) => {
        const { queryParams } = draft;
        queryParams.sort = newParams.sort;
        queryParams.resourceKinds =
          newParams.kinds as DocumentClusterResourceKind[];
        queryParams.search = newParams.search || newParams.query;
        queryParams.advancedSearchEnabled = !!newParams.query;
      });
    },
    [documentsService, props.docUri]
  );

  return (
    <Resources
      queryParams={mergedParams}
      onParamsChange={onParamsChange}
      clusterUri={props.clusterUri}
      userPreferencesAttempt={userPreferencesAttempt}
      updateUserPreferences={updateUserPreferences}
      userPreferences={userPreferences}
      canAddResources={canAddResources}
      canUseConnectMyComputer={canUseConnectMyComputer}
      openConnectMyComputerDocument={openConnectMyComputerDocument}
      onResourcesRefreshRequest={onResourcesRefreshRequest}
      discoverUrl={discoverUrl}
      // Reset the component state when query params object change.
      // JSON.stringify on the same object will always produce the same string.
      key={JSON.stringify(mergedParams)}
    />
  );
}

const Resources = memo(
  (props: {
    clusterUri: uri.ClusterUri;
    queryParams: UnifiedResourcesQueryParams;
    onParamsChange(params: UnifiedResourcesQueryParams): void;
    userPreferencesAttempt?: Attempt<void>;
    userPreferences: UserPreferences;
    updateUserPreferences(u: UserPreferences): Promise<void>;
    canAddResources: boolean;
    canUseConnectMyComputer: boolean;
    openConnectMyComputerDocument(): void;
    onResourcesRefreshRequest: ResourcesContext['onResourcesRefreshRequest'];
    discoverUrl: string;
  }) => {
    const appContext = useAppContext();

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
                    isDesc: props.queryParams.sort.dir === 'DESC',
                    field: props.queryParams.sort.fieldName,
                  },
                  search: props.queryParams.search,
                  kinds: props.queryParams.kinds,
                  query: props.queryParams.query,
                  pinnedOnly: props.queryParams.pinnedOnly,
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
          props.queryParams.kinds,
          props.queryParams.pinnedOnly,
          props.queryParams.query,
          props.queryParams.search,
          props.queryParams.sort.dir,
          props.queryParams.sort.fieldName,
          props.clusterUri,
        ]
      ),
    });

    const { onResourcesRefreshRequest } = props;
    useEffect(() => {
      const { cleanup } = onResourcesRefreshRequest(() => {
        clear();
        fetch({ force: true });
      });
      return cleanup;
    }, [onResourcesRefreshRequest, fetch, clear]);

    const resourceIds =
      props.userPreferences.clusterPreferences?.pinnedResources?.resourceIds;
    const { updateUserPreferences } = props;
    const pinning = useMemo<UnifiedResourcesPinning>(() => {
      return resourceIds
        ? {
            kind: 'supported',
            getClusterPinnedResources: async () => resourceIds,
            updateClusterPinnedResources: pinnedIds =>
              updateUserPreferences({
                clusterPreferences: {
                  pinnedResources: { resourceIds: pinnedIds },
                },
              }),
          }
        : { kind: 'not-supported' };
    }, [updateUserPreferences, resourceIds]);

    return (
      <SharedUnifiedResources
        params={props.queryParams}
        setParams={props.onParamsChange}
        unifiedResourcePreferencesAttempt={props.userPreferencesAttempt}
        unifiedResourcePreferences={
          props.userPreferences.unifiedResourcePreferences
        }
        updateUnifiedResourcesPreferences={unifiedResourcePreferences =>
          props.updateUserPreferences({ unifiedResourcePreferences })
        }
        pinning={pinning}
        resources={resources.map(mapToSharedResource)}
        resourcesFetchAttempt={attempt}
        fetchResources={fetch}
        availableKinds={[
          {
            kind: 'node',
            disabled: false,
          },
          {
            kind: 'app',
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
            canCreate={props.canAddResources}
            discoverUrl={props.discoverUrl}
            canUseConnectMyComputer={props.canUseConnectMyComputer}
            onConnectMyComputerCtaClick={props.openConnectMyComputerDocument}
          />
        }
      />
    );
  }
);

const mapToSharedResource = (
  resource: UnifiedResourceResponse
): SharedUnifiedResource => {
  switch (resource.kind) {
    case 'server': {
      const { resource: server } = resource;
      return {
        resource: {
          kind: 'node' as const,
          labels: server.labels,
          id: server.name,
          hostname: server.hostname,
          addr: server.addr,
          tunnel: server.tunnel,
          subKind: server.subKind as NodeSubKind,
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
          labels: database.labels,
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
          labels: kube.labels,
          name: kube.name,
        },
        ui: {
          ActionButton: <ConnectKubeActionButton kube={kube} />,
        },
      };
    }
    case 'app': {
      const { resource: app } = resource;

      return {
        resource: {
          kind: 'app' as const,
          labels: app.labels,
          name: app.name,
          id: app.name,
          addrWithProtocol: getAppAddrWithProtocol(app),
          awsConsole: app.awsConsole,
          description: app.desc,
          friendlyName: app.friendlyName,
          samlApp: app.samlApp,
        },
        ui: {
          ActionButton: <ConnectAppActionButton app={app} />,
        },
      };
    }
  }
};

function NoResources(props: {
  canCreate: boolean;
  discoverUrl: string | undefined;
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
    const $discoverLink = (
      <Link href={props.discoverUrl} target="_blank">
        the&nbsp;Teleport Web UI
      </Link>
    );
    $content = (
      <>
        <Image src={stack} ml="auto" mr="auto" mb={4} height="100px" />
        <Text typography="h3" mb={2} fontWeight={600}>
          Add your first resource to Teleport
        </Text>
        <Text color="text.slightlyMuted">
          {props.canUseConnectMyComputer ? (
            <>
              You can add it in {$discoverLink} or by connecting your computer
              to the cluster.
            </>
          ) : (
            <>
              Connect SSH servers, Kubernetes clusters, Databases and more from{' '}
              {$discoverLink}.
            </>
          )}
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
