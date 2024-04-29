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

import React from 'react';
import styled from 'styled-components';
import { ButtonBorder, ButtonPrimary, Box } from 'design';
import Table, { Cell } from 'design/DataTable';
import { Desktop } from 'teleport/services/desktops';
import { Database } from 'teleport/services/databases';
import { App } from 'teleport/services/apps';
import { Kube } from 'teleport/services/kube';
import { Node } from 'teleport/services/nodes';
import { UserGroup } from 'teleport/services/userGroups';
import { CustomSort } from 'design/DataTable/types';

import { ResourceLabel, UnifiedResource } from 'teleport/services/agents';

import { ResourceMap, ResourceKind } from '../resource';

import { Apps } from './Apps';
import { Databases } from './Databases';
import { Nodes } from './Nodes';
import { Desktops } from './Desktops';
import { Kubes } from './Kubes';
import { Roles } from './Roles';
import { UserGroups } from './UserGroups';

export function ResourceList(props: ResourceListProps) {
  const {
    agents,
    disableRows,
    selectedResource,
    requestableRoles,
    ...listProps
  } = props;
  return (
    <Wrapper className={disableRows ? 'disabled' : ''}>
      {selectedResource === 'app' && (
        <Apps apps={agents as App[]} {...listProps} />
      )}
      {selectedResource === 'db' && (
        <Databases databases={agents as Database[]} {...listProps} />
      )}
      {selectedResource === 'node' && (
        <Nodes nodes={agents as Node[]} {...listProps} />
      )}
      {selectedResource === 'windows_desktop' && (
        <Desktops desktops={agents as Desktop[]} {...listProps} />
      )}
      {selectedResource === 'kube_cluster' && (
        <Kubes kubes={agents as Kube[]} {...listProps} />
      )}
      {selectedResource === 'role' && (
        <Roles roles={requestableRoles} {...listProps} />
      )}
      {selectedResource === 'user_group' && (
        <UserGroups userGroups={agents as UserGroup[]} {...listProps} />
      )}
    </Wrapper>
  );
}

export const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: middle;
  }
` as typeof Table;

const Wrapper = styled(Box)`
  &.disabled {
    pointer-events: none;
    opacity: 0.5;
  }
`;

export function renderActionCell(
  isAgentAdded: boolean,
  toggleAgent: () => void
) {
  return (
    <Cell align="right">
      {isAgentAdded ? (
        <ButtonPrimary width="134px" size="small" onClick={toggleAgent}>
          Remove
        </ButtonPrimary>
      ) : (
        <ButtonBorder width="134px" size="small" onClick={toggleAgent}>
          + Add to request
        </ButtonBorder>
      )}
    </Cell>
  );
}

export type ListProps = {
  customSort: CustomSort;
  onLabelClick: (label: ResourceLabel) => void;
  addedResources: ResourceMap;
  addOrRemoveResource: (
    kind: ResourceKind,
    resourceId: string,
    resourceName?: string
  ) => void;
  requestableRoles?: string[];
};

export type ResourceListProps = {
  agents: UnifiedResource[];
  selectedResource: ResourceKind;
  // disableRows disable clicking on any buttons (when fetching).
  disableRows: boolean;
} & ListProps;
