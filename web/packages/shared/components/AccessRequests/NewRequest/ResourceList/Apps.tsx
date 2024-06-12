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

import React, { useState, useEffect } from 'react';
import styled from 'styled-components';
import { components } from 'react-select';
import { Flex, Text, ButtonBorder, ButtonPrimary } from 'design';
import { ClickableLabelCell, Cell } from 'design/DataTable';

import { App } from 'teleport/services/apps';

import Select, {
  Option as BaseOption,
  CustomSelectComponentProps,
} from 'shared/components/Select';
import { StyledSelect as BaseStyledSelect } from 'shared/components/Select/Select';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { ResourceMap, ResourceKind } from '../resource';

import { ListProps, StyledTable } from './ResourceList';

type Option = BaseOption & {
  isSelected?: boolean;
};

export function Apps(props: ListProps & { apps: App[] }) {
  const {
    apps = [],
    addedResources,
    customSort,
    onLabelClick,
    addOrRemoveResource,
  } = props;
  return (
    <StyledTable
      data={apps}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'description',
          headerText: 'Description',
          isSortable: true,
        },
        {
          key: 'addrWithProtocol',
          headerText: 'Address',
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: ({ labels }) => (
            <ClickableLabelCell labels={labels} onClick={onLabelClick} />
          ),
        },
        {
          altKey: 'action-btn',
          render: agent => (
            <ActionCell
              agent={agent}
              addedResources={addedResources}
              addOrRemoveResource={addOrRemoveResource}
            />
          ),
        },
      ]}
      emptyText="No Results Found"
      customSort={customSort}
      disableFilter
    />
  );
}

const OptionComponent = (
  props: CustomSelectComponentProps<
    { toggleUserGroup(groupId: string, groupDescription: string): void },
    Option
  >
) => {
  const { toggleUserGroup } = props.selectProps.customProps;
  return (
    <components.Option {...props} className="react-select__selected">
      <Flex
        alignItems="center"
        onClick={() => toggleUserGroup(props.value, props.label)}
        py="8px"
        px="12px"
      >
        <input
          type="checkbox"
          checked={props.isSelected}
          readOnly
          name={props.value}
          id={props.value}
        />{' '}
        <Text ml={1}>{props.label}</Text>
      </Flex>
    </components.Option>
  );
};

function ActionCell({
  agent,
  addedResources,
  addOrRemoveResource,
}: {
  agent: App;
  addedResources: ResourceMap;
  addOrRemoveResource: (
    kind: ResourceKind,
    resourceId: string,
    resourceName?: string
  ) => void;
}) {
  const [userGroupOptions] = useState<Option[]>(() => {
    return agent.userGroups.map(ug => {
      return { label: ug.description, value: ug.name };
    });
  });
  const [selectedGroups, setSelectedGroups] = useState<Option[]>([]);

  useEffect(() => {
    if (userGroupOptions.length === 0) {
      return;
    }

    // Applications can refer to the same user group id.
    // When user selects an option from one row, we need
    // to update selected groups for all other rows.
    const updatedSelectedGroups = userGroupOptions.flatMap(o => {
      if (addedResources.user_group[o.value]) {
        return { ...o, isSelected: true };
      }
      return []; // skip this option
    });
    setSelectedGroups(updatedSelectedGroups);

    // A user can only select an app OR user groups.
    // If a user selected a user group from one row,
    // that is also applicable to this row,
    // remove app from selection.
    if (addedResources.app[agent.name] && updatedSelectedGroups.length > 0) {
      addOrRemoveResource('app', agent.name);
    }
  }, [addedResources]);

  function handleSelectedGroups(o: Option[]) {
    // Deselect the app if a user is selecting from a list of groups
    // for the first time.
    if (selectedGroups.length === 0 && addedResources.app[agent.name]) {
      addOrRemoveResource('app', agent.name); // remove app from selection.
    }

    setSelectedGroups(o);
  }

  function toggleUserGroup(id: string, description = '') {
    addOrRemoveResource('user_group', id, description);
  }

  function toggleApp() {
    addOrRemoveResource('app', agent.name, agent.friendlyName);
  }

  const isAppAdded = Boolean(addedResources.app[agent.name]);
  const hasSelectedGroups = selectedGroups.length > 0;

  if (!isAppAdded && !hasSelectedGroups) {
    return (
      <Cell align="right">
        <ButtonBorder width="134px" size="small" onClick={toggleApp}>
          + Add to request
        </ButtonBorder>
      </Cell>
    );
  }

  if (isAppAdded && agent.userGroups.length === 0) {
    return (
      <Cell align="right">
        <ButtonPrimary width="134px" size="small" onClick={toggleApp}>
          Remove
        </ButtonPrimary>
      </Cell>
    );
  }

  // Remove button is only shown when user has not added user
  // groups yet, but has the option to do so
  const showRemoveButton = isAppAdded && !hasSelectedGroups;

  return (
    <Cell align="right">
      {showRemoveButton && (
        <ButtonPrimary width="134px" size="small" onClick={toggleApp}>
          Remove
        </ButtonPrimary>
      )}
      <Flex alignItems="center" justifyContent="end">
        <ToolTipInfo muteIconColor={true}>
          This application {agent.name} can be alternatively requested by
          members of user groups. You can alternatively select user groups
          instead to access this application.
        </ToolTipInfo>
        <StyledSelect className={hasSelectedGroups ? 'hasSelectedGroups' : ''}>
          <Select
            placeholder={
              hasSelectedGroups
                ? `${selectedGroups.length} User Groups Added`
                : 'Alternatively Select User Groups'
            }
            value={selectedGroups}
            options={userGroupOptions}
            isSearchable={false}
            isClearable={false}
            isMulti={true}
            hideSelectedOptions={false}
            controlShouldRenderValue={false}
            onChange={handleSelectedGroups}
            components={{
              Option: OptionComponent,
            }}
            customProps={{ toggleUserGroup }}
          />
        </StyledSelect>
      </Flex>
    </Cell>
  );
}

const StyledSelect = styled(BaseStyledSelect)`
  margin-left: 8px;
  input[type='checkbox'] {
    cursor: pointer;
  }

  .react-select__control {
    font-size: 12px;
    width: 225px;
    height: 26px;
    min-height: 24px;
    border: 2px solid ${p => p.theme.colors.buttons.secondary.default};
  }

  .react-select__menu {
    font-size: 12px;
    width: 275px;
    right: 0;
  }

  .react-select__option {
    padding: 0;
  }

  .react-select__value-container {
    position: static;
  }

  .react-select__dropdown-indicator {
    padding-top: 0px;
  }

  &.hasSelectedGroups {
    .react-select-container {
      background: ${p => p.theme.colors.buttons.primary.default};
    }
    .react-select__placeholder,
    .react-select__dropdown-indicator {
      color: ${p => p.theme.colors.buttons.primary.text};
    }
  }
`;
