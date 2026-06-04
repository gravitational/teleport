import React, { useEffect, useMemo, useRef, useState } from 'react';
import { components, MenuListProps } from 'react-select';
import styled, { css } from 'styled-components';

import {
  Box,
  Button,
  ButtonBorder,
  ButtonPrimary,
  Flex,
  Menu,
  Text,
} from 'design';
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

import { AWSRoleToLoginChoice } from 'e-teleport/Workflow/NewRequest/aws';
import {
  type RequestItem,
  requestItems,
} from 'e-teleport/Workflow/NewRequest/useNewRequest';
import cfg from 'teleport/config';
import { UnifiedResource } from 'teleport/services/agents';
import { App, AppSubKind, PermissionSet } from 'teleport/services/apps';
import { Node, SshLogin } from 'teleport/services/nodes';

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

/**
 * MenuChoice is a generic item displayed in a ConstraintMenu dropdown.
 */
type MenuChoice = {
  id: string;
  label: string;
  requiresRequest: boolean;
  connectUrl?: string;
};

const constraintMenuPopoverCss = () => css`
  margin-top: 4px;
`;
const constraintMenuMenuListCss = () => css`
  min-width: 180px;
  max-height: 340px;
  max-width: 320px;
  overflow-y: auto;
  overflow-x: clip;
  scrollbar-width: thin;
  scrollbar-color: ${p => p.theme.colors.spotBackground[2]} transparent;
`;
const constraintMenuTransformOrigin = {
  vertical: 'top',
  horizontal: 'right',
} as const;
const constraintMenuAnchorOrigin = {
  vertical: 'bottom',
  horizontal: 'right',
} as const;

type ConstraintMenuProps = {
  choices: MenuChoice[];
  selectedIds: string[];
  onToggle: (choices: MenuChoice[]) => void;
  noPrincipalsText?: string;
  connectText?: string;
  searchPlaceholder?: string;
  requestStarted?: boolean;
  isNewRequestFlow?: boolean;
  isInCart?: boolean;
  width?: string;
};

/**
 * ConstraintMenu renders a dropdown button that splits choices into
 * "Connect" (granted) and "Request Access" (requestable) sections.
 */
const ConstraintMenu = ({
  choices,
  selectedIds,
  onToggle,
  noPrincipalsText = 'No principals found',
  connectText = 'Connect',
  searchPlaceholder = 'Search...',
  requestStarted = false,
  isNewRequestFlow = false,
  isInCart = false,
  width = '123px',
}: ConstraintMenuProps) => {
  const anchorEl = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');

  const { granted, requestable } = useMemo(
    () =>
      choices.reduce<{ granted: MenuChoice[]; requestable: MenuChoice[] }>(
        (acc, choice) => {
          const target =
            choice.requiresRequest || isNewRequestFlow
              ? acc.requestable
              : acc.granted;
          target.push(choice);
          return acc;
        },
        { granted: [], requestable: [] }
      ),
    [choices, isNewRequestFlow]
  );

  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);
  const isChecked = (choice: MenuChoice) => selectedSet.has(choice.id);

  const requestStartedOrNoGranted = requestStarted || granted.length === 0;
  const showSearch = granted.length + requestable.length > 2;
  const isFilled = isInCart || open || granted.length > 0;

  const trimmedSearch = search.trim().toLowerCase();
  const matches = (item: MenuChoice) =>
    !trimmedSearch || item.label.toLowerCase().includes(trimmedSearch);

  const visibleRequestable = requestable.filter(matches);
  const allVisibleChecked =
    visibleRequestable.length > 0 && visibleRequestable.every(isChecked);

  const requestableHidden = visibleRequestable.length === 0;
  const hasVisibleGranted = granted.some(matches);

  if (granted.length === 0 && requestable.length === 0) {
    return (
      <HoverTooltip tipContent={noPrincipalsText}>
        <Box>
          <Button
            textTransform="none"
            width={width}
            size="small"
            intent="neutral"
            aria-disabled="true"
            disabled
          >
            {connectText}
          </Button>
        </Box>
      </HoverTooltip>
    );
  }

  return (
    <div>
      <StyledButton
        fill={isFilled ? 'filled' : 'border'}
        intent={isInCart ? 'primary' : 'neutral'}
        textTransform="none"
        width={width}
        size="small"
        ref={el => (anchorEl.current = el!)}
        onClick={() => setOpen(true)}
      >
        {isNewRequestFlow
          ? 'Add to request'
          : requestStartedOrNoGranted
            ? 'Request Access'
            : connectText}
        <ChevronDown
          ml={1}
          size="small"
          color={isInCart ? 'text.primaryInverse' : 'text.slightlyMuted'}
        />
      </StyledButton>

      <Menu
        popoverCss={constraintMenuPopoverCss}
        menuListCss={constraintMenuMenuListCss}
        transformOrigin={constraintMenuTransformOrigin}
        anchorOrigin={constraintMenuAnchorOrigin}
        getContentAnchorEl={null}
        anchorEl={anchorEl.current}
        open={open}
        onClose={() => setOpen(false)}
      >
        {showSearch && (
          <StyledMenuSearchWrapper>
            <StyledMenuSearch
              type="text"
              name="notsearch_password"
              autoComplete="off"
              autoFocus
              value={search}
              placeholder={searchPlaceholder}
              onChange={e => setSearch(e.currentTarget.value)}
            />
          </StyledMenuSearchWrapper>
        )}
        {granted.length > 0 && (
          <>
            <SectionHeader
              aria-hidden={requestable.length === 0 || !hasVisibleGranted}
            >
              Connect:
            </SectionHeader>
            <StyledMenuSection aria-hidden={!hasVisibleGranted}>
              {granted.map(item => (
                <StyledMenuItem
                  as="a"
                  key={`g:${item.id}`}
                  px={2}
                  mx={2}
                  href={requestStarted ? undefined : item.connectUrl}
                  target="_blank"
                  title={item.label}
                  onClick={() => !requestStarted && setOpen(false)}
                  disabled={requestStarted}
                  aria-disabled={requestStarted}
                  aria-hidden={!matches(item)}
                >
                  <Text>{item.label}</Text>
                </StyledMenuItem>
              ))}
            </StyledMenuSection>
          </>
        )}
        {requestable.length > 0 && (
          <>
            <SectionHeader
              aria-hidden={granted.length === 0 || requestableHidden}
            >
              Request Access:
            </SectionHeader>
            <StyledMenuSection aria-hidden={requestableHidden}>
              <StyledMenuItem
                as="label"
                aria-hidden={visibleRequestable.length < 2}
              >
                <CheckboxInput
                  type="checkbox"
                  checked={allVisibleChecked}
                  onChange={() => onToggle(visibleRequestable)}
                />
                <Text color="text.slightlyMuted">Select All</Text>
              </StyledMenuItem>
              {requestable.map(item => (
                <StyledMenuItem
                  as="label"
                  key={`r:${item.id}`}
                  title={item.label}
                  aria-hidden={!matches(item)}
                >
                  <CheckboxInput
                    type="checkbox"
                    checked={isChecked(item)}
                    onChange={() => onToggle([item])}
                  />
                  <Text>{item.label}</Text>
                </StyledMenuItem>
              ))}
            </StyledMenuSection>
          </>
        )}
        <SectionHeader
          pb={2}
          aria-hidden={hasVisibleGranted || !requestableHidden}
        >
          {noPrincipalsText}
        </SectionHeader>
      </Menu>
    </div>
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
): resource is AwsConsoleApp =>
  isApp(resource) &&
  isAwsConsoleApp(resource) &&
  supportsResourceConstraints(resource);

/**
 * Toggles the given choices in/out of the current selection.
 * If all choices are already selected, they're removed; otherwise they're added.
 */
function toggleSelections(
  currentIds: string[],
  choices: MenuChoice[]
): string[] {
  const choiceIds = new Set(choices.map(c => c.id));
  const allSelected = choices.every(c => currentIds.includes(c.id));
  return allSelected
    ? currentIds.filter(id => !choiceIds.has(id))
    : Array.from(new Set([...currentIds, ...choiceIds]));
}

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
 * AppAwsRoleMenu allows selecting/requesting AWS IAM Roles for an AWS Console app.
 */
export const AppAwsRoleMenu = ({
  agent,
  addedResources,
  addedResourceConstraints,
  addOrRemoveResources,
  setResourceConstraints,
  requestStarted,
  isNewRequestFlow,
  width = '133px',
}: AppAWSRoleMenuProps) => {
  const choices: MenuChoice[] = sortAwsRoles(agent.awsRoles || [])
    .map(AWSRoleToLoginChoice(agent))
    .map(r => ({
      id: r.id,
      label: r.label,
      requiresRequest: r.requiresRequest,
      connectUrl: r.launchUrl,
    }));

  const key = getResourceIDString({
    cluster: agent.clusterId,
    kind: agent.kind,
    name: agent.name,
  });
  const isInCart = !!addedResources.app[agent.name];
  const selectedARNs =
    addedResourceConstraints[key]?.aws_console?.role_arns ?? [];

  const handleToggle = (choices: MenuChoice[]) => {
    const next = toggleSelections(selectedARNs, choices);

    const rc = (
      next.length ? { aws_console: { role_arns: next } } : undefined
    ) satisfies ResourceConstraints;

    if (isInCart !== !!next.length) {
      addOrRemoveResources(requestItems('app', agent.name, agent.friendlyName));
    }
    setResourceConstraints(key, rc);
  };

  return (
    <ConstraintMenu
      choices={choices}
      selectedIds={selectedARNs}
      onToggle={handleToggle}
      requestStarted={requestStarted}
      isNewRequestFlow={isNewRequestFlow}
      isInCart={isInCart}
      width={width}
      connectText="Launch"
      noPrincipalsText="No IAM roles found"
      searchPlaceholder="Search IAM roles..."
    />
  );
};

type NodeWithLoginDetails = Node & { sshLoginDetails: SshLogin[] };

const isNode = (resource: UnifiedResource): resource is Node =>
  resource.kind === 'node';
const isNodeWithLoginDetails = (node: Node): node is NodeWithLoginDetails =>
  !!node.sshLoginDetails?.length;
const nodeSupportsResourceConstraints = (node: NodeWithLoginDetails) =>
  node.supportedFeatureIds?.includes(
    ComponentFeatureID.ResourceConstraintsV1
  ) || false;

/**
 * resourceIsNodeAndSupportsConstraints returns whether the given resource is
 * an SSH node that supports requesting/specifying logins via constraints.
 */
export const resourceIsNodeAndSupportsConstraints = (
  resource: UnifiedResource
): resource is NodeWithLoginDetails =>
  isNode(resource) &&
  isNodeWithLoginDetails(resource) &&
  nodeSupportsResourceConstraints(resource);

type NodeSshLoginMenuProps = {
  agent: NodeWithLoginDetails;
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
  clusterId: string;
  width?: string;
};

/**
 * NodeSshLoginMenu allows selecting/requesting SSH logins for a node.
 */
export const NodeSshLoginMenu = ({
  agent,
  addedResources,
  addedResourceConstraints,
  addOrRemoveResources,
  setResourceConstraints,
  requestStarted,
  isNewRequestFlow,
  clusterId,
  width = '133px',
}: NodeSshLoginMenuProps) => {
  const choices: MenuChoice[] = sortSshLoginDetails(
    agent.sshLoginDetails || []
  ).map(sshLogin => ({
    id: sshLogin.login,
    label: sshLogin.login,
    requiresRequest: !!sshLogin.requiresRequest,
    connectUrl: cfg.getSshConnectRoute({
      clusterId,
      serverId: agent.id,
      login: sshLogin.login,
    }),
  }));

  const key = getResourceIDString({
    cluster: clusterId,
    kind: agent.kind,
    name: agent.id,
  });
  const isInCart = !!addedResources.node[agent.id];
  const selectedLogins = addedResourceConstraints[key]?.ssh?.logins ?? [];

  const handleToggle = (choices: MenuChoice[]) => {
    const next = toggleSelections(selectedLogins, choices);

    const rc = (
      next.length ? { ssh: { logins: next } } : undefined
    ) satisfies ResourceConstraints;

    if (isInCart !== !!next.length) {
      addOrRemoveResources(requestItems('node', agent.id, agent.hostname));
    }
    setResourceConstraints(key, rc);
  };

  return (
    <ConstraintMenu
      choices={choices}
      selectedIds={selectedLogins}
      onToggle={handleToggle}
      requestStarted={requestStarted}
      isNewRequestFlow={isNewRequestFlow}
      isInCart={isInCart}
      width={width}
      noPrincipalsText="No logins found"
      searchPlaceholder="Search logins..."
    />
  );
};

// Sorts AWS roles by account ID, then by role name within each account
const sortAwsRoles = (roles: AwsRole[]): AwsRole[] =>
  roles.toSorted(
    (a, b) =>
      (a.accountId ?? '').localeCompare(b.accountId ?? '') ||
      (a.name ?? '').localeCompare(b.name ?? '')
  );

// Sorts logins alphabetically, with 'root' taking precedence if present
const sortSshLoginDetails = (logins: SshLogin[]): SshLogin[] =>
  logins.toSorted((a, b) => {
    if (b.login === 'root') {
      return 1;
    }
    if (a.login === 'root') {
      return -1;
    }
    return a.login.localeCompare(b.login);
  });

const StyledMenuSearchWrapper = styled.div`
  position: sticky;
  top: 0;
  left: 0;
  right: 0;
  padding-top: ${({ theme }) => theme.space[1]}px;
  padding-bottom: ${({ theme }) => theme.space[2]}px;
  background-color: ${({ theme }) => theme.colors.levels.elevated};
  z-index: 10;
`;

const StyledMenuSearch = styled.input`
  ${({ theme }) => `
  box-sizing: border-box;
  display: block;
  height: 32px;
  width: calc(100% - ${theme.space[3]}px);
  padding: ${theme.space[1]}px ${theme.space[2]}px;
  margin: ${theme.space[1]}px ${theme.space[2]}px;
  border: 1px solid ${theme.colors.buttons.border.active};
  border-radius: ${theme.radii[2]}px;
  color: ${theme.colors.text.main};
  background: transparent;
  outline: none;
  transition: border-color 150ms ease, background 150ms ease;

  &:focus-visible {
    border-color: ${theme.colors.buttons.border.border};
  }

  &:focus-visible,
  &:hover {
    background: ${theme.colors.interactive.tonal.neutral[0]};
  }
`}
`;

const SectionHeader = styled(Text)`
  ${({ theme }) => theme.typography.body3};
  font-weight: 500;
  color: ${({ theme }) => theme.colors.text.muted};
  padding-left: ${({ theme }) => theme.space[3]}px;
  padding-right: ${({ theme }) => theme.space[3]}px;
  pointer-events: none;

  &:first-child {
    padding-top: ${({ theme }) => theme.space[2]}px;
  }

  &[aria-hidden='true'] {
    height: 0;
    min-height: 0;
    padding: 0;
    margin: 0;
    overflow: hidden;
    visibility: hidden;
  }
`;

const StyledButton = styled(Button)`
  transition:
    background 150ms ease,
    border-color 150ms ease,
    color 200ms ease;

  svg {
    transition: color 200ms ease;
  }
`;

const StyledMenuSection = styled(Box)`
  margin: ${({ theme }) => theme.space[1]}px 0;

  &[aria-hidden='true'] {
    height: 0;
    margin: 0;
    overflow: hidden;
    visibility: hidden;
  }
`;

const StyledMenuItem = styled(MenuItem)`
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: flex-start;
  gap: ${({ theme }) => theme.space[2]}px;
  min-height: ${({ theme }) => theme.space[3]}px;
  margin: 0;
  padding: ${({ theme }) => `${theme.space[2]}px ${theme.space[3]}px`};
  user-select: none;
  transition:
    background-color 150ms ease,
    color 150ms ease;

  &:focus-visible,
  &:focus-within,
  &:hover {
    background: ${({ theme }) => theme.colors.spotBackground[0]};
    color: ${({ theme }) => theme.colors.text.main};
  }

  &[aria-disabled='true'] {
    background: transparent;
    color: ${({ theme }) => theme.colors.text.muted};
  }

  &[aria-hidden='true'] {
    height: 0;
    min-height: 0;
    padding: 0;
    margin: 0;
    overflow: hidden;
    visibility: hidden;
  }
`;
