import React, { useEffect, useState } from 'react';
import { components, MenuListProps } from 'react-select';
import styled from 'styled-components';
import { ButtonBorder, ButtonPrimary, Flex, Text } from 'design';
import Select from 'shared/components/Select';
import { HoverTooltip } from 'design/Tooltip';
import { App, AppSubKind, PermissionSet } from 'teleport/services/apps';

import { ResourceMap } from 'shared/components/AccessRequests/NewRequest';

import {
  CheckableOptionComponent,
  Option,
} from 'shared/components/AccessRequests/NewRequest/CheckableOption';

import { requestItems } from 'e-teleport/Workflow/NewRequest/useNewRequest';

import type { RequestItem } from 'e-teleport/Workflow/NewRequest/useNewRequest';

function getButtonText(addText: string, requestStarted: boolean): string {
  if (addText) {
    return addText;
  }
  if (requestStarted) {
    return '+ Add to request';
  }

  return '+ Request Access';
}

export const RequestButton = ({
  isAgentAdded,
  onClick,
  removeText,
  addText,
  disabled,
  requestStarted = false,
}: {
  // addText is an optional parameter that will be displayed when an agent is not added
  addText?: string;
  // removeText is an optional parameter that will be displayed when an added is added
  removeText?: string;
  isAgentAdded: boolean;
  onClick: React.MouseEventHandler<HTMLButtonElement>;
  disabled: boolean;
  requestStarted?: boolean;
}) => {
  if (isAgentAdded) {
    return (
      <ButtonPrimary
        textTransform="none"
        disabled={disabled}
        width="123px"
        size="small"
        onClick={onClick}
      >
        {removeText || 'Remove'}
      </ButtonPrimary>
    );
  }
  return (
    <ButtonBorder
      textTransform="none"
      disabled={disabled}
      onClick={onClick}
      width="123px"
      size="small"
    >
      {getButtonText(addText, requestStarted)}
    </ButtonBorder>
  );
};

export function AppRequestButton({
  agent,
  addedResources,
  disabled = false,
  addOrRemoveResources,
  addText,
  requestStarted,
}: {
  disabled?: boolean;
  agent: App;
  addedResources: ResourceMap;
  addText?: string;
  requestStarted?: boolean;
  addOrRemoveResources: (
    items: RequestItem[],
    action?: 'add' | 'remove'
  ) => void;
}) {
  if (agent.subKind == AppSubKind.AwsIcAccount) {
    return (
      <IdentityCenterRequestButton
        agent={agent}
        addedResources={addedResources}
        addOrRemoveResources={addOrRemoveResources}
      />
    );
  }

  const selectedUserGroup =
    Object.keys(addedResources.user_group).length > 0
      ? Object.keys(addedResources.user_group)[0]
      : null;

  const isAppAdded =
    Boolean(addedResources.app[agent.name]) ||
    Boolean(addedResources.saml_idp_service_provider[agent.name]);
  const resourceKind = agent.samlApp ? 'saml_idp_service_provider' : 'app';
  if (agent.userGroups.length === 0) {
    return (
      <RequestButton
        isAgentAdded={isAppAdded}
        onClick={() =>
          addOrRemoveResources([
            {
              kind: resourceKind,
              resourceId: agent.name,
              resourceName: agent.friendlyName,
            },
          ])
        }
        addText={addText}
        disabled={disabled}
        requestStarted={requestStarted}
      />
    );
  }

  const isUserGroupAdded =
    agent.userGroups.length > 0 &&
    agent.userGroups.some(userGroup =>
      Boolean(addedResources.user_group[userGroup.name])
    );

  const options: Option[] = agent.userGroups.map(user_group => ({
    label: user_group.description,
    value: user_group.name,
    isAdded: Boolean(addedResources.user_group[user_group.name]),
    kind: 'user_group',
  }));

  function handleSelect(option: Option) {
    if (selectedUserGroup !== null && selectedUserGroup !== option.value) {
      addOrRemoveResources(requestItems('user_group', selectedUserGroup));
      addOrRemoveResources(
        requestItems('user_group', option.value, option.label)
      );
    } else {
      addOrRemoveResources(
        requestItems('user_group', option.value, option.label)
      );
    }
  }

  return (
    <Flex alignItems="center" justifyContent="end">
      <StyledSelect
        size="small"
        className={isUserGroupAdded ? 'optionsSelected' : ''}
        placeholder={isUserGroupAdded ? 'Edit App Role' : 'Select App Role'}
        value={null}
        options={options}
        isSearchable={false}
        isClearable={false}
        isMulti={false}
        hideSelectedOptions={false}
        controlShouldRenderValue={false}
        closeMenuOnSelect={false}
        onChange={handleSelect}
        components={{
          Option: CheckableOptionComponent,
        }}
      />
    </Flex>
  );
}

const StyledSelect = styled(Select)`
  input[type='checkbox'] {
    cursor: pointer;
  }

  .react-select__control {
    ${props => props.theme.typography.body4}
    width: 123px;
    min-height: 24px;
  }

  .react-select__menu {
    width: 230px;
    right: 0;
  }

  .react-select__option {
    ${props => props.theme.typography.body3}
    padding: 0;
  }

  .react-select__value-container {
    position: static;
  }

  .react-select__dropdown-indicator {
    padding: 2px;
    height: 24px;
  }

  .react-select__placeholder {
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
  }

  &.optionsSelected {
    .react-select__control {
      background: ${p => p.theme.colors.interactive.solid.primary.default};
      border: transparent;
    }

    .react-select__placeholder,
    .react-select__dropdown-indicator {
      color: ${p => p.theme.colors.text.primaryInverse};
    }
  }
`;

type AccountAssignmentOption = Option & {
  friendlyName: string;
};

/**
 * IdentityCenterRequestButton returns select button that renders
 * requestable permission set(s) for the given Identity Center
 * account.
 */
export function IdentityCenterRequestButton({
  agent,
  addedResources,
  addOrRemoveResources,
}: {
  agent: App;
  addedResources: ResourceMap;
  addOrRemoveResources: (
    items: RequestItem[],
    action?: 'add' | 'remove'
  ) => void;
}) {
  const options = agent.permissionSets
    .map(ps => makeAssignmentOption(ps, addedResources, agent.name))
    .sort((a, b) => (a.label < b.label ? -1 : 1));

  const handleSelect = (options: AccountAssignmentOption[]) => {
    options.forEach(option => {
      addOrRemoveResources(
        requestItems(
          'aws_ic_account_assignment',
          option.value,
          option.friendlyName
        )
      );
    });
  };

  const hasAddedPS = options.some(aaOpt =>
    addedResources.aws_ic_account_assignment
      ? Boolean(addedResources.aws_ic_account_assignment[aaOpt.value])
      : false
  );

  const [checked, setChecked] = useState(false);
  function handleCheck() {
    const req: RequestItem[] = [];
    options.forEach(option => {
      req.push({
        kind: 'aws_ic_account_assignment',
        resourceId: option.value,
        resourceName: option.friendlyName,
      });
    });
    addOrRemoveResources(req, checked ? 'remove' : 'add');
  }

  useEffect(() => {
    const allAdded = options.every(
      ({ value }) => addedResources.aws_ic_account_assignment[value]
    );
    setChecked(allAdded);
  }, [options, addedResources]);

  return (
    <HoverTooltip tipContent="Select Permission Set(s)">
      <StyledSelect
        size="small"
        placeholder="Select Permission Set(s)"
        className={hasAddedPS ? 'optionsSelected' : ''}
        value={null}
        options={options}
        isSearchable={false}
        isClearable={false}
        isMulti={true}
        hideSelectedOptions={false}
        controlShouldRenderValue={true}
        closeMenuOnSelect={false}
        onChange={handleSelect}
        components={{
          Option: CheckableOptionComponent,
          MenuList: (props: MenuListProps) => (
            <MenuList
              props={props}
              checked={checked}
              handleCheck={handleCheck}
            />
          ),
        }}
      />
    </HoverTooltip>
  );
}

function makeAssignmentOption(
  ps: PermissionSet,
  addedResources: ResourceMap,
  accountName: string
) {
  const resourceName = ps.assignmentId;
  return {
    label: ps.name,
    value: resourceName,
    disabled: false,
    isAdded: Boolean(addedResources.aws_ic_account_assignment[resourceName]),
    friendlyName: `"${ps.name}" on "${accountName}"`,
  };
}

const MenuList = ({
  props,
  checked,
  handleCheck,
}: {
  props: MenuListProps;
  checked: boolean;
  handleCheck: () => void;
}) => {
  return (
    <components.MenuList {...props}>
      <Flex alignItems="center" py="8px" px="12px">
        <input type="checkbox" checked={checked} onChange={handleCheck} />{' '}
        <Text ml={1} bold>
          Select All
        </Text>
      </Flex>
      {props.children}
    </components.MenuList>
  );
};
