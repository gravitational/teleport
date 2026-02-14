/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useRef, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonBorder, Menu, Text } from 'design';
import { CheckboxInput } from 'design/Checkbox';
import { ChevronDown } from 'design/Icon';
import { MenuItem } from 'design/Menu';
import { HoverTooltip } from 'design/Tooltip';
import {
  getResourceIDString,
  ResourceConstraints,
  ResourceConstraintsMap,
  ResourceIDString,
} from 'shared/services/accessRequests';
import { AwsRole } from 'shared/services/apps';
import { ComponentFeatureID } from 'shared/utils/componentFeatures';

export type AwsLoginChoice = {
  /** The ARN of the AWS role */
  id: string;
  /** Display label for the role */
  label: string;
  /** Whether an access request is needed to assume this role */
  requiresRequest: boolean;
  /** URL to launch the AWS console with this role */
  launchUrl?: string;
};

/**
 * AppResource represents a minimal interface that satisfies
 * an AWS Console application resource with support for Resource Constraints.
 */
type AppResource = {
  awsConsole: boolean;
  supportedFeatureIds?: number[];
  awsRoles: AwsRole[];
};

/**
 * appIsAwsConsoleAndSupportsConstraints returns whether the given
 * app is an AWS Console app that supports specifying Role ARN constraints.
 */
export const appIsAwsConsoleAndSupportsConstraints = (
  resource: AppResource
): boolean => {
  if (!resource.awsConsole) {
    return false;
  }

  if (
    !resource.supportedFeatureIds?.includes(
      ComponentFeatureID.ResourceConstraintsV1
    )
  ) {
    return false;
  }

  return !!resource.awsRoles.length;
};

type AwsRoleWithLaunchUrl = AwsRole & {
  launchUrl: string;
};

type AppAWSRoleMenuProps = {
  /**
   * AWS roles available on the application, mapped with additional
   * `launchUrl` property.
   */
  awsRoles: AwsRoleWithLaunchUrl[];
  /** Whether the app is currently in the access request cart */
  isAppInCart: boolean;
  /** Resource constraints that have been added to the access request cart */
  addedResourceConstraints: ResourceConstraintsMap;
  /** Name of the cluster where the app is located */
  clusterName: string;
  /** Name of the app */
  appName: string;
  /** Width of the button (default: '123px') */
  width?: string;
  /** Whether an access request has already been started. */
  requestStarted?: boolean;
  /**
   * Whether we're in the "new request" flow where all roles should be treated
   * as requestable (no granted roles)
   */
  isNewRequestFlow?: boolean;
  /** Callback to add or remove the app from the access request cart */
  addOrRemoveApp: (action?: 'add' | 'remove') => void;
  /** Callback to update resource constraints for the app */
  setResourceConstraints: (
    key: ResourceIDString,
    rc?: ResourceConstraints
  ) => void;
};

/**
 * Converts an AwsRole to an AWSLoginChoice for display purposes
 */
export const awsRoleToLoginChoice = (
  role: AwsRoleWithLaunchUrl
): AwsLoginChoice => ({
  id: role.arn,
  label: `${role.accountId}: ${role.display}${role.display !== role.name ? ` (${role.name})` : ''}`,
  requiresRequest: role.requiresRequest ?? false,
  launchUrl: role.launchUrl,
});

/**
 * AppAWSRoleMenu allows selecting/requesting AWS IAM Roles for an AWS Console app.
 *
 * This component renders a dropdown that:
 * - Shows granted roles that can be used to connect directly (if getLaunchUrl is provided)
 * - Shows requestable roles that can be added to an access request
 * - Manages the selected role ARNs as resource constraints
 */
export const AppAWSRoleMenu = ({
  awsRoles,
  isAppInCart,
  addedResourceConstraints,
  clusterName,
  appName,
  addOrRemoveApp,
  setResourceConstraints,
  requestStarted = false,
  isNewRequestFlow = false,
  width = '123px',
}: AppAWSRoleMenuProps) => {
  const anchorEl = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);

  const { granted, requestable } = (awsRoles || [])
    .map(awsRoleToLoginChoice)
    .reduce<{ granted: AwsLoginChoice[]; requestable: AwsLoginChoice[] }>(
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
      { granted: [], requestable: [] }
    );

  const requestStartedOrNoGranted = requestStarted || !granted.length;

  // Resource ID string for constraints map key
  const key = getResourceIDString({
    cluster: clusterName,
    kind: 'app',
    name: appName,
  });
  const selectedARNs =
    addedResourceConstraints?.[key]?.aws_console?.role_arns ?? [];

  const isChecked = (choice: AwsLoginChoice) =>
    selectedARNs.includes(choice.id);

  const toggleRequestable = (choice: AwsLoginChoice) => {
    const next = isChecked(choice)
      ? selectedARNs.filter(arn => arn !== choice.id)
      : [...selectedARNs, choice.id];
    const rc = (
      next.length
        ? {
            aws_console: { role_arns: next },
          }
        : undefined
    ) satisfies ResourceConstraints | undefined;

    // Add/remove agent from cart if needed
    if (isAppInCart !== !!next.length) {
      addOrRemoveApp();
    }
    setResourceConstraints(key, rc);
  };

  // If <= one granted login is available and none requestable, show normal button.
  if (granted.length <= 1 && !requestable.length) {
    // If we have a granted role, show Connect button
    if (granted.length === 1) {
      return (
        <ButtonBorder
          as="a"
          textTransform="none"
          width={width}
          size="small"
          href={granted[0].launchUrl}
          target="_blank"
          rel="noreferrer"
        >
          Connect
        </ButtonBorder>
      );
    }
    // No available logins
    return (
      <HoverTooltip tipContent="No available logins">
        <ButtonBorder
          textTransform="none"
          width={width}
          size="small"
          disabled={true}
        >
          Connect
        </ButtonBorder>
      </HoverTooltip>
    );
  }

  return (
    <>
      <ButtonBorder
        textTransform="none"
        width={width}
        size="small"
        fill={isAppInCart ? 'filled' : undefined}
        intent={isAppInCart ? 'primary' : undefined}
        css={`
          text-wrap: nowrap;
        `}
        ref={el => (anchorEl.current = el!)}
        onClick={() => {
          setOpen(true);
        }}
      >
        {isNewRequestFlow || requestStarted
          ? 'Add to request'
          : requestStartedOrNoGranted
            ? 'Request Access'
            : 'Connect'}
        <ChevronDown
          ml={1}
          mr={-1}
          size="small"
          color={isAppInCart ? 'test.primaryInverse' : 'text.slightlyMuted'}
        />
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
                  as="label"
                  key={`r:${item.id}`}
                  title={item.label}
                  htmlFor={`r:${item.id}`}
                >
                  <CheckboxInput
                    type="checkbox"
                    id={`r:${item.id}`}
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
