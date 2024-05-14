import React from 'react';
import { ButtonPrimary, ButtonBorder, Flex, Text } from 'design';
import styled from 'styled-components';
import { StyledSelect as BaseStyledSelect } from 'shared/components/Select/Select';
import { components } from 'react-select';
import Select, { Option as BaseOption } from 'shared/components/Select';
import { App } from 'teleport/services/apps';

import {
  ResourceKind,
  ResourceMap,
} from 'shared/components/AccessRequests/NewRequest';

type Option = BaseOption & {
  isAdded?: boolean;
  kind: 'app' | 'user_group';
};

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

const OptionComponent = (props: { data: Option }) => {
  const { data } = props;
  return (
    <components.Option {...props}>
      <Flex alignItems="center" py="8px" px="12px">
        <input
          type="checkbox"
          checked={data.isAdded}
          readOnly
          name={data.value}
          id={data.value}
        />{' '}
        <Text ml={1}>{data.label}</Text>
      </Flex>
    </components.Option>
  );
};

export function AppRequestButton({
  agent,
  addedResources,
  disabled = false,
  addOrRemoveResource,
  addText,
  requestStarted,
}: {
  disabled?: boolean;
  agent: App;
  addedResources: ResourceMap;
  addText?: string;
  requestStarted?: boolean;
  addOrRemoveResource: (
    kind: ResourceKind,
    resourceId: string,
    resourceName?: string
  ) => void;
}) {
  const selectedUserGroup =
    Object.keys(addedResources.user_group).length > 0
      ? Object.keys(addedResources.user_group)[0]
      : null;

  const isAppAdded = Boolean(addedResources.app[agent.name]);

  if (agent.userGroups.length === 0) {
    return (
      <RequestButton
        isAgentAdded={isAppAdded}
        onClick={() =>
          addOrRemoveResource('app', agent.name, agent.friendlyName)
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

  const options = agent.userGroups.map(user_group => ({
    label: user_group.description,
    value: user_group.name,
    isAdded: Boolean(addedResources.user_group[user_group.name]),
    kind: 'user_group',
  }));

  function handleSelect(option: Option) {
    if (selectedUserGroup !== null && selectedUserGroup !== option.value) {
      addOrRemoveResource('user_group', selectedUserGroup);
      addOrRemoveResource('user_group', option.value, option.label);
    } else {
      addOrRemoveResource('user_group', option.value, option.label);
    }
  }

  return (
    <Flex alignItems="center" justifyContent="end">
      <StyledSelect className={isUserGroupAdded ? 'hasSelectedGroups' : ''}>
        <Select
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
            Option: OptionComponent,
          }}
        />
      </StyledSelect>
    </Flex>
  );
}

const StyledSelect = styled(BaseStyledSelect)`
  input[type='checkbox'] {
    cursor: pointer;
  }

  .react-select__control {
    font-size: 10px;
    width: 123px;
    height: 26px;
    min-height: 24px;
    border: 2px solid ${p => p.theme.colors.buttons.secondary.default};
  }

  .react-select__menu {
    font-size: 12px;
    width: 230px;
    right: 0;
    color: ${p => p.theme.colors.primary};
  }

  .react-select__option {
    padding: 0;
    font-size: 12px;
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
