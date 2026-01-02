/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query';
import { useCallback, useMemo } from 'react';
import { useHistory, useLocation } from 'react-router';
import styled from 'styled-components';

import { Alert } from 'design/Alert';
import Box from 'design/Box';
import Flex from 'design/Flex/Flex';
import { MultiselectMenu } from 'shared/components/Controls/MultiselectMenu';
import { SortMenu } from 'shared/components/Controls/SortMenuV2';
import { SearchPanel } from 'shared/components/Search';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import cfg from 'teleport/config';
import type { SortType } from 'teleport/services/agents';
import api from 'teleport/services/api';
import { ApiError } from 'teleport/services/api/parseError';
import type { UnifiedInstancesResponse } from 'teleport/services/instances/types';
import useTeleport from 'teleport/useTeleport';

import { InstancesList } from './InstancesList';
import {
  buildVersionPredicate,
  CustomOperator,
  FilterOption,
  VersionsFilterPanel,
} from './VersionsFilterPanel';

async function fetchInstances(
  variables: {
    clusterId: string;
    limit: number;
    startKey?: string;
    query?: string;
    search?: string;
    sort?: SortType;
    types?: string;
    services?: string;
    upgraders?: string;
  },
  signal?: AbortSignal
): Promise<UnifiedInstancesResponse> {
  const { clusterId, ...params } = variables;

  const response = await api.get(
    cfg.getInstancesUrl(clusterId, params),
    signal
  );

  return {
    instances: response?.instances || [],
    startKey: response?.startKey,
  };
}

export function Instances() {
  const history = useHistory();
  const location = useLocation();
  const queryParams = new URLSearchParams(location.search);
  const query = queryParams.get('query') ?? '';
  const isAdvancedQuery = Boolean(queryParams.get('is_advanced'));
  const sortField = queryParams.get('sort') || 'name';
  const sortDir = queryParams.get('sort_dir') || 'ASC';

  const typesParam = queryParams.get('types');
  const selectedTypes = (
    typesParam ? typesParam.split(',') : []
  ) as InstanceType[];

  const servicesParam = queryParams.get('services');
  const selectedServices = (
    servicesParam ? servicesParam.split(',') : []
  ) as ServiceType[];

  const upgradersParam = queryParams.get('upgraders');
  const selectedUpgraders = (
    upgradersParam ? upgradersParam.split(',') : []
  ) as UpgraderType[];

  const versionFilter = queryParams.get('version_filter') || '';
  const versionOperator = queryParams.get('version_operator') || '';
  const versionValue1 = queryParams.get('version_value1') || '';
  const versionValue2 = queryParams.get('version_value2') || '';

  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const authVersion = ctx.storeUser.state.cluster.authVersion;
  const flags = ctx.getFeatureFlags();

  const hasInstancePermissions = flags.listInstances && flags.readInstances;
  const hasBotInstancePermissions =
    flags.listBotInstances && flags.readBotInstances;
  const hasAnyPermissions = hasInstancePermissions || hasBotInstancePermissions;

  // versionPredicateQuery is the predicate query for the selected version filter, if any.
  // Under the hood, the version filter works by appending a predicate query to the request which
  // applies the selected version filters
  const versionPredicateQuery = useMemo(
    () =>
      buildVersionPredicate(
        versionFilter,
        versionOperator,
        versionValue1,
        versionValue2,
        authVersion
      ),
    [versionFilter, versionOperator, versionValue1, versionValue2, authVersion]
  );

  // If there is also an existing predicate query (ie. the user made an advanced search), we append the version predicate query to it in the request
  const combinedQuery = useMemo(() => {
    if (versionPredicateQuery && query && isAdvancedQuery) {
      return `(${query}) && (${versionPredicateQuery})`;
    }

    if (versionPredicateQuery) {
      return versionPredicateQuery;
    }

    if (isAdvancedQuery && query) {
      return query;
    }

    return '';
  }, [query, isAdvancedQuery, versionPredicateQuery]);

  const onlyBotInstancesSelected =
    selectedTypes.length === 1 && selectedTypes[0] === 'bot_instance';

  const {
    isSuccess,
    data,
    isFetching,
    isFetchingNextPage,
    error,
    hasNextPage,
    fetchNextPage,
  } = useInfiniteQuery({
    enabled: hasAnyPermissions,
    queryKey: [
      'instances',
      'list',
      clusterId,
      sortField,
      sortDir,
      query,
      isAdvancedQuery,
      selectedTypes.join(','),
      selectedServices.join(','),
      selectedUpgraders.join(','),
      versionPredicateQuery,
    ],
    queryFn: ({ pageParam, signal }) =>
      fetchInstances(
        {
          clusterId,
          limit: 32,
          startKey: pageParam,
          query: combinedQuery || undefined,
          search: !isAdvancedQuery ? query : undefined,
          sort: { fieldName: sortField, dir: sortDir as 'ASC' | 'DESC' },
          types: selectedTypes.length > 0 ? selectedTypes.join(',') : undefined,
          services:
            selectedServices.length > 0
              ? selectedServices.join(',')
              : undefined,
          upgraders:
            selectedUpgraders.length > 0
              ? selectedUpgraders.join(',')
              : undefined,
        },
        signal
      ),
    initialPageParam: '',
    getNextPageParam: data => data?.startKey || undefined,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });

  // Check if the error is due to cache initialization (HTTP 503)
  const isCacheInitializing =
    error instanceof ApiError && error.response.status === 503;

  const updateSearch = useCallback(
    (updateFn: (search: URLSearchParams) => void) => {
      const search = new URLSearchParams(location.search);
      updateFn(search);
      history.push({
        pathname: location.pathname,
        search: search.toString(),
      });
    },
    [history, location.pathname, location.search]
  );

  const handleQueryChange = useCallback(
    (query: string, isAdvanced: boolean) =>
      updateSearch(search => {
        if (query) {
          search.set('query', query);
        } else {
          search.delete('query');
        }
        if (isAdvanced) {
          search.set('is_advanced', '1');
        } else {
          search.delete('is_advanced');
        }
      }),
    [updateSearch]
  );

  const handleSortChange = useCallback(
    (sortField: string, sortDir: string) => {
      const search = new URLSearchParams(location.search);
      search.set('sort', sortField);
      search.set('sort_dir', sortDir);

      history.replace({
        pathname: location.pathname,
        search: search.toString(),
      });
    },
    [history, location.pathname, location.search]
  );

  const handleTypesChange = useCallback(
    (types: InstanceType[]) =>
      updateSearch(search => {
        if (types.length > 0) {
          search.set('types', types.join(','));
        } else {
          search.delete('types');
        }
      }),
    [updateSearch]
  );

  const handleServicesChange = useCallback(
    (services: ServiceType[]) =>
      updateSearch(search => {
        if (services.length > 0) {
          search.set('services', services.join(','));
        } else {
          search.delete('services');
        }
      }),
    [updateSearch]
  );

  const handleUpgradersChange = useCallback(
    (upgraders: UpgraderType[]) =>
      updateSearch(search => {
        if (upgraders.length > 0) {
          search.set('upgraders', upgraders.join(','));
        } else {
          search.delete('upgraders');
        }
      }),
    [updateSearch]
  );

  const handleVersionFilterChange = useCallback(
    (filter: {
      selectedOption: string;
      operator: string;
      value1: string;
      value2: string;
    }) =>
      updateSearch(search => {
        if (filter.selectedOption) {
          // If it's one of the preset filters, set it as the version_filter param in the route
          search.set('version_filter', filter.selectedOption);

          // For a custom condition version filter, we also set the custom version values
          if (filter.selectedOption === 'custom') {
            search.set('version_operator', filter.operator);
            if (filter.value1) {
              search.set('version_value1', filter.value1);
            } else {
              search.delete('version_value1');
            }

            if (filter.value2) {
              search.set('version_value2', filter.value2);
            } else {
              search.delete('version_value2');
            }
          } else {
            search.delete('version_operator');
            search.delete('version_value1');
            search.delete('version_value2');
          }
        } else {
          search.delete('version_filter');
          search.delete('version_operator');
          search.delete('version_value1');
          search.delete('version_value2');
        }
      }),
    [updateSearch]
  );

  const flatData = useMemo(
    () => (isSuccess ? data.pages.flatMap(page => page.instances) : []),
    [data?.pages, isSuccess]
  );

  // If they have neither instances nor bot instances permissions, just render a message informing them
  if (!hasAnyPermissions) {
    return (
      <FeatureBox>
        <FeatureHeader mb="0">
          <FeatureHeaderTitle>Instance Inventory</FeatureHeaderTitle>
        </FeatureHeader>
        <Alert kind="info" mt={4}>
          You do not have permission to view the instance inventory. Missing
          permissions: <code>instance.list</code> or <code>instance.read</code>,
          and <code>bot_instance.list</code> or <code>bot_instance.read</code>.
        </Alert>
      </FeatureBox>
    );
  }

  if (isCacheInitializing) {
    return (
      <FeatureBox>
        <FeatureHeader mb="0">
          <FeatureHeaderTitle>Instance Inventory</FeatureHeaderTitle>
        </FeatureHeader>
        <Alert kind="info" mt={3} mb={3}>
          The instance inventory is not yet ready to be displayed, please check
          back in a few minutes.
        </Alert>
      </FeatureBox>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader mb="0">
        <FeatureHeaderTitle>Instance Inventory</FeatureHeaderTitle>
      </FeatureHeader>

      <Box>
        {!hasInstancePermissions && hasBotInstancePermissions && (
          <Alert kind="info" mt={3} mb={3}>
            You do not have permission to view instances. This list will only
            show bot instances.
            <br />
            Listing instances requires permissions <code>
              instance.list
            </code>{' '}
            and <code>instance.read</code>.
          </Alert>
        )}
        {hasInstancePermissions && !hasBotInstancePermissions && (
          <Alert kind="info" mt={3} mb={3}>
            You do not have permission to view bot instances. This list will
            only show instances.
            <br />
            Listing bot instances requires permissions{' '}
            <code>bot_instance.list</code> and <code>bot_instance.read</code>.
          </Alert>
        )}
        <SearchPanel
          filter={{
            query: isAdvancedQuery ? query : undefined,
            search: isAdvancedQuery ? undefined : query,
          }}
          updateSearch={query => handleQueryChange(query, false)}
          updateQuery={query => handleQueryChange(query, true)}
          mb={3}
        />
        <Flex justifyContent="space-between" alignItems="flex-start" mb={3}>
          <FiltersRow gap={2}>
            <MultiselectMenu
              options={typeOptions}
              selected={selectedTypes}
              onChange={handleTypesChange}
              label="Type"
              tooltip="Filter by instance type"
              buffered={true}
              disabled={!hasInstancePermissions}
            />
            <MultiselectMenu
              options={serviceOptions}
              selected={selectedServices}
              onChange={handleServicesChange}
              label="Services"
              tooltip={
                onlyBotInstancesSelected
                  ? 'You cannot filter bot instances by services'
                  : 'Filter by services'
              }
              buffered={true}
              disabled={onlyBotInstancesSelected}
            />
            <MultiselectMenu
              options={upgraderOptions}
              selected={selectedUpgraders}
              onChange={handleUpgradersChange}
              label="Upgrader"
              tooltip="Filter by upgrader"
              buffered={true}
            />
            <VersionsFilterPanel
              currentVersion={authVersion}
              onApply={handleVersionFilterChange}
              tooltip="Filter by version"
              filter={versionFilter as FilterOption}
              operator={versionOperator as CustomOperator}
              value1={versionValue1}
              value2={versionValue2}
            />
          </FiltersRow>
          <SortMenu
            items={sortFields}
            selectedKey={sortField}
            selectedOrder={sortDir as 'ASC' | 'DESC'}
            onChange={(key, order) => {
              handleSortChange(key, order);
            }}
          />
        </Flex>
        <InstancesList
          data={flatData}
          isLoading={isFetching && !isFetchingNextPage}
          isFetchingNextPage={isFetchingNextPage}
          error={error}
          hasNextPage={hasNextPage}
          sortField={sortField}
          sortDir={sortDir}
          onSortChanged={handleSortChange}
          onLoadNextPage={fetchNextPage}
        />
      </Box>
    </FeatureBox>
  );
}

const FiltersRow = styled(Flex)`
  flex-wrap: wrap;
`;

type InstanceType = 'instance' | 'bot_instance';

type ServiceType =
  | 'App'
  | 'Db'
  | 'WindowsDesktop'
  | 'Kube'
  | 'Node'
  | 'Auth'
  | 'Proxy';

type UpgraderType =
  | ''
  | 'kube-updater'
  | 'unit-updater'
  | 'systemd-unit-updater';

const typeOptions: { value: InstanceType; label: string }[] = [
  { value: 'instance', label: 'Instances' },
  { value: 'bot_instance', label: 'Bot Instances' },
];

const serviceOptions: { value: ServiceType; label: string }[] = [
  { value: 'App', label: 'Applications' },
  { value: 'Db', label: 'Databases' },
  { value: 'WindowsDesktop', label: 'Desktops' },
  { value: 'Kube', label: 'Kubernetes Clusters' },
  { value: 'Node', label: 'SSH Servers' },
  { value: 'Auth', label: 'Auth' },
  { value: 'Proxy', label: 'Proxy' },
];

const upgraderOptions = [
  { value: '', label: 'None' },
  { value: 'unit-updater', label: 'Unit Updater (legacy)' },
  {
    value: 'systemd-unit-updater',
    label: 'Systemd Unit Updater',
  },
  { value: 'kube-updater', label: 'Kubernetes' },
];

const sortFields = [
  { key: 'name', label: 'Name' },
  { key: 'version', label: 'Version' },
  { key: 'type', label: 'Type' },
];
