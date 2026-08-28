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

import { useEffect, useMemo, useState } from 'react';

import { Flex } from 'design';
import { StyledArrowBtn } from 'design/DataTable/Pager/StyledPager';
import { StyledPanel } from 'design/DataTable/StyledTable';
import { SortType } from 'design/DataTable/types';
import { CircleArrowLeft, CircleArrowRight } from 'design/Icon';
import { SearchPanel } from 'shared/components/Search';
import { makeAdvancedSearchQueryForLabel } from 'shared/utils/advancedSearchLabelQuery';

import { useServerSidePagination } from 'teleport/components/hooks';
import cfg, { UrlResourcesParams } from 'teleport/config';
import type {
  ResourceFilter,
  ResourceLabel,
  ResourcesResponse,
} from 'teleport/services/agents';
import { Desktop } from 'teleport/services/desktops';
import { Node } from 'teleport/services/nodes';
import { RoleResource } from 'teleport/services/resources';
import Ctx from 'teleport/teleportContext';
import useTeleport from 'teleport/useTeleport';

import { CommonListProps, LockResourceKind } from '../../common';
import { ServerSideListProps, TableWrapper } from '../common';
import { Desktops } from './Desktops';
import { Nodes } from './Nodes';
import { Roles } from './Roles';

export function ServerSideSupportedList(props: CommonListProps) {
  const ctx = useTeleport();

  const [resourceFilter, setResourceFilter] = useState<ResourceFilter>({});

  const {
    fetchStatus,
    fetchNext,
    fetchPrev,
    fetch,
    attempt: fetchAttempt,
    pageIndicators,
    fetchedData,
  } = useServerSidePagination({
    fetchFunc: getFetchFuncForServerSidePaginating(
      ctx,
      props.selectedResourceKind
    ),
    clusterId: cfg.proxyCluster, // Locking only supported with root cluster
    params: resourceFilter,
    pageSize: props.pageSize,
  });

  useEffect(() => {
    // Resetting the filter will trigger a fetch.
    setResourceFilter({
      sort: getDefaultSort(props.selectedResourceKind),
      search: '',
      query: '',
    });
  }, [props.selectedResourceKind]);

  useEffect(() => {
    fetch();
  }, [resourceFilter]);

  useEffect(() => {
    props.setAttempt(fetchAttempt);
  }, [fetchAttempt]);

  function updateSort(sort: SortType) {
    setResourceFilter({ ...resourceFilter, sort });
  }

  function updateSearch(search: string) {
    setResourceFilter({ ...resourceFilter, query: '', search });
  }

  function updateQuery(query: string) {
    setResourceFilter({ ...resourceFilter, search: '', query });
  }

  function onResourceLabelClick(label: ResourceLabel) {
    const query = makeAdvancedSearchQueryForLabel(label, resourceFilter);
    setResourceFilter({ ...resourceFilter, search: '', query });
  }

  const table = useMemo(() => {
    // If there is a fetchStatus, a fetching is going on.
    // Show the loading indicator instead of trying to process previous data.
    const resources = fetchStatus === 'loading' ? [] : fetchedData.agents;
    const listProps: ServerSideListProps = {
      fetchStatus,
      customSort: {
        dir: resourceFilter.sort?.dir,
        fieldName: resourceFilter.sort?.fieldName,
        onSort: updateSort,
      },
      onLabelClick: onResourceLabelClick,
      selectedResources: props.selectedResources,
      toggleSelectResource: props.toggleSelectResource,
    };

    switch (props.selectedResourceKind) {
      case 'role':
        return <Roles roles={resources as RoleResource[]} {...listProps} />;
      case 'node':
        return <Nodes nodes={resources as Node[]} {...listProps} />;
      case 'windows_desktop':
        return <Desktops desktops={resources as Desktop[]} {...listProps} />;
      default:
        console.error(
          `[ServerSideSupportedList.tsx] table not defined for resource kind ${props.selectedResourceKind}`
        );
    }
  }, [fetchedData, fetchStatus, props.selectedResources]);

  return (
    <TableWrapper
      className={fetchStatus === 'loading' ? 'disabled' : ''}
      css={`
        border-radius: 8px;
        overflow: hidden;
      `}
    >
      <SearchPanel
        updateQuery={updateQuery}
        updateSearch={updateSearch}
        pageIndicators={{
          from: pageIndicators.from,
          to: pageIndicators.to,
          total: pageIndicators.totalCount,
        }}
        hideAdvancedSearch={props.selectedResourceKind === 'role'} // Roles don't support advanced search.
        filter={resourceFilter}
        disableSearch={fetchStatus === 'loading'}
      />
      {table}
      <StyledPanel>
        <Flex justifyContent="flex-end" width="100%">
          <Flex alignItems="center" mr={2}></Flex>
          <Flex>
            <StyledArrowBtn
              onClick={fetchPrev}
              title="Previous page"
              disabled={!fetchPrev || fetchStatus === 'loading'}
              mx={0}
            >
              <CircleArrowLeft />
            </StyledArrowBtn>
            <StyledArrowBtn
              ml={0}
              onClick={fetchNext}
              title="Next page"
              disabled={!fetchNext || fetchStatus === 'loading'}
            >
              <CircleArrowRight />
            </StyledArrowBtn>
          </Flex>
        </Flex>
      </StyledPanel>
    </TableWrapper>
  );
}

function getDefaultSort(kind: LockResourceKind): SortType {
  if (kind === 'node') {
    return { fieldName: 'hostname', dir: 'ASC' };
  }
  return { fieldName: 'name', dir: 'ASC' };
}

function getFetchFuncForServerSidePaginating(
  ctx: Ctx,
  resourceKind: LockResourceKind
): (
  clusterId: string,
  params: UrlResourcesParams
) => Promise<ResourcesResponse<unknown>> {
  if (resourceKind === 'role') {
    return async (clusterId, params) => {
      const { items, startKey } = await ctx.resourceService.fetchRoles(params);
      return { agents: items, startKey };
    };
  }
  if (resourceKind === 'node') {
    return ctx.nodeService.fetchNodes;
  }

  if (resourceKind === 'windows_desktop') {
    return ctx.desktopService.fetchDesktops;
  }
}
