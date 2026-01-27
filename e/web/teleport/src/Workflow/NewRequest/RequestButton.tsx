import React, { useEffect, useRef, useState } from 'react';
import { components, MenuListProps } from 'react-select';
import styled from 'styled-components';

import { Box, ButtonBorder, ButtonPrimary, Flex, Menu, Text } from 'design';
import { CheckboxInput } from 'design/Checkbox';
import { ChevronDown } from 'design/Icon';
import { MenuItem } from 'design/Menu';
import { HoverTooltip } from 'design/Tooltip';
import { ResourceMap } from 'shared/components/AccessRequests/NewRequest';
import {
  CheckableOptionComponent,
  Option,
} from 'shared/components/AccessRequests/NewRequest/CheckableOption';
import Select from 'shared/components/Select';
import {
  getResourceIDString,
  ResourceConstraints,
  ResourceConstraintsMap,
  ResourceIDString,
} from 'shared/services/accessRequests';
import { AwsRole } from 'shared/services/apps';
import { ComponentFeatureID } from 'shared/utils/componentFeatures';

import {
  AWSLoginChoice,
  AWSRoleToLoginChoice,
} from 'e-teleport/Workflow/NewRequest/aws';
import {
  requestItems,
  type RequestItem,
} from 'e-teleport/Workflow/NewRequest/useNewRequest';
import { UnifiedResource } from 'teleport/services/agents';
import { App, AppSubKind, PermissionSet } from 'teleport/services/apps';

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

type AwsConsoleApp = App & { awsConsole: true; awsRoles?: AwsRole[] };

const isApp = (resource: UnifiedResource): resource is App =>
  resource.kind === 'app';
const isAwsConsoleApp = (app: App): app is AwsConsoleApp => !!app.awsConsole;
const supportsResourceConstraints = (app: AwsConsoleApp) =>
  app.supportedFeatureIds?.includes?.(
    ComponentFeatureID.ResourceConstraintsV1
  ) || false;

/**
 * resourceIsAwsAndSupportsConstraints returns whether the given resource is
 * an AWS Console app that supports requesting/specifying IAM roles.
 */
export const resourceIsAWSConsoleAndSupportsConstraints = (
  resource: UnifiedResource
): resource is AwsConsoleApp => {
  // Must be 'app'
  if (!isApp(resource)) {
    return false;
  }
  // Must be AWS Console
  if (!isAwsConsoleApp(resource)) {
    return false;
  }
  // Must support ResourceConstraints
  return supportsResourceConstraints(resource);
};

type AppAWSRoleMenuProps = {
  agent: AwsConsoleApp;
  addedResources: ResourceMap;
  requestStarted?: boolean;
  isNewRequestFlow?: boolean;
  addOrRemoveResources: (
    items: RequestItem[],
    action?: 'add' | 'remove'
  ) => void;
  addedResourceConstraints: ResourceConstraintsMap;
  setResourceConstraints: (
    key: ResourceIDString,
    rc?: ResourceConstraints
  ) => void;
  width?: string;
};

/**
 * AppAWSRoleMenu allows selecting/requesting AWS IAM Roles for an AWS Console app.
 */
export const AppAWSRoleMenu = ({
  agent,
  addedResources,
  addedResourceConstraints,
  addOrRemoveResources,
  setResourceConstraints,
  requestStarted = false,
  isNewRequestFlow = false,
  width = '123px',
}: AppAWSRoleMenuProps) => {
  const anchorEl = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);

  const { granted, requestable } = (agent.awsRoles || [])
    .map(AWSRoleToLoginChoice(agent))
    .reduce(
      (acc, role) => {
        // If in new request flow, all present roles are requestable
        // and will not have 'requiresRequest' property.
        const target =
          role.requiresRequest || isNewRequestFlow
            ? acc.requestable
            : acc.granted;
        target.push(role);
        return acc;
      },
      { granted: [] as AWSLoginChoice[], requestable: [] as AWSLoginChoice[] }
    );

  const requestStartedOrNoGranted = requestStarted || !granted.length;

  // Cart key for apps is just the app name; for constraints map we use
  // clusterName and kind to ensure uniqueness.
  const key = getResourceIDString({
    cluster: agent.clusterId,
    kind: agent.kind,
    name: agent.name,
  });
  const isAppInCart = !!addedResources.app[agent.name];
  const selectedARNs =
    addedResourceConstraints[key]?.aws_console?.role_arns ?? [];

  const isChecked = (choice: AWSLoginChoice) =>
    selectedARNs.includes(choice.id);

  const toggleRequestable = (choice: AWSLoginChoice) => {
    const next = isChecked(choice)
      ? selectedARNs.filter(arn => arn !== choice.id)
      : [...selectedARNs, choice.id];
    const rc = (
      next.length
        ? {
            aws_console: { role_arns: next },
          }
        : undefined
    ) satisfies ResourceConstraints;

    // Add/remove agent from cart if needed
    if (isAppInCart !== !!next.length) {
      addOrRemoveResources(requestItems('app', agent.name, agent.friendlyName));
    }
    setResourceConstraints(key, rc);
  };

  // If < one login is available and none requestable, show normal 'Connect' button.
  if (granted.length <= 1 && !requestable.length) {
    return (
      <HoverTooltip
        tipContent={!granted.length ? 'No available logins' : undefined}
      >
        <ButtonBorder
          as="a"
          textTransform="none"
          width={width}
          size="small"
          href={granted[0]?.launchUrl}
          target="_blank"
          rel="noreferrer"
          disabled={!granted.length}
        >
          Connect
        </ButtonBorder>
      </HoverTooltip>
    );
  }

  return (
    <>
      {/* TODO(kiosion): Should be ButtonPrimary when in cart; need to fix wrapper overriding styles and making text unintelligible */}
      <ButtonBorder
        textTransform="none"
        width={width}
        size="small"
        ref={el => (anchorEl.current = el!)}
        onClick={() => {
          setOpen(true);
        }}
      >
        {isNewRequestFlow
          ? 'Add to request'
          : requestStartedOrNoGranted
            ? 'Request Access'
            : 'Connect'}
        <ChevronDown ml={1} mr={-2} size="small" color="text.slightlyMuted" />
      </ButtonBorder>

      <Menu
        popoverCss={() => ({
          marginTop: '4px',
        })}
        menuListCss={p => ({
          minWidth: '220px',
          maxHeight: '280px',
          overflowY: 'auto',
          overflowX: 'clip',
          scrollbarWidth: 'thin',
          scrollbarGutter: 'stable',
          scrollbarColor: `${p.theme.colors.spotBackground[2]} transparent`,
        })}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        getContentAnchorEl={null}
        anchorEl={anchorEl.current}
        open={open}
        onClose={() => setOpen(false)}
      >
        {/* Hide 'connect' section when in request mode */}
        {!requestStartedOrNoGranted && (
          <>
            {!!requestable.length && <SectionHeader>Connect:</SectionHeader>}
            <Box>
              {granted.map(item => (
                <StyledMenuItem
                  as="a"
                  key={`g:${item.id}`}
                  px={2}
                  mx={2}
                  href={item.launchUrl}
                  target="_blank"
                  title={item.label}
                  onClick={() => setOpen(false)}
                >
                  <Text>{item.label}</Text>
                </StyledMenuItem>
              ))}
            </Box>
          </>
        )}
        {!!requestable.length && (
          <>
            {!requestStartedOrNoGranted && (
              <SectionHeader>Request Access:</SectionHeader>
            )}
            <Box>
              {requestable.map(item => (
                <StyledMenuItem
                  as="div"
                  key={`r:${item.id}`}
                  title={item.label}
                  onClick={() => toggleRequestable(item)}
                >
                  <CheckboxInput
                    type="checkbox"
                    checked={isChecked(item)}
                    onChange={() => toggleRequestable(item)}
                  />
                  <Text>{item.label}</Text>
                </StyledMenuItem>
              ))}
            </Box>
          </>
        )}
      </Menu>
    </>
  );
};

const SectionHeader = styled(Text)`
  ${({ theme }) => theme.typography.body3};
  font-weight: 500;
  color: ${({ theme }) => theme.colors.text.muted};
  padding: 0 ${({ theme }) => theme.space[3]}px;
  pointer-events: none;

  &:first-child {
    margin-top: ${({ theme }) => theme.space[2]}px;
  }
`;

const StyledMenuItem = styled(MenuItem)`
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: flex-start;
  gap: ${({ theme }) => theme.space[2]}px;
  min-height: 32px;
  margin: 0;
  padding: ${({ theme }) => theme.space[2]}px ${({ theme }) => theme.space[3]}px;
  user-select: none;

  &:hover {
    background: ${({ theme }) => theme.colors.spotBackground[0]};
    color: ${({ theme }) => theme.colors.text.main};
  }

  &:first-child {
    margin-top: ${({ theme }) => theme.space[1]}px;
  }

  &:last-child {
    margin-bottom: ${({ theme }) => theme.space[1]}px;
  }
`;
