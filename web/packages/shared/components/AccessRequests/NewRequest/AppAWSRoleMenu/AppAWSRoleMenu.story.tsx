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

import type { Meta, StoryFn, StoryObj } from '@storybook/react-vite';
import { action } from 'storybook/actions';
import { useArgs } from 'storybook/preview-api';

import { Flex } from 'design';
import {
  getResourceIDString,
  ResourceConstraints,
  ResourceIDString,
} from 'shared/services/accessRequests';
import { AwsRole } from 'shared/services/apps';

import { AppAWSRoleMenu } from './AppAWSRoleMenu';

const clusterName = 'test-cluster';
const appName = 'aws-console';
const resourceKey = getResourceIDString({
  cluster: clusterName,
  kind: 'app',
  name: appName,
});

function makeRole(
  name: string,
  accountId: string,
  opts?: { requiresRequest?: boolean }
): AwsRole & { launchUrl: string } {
  return {
    name,
    display: name,
    arn: `arn:aws:iam::${accountId}:role/${name}`,
    accountId,
    requiresRequest: opts?.requiresRequest ?? false,
    launchUrl: `https://console.aws.amazon.com/federation?arn=arn:aws:iam::${accountId}:role/${name}`,
  };
}

const grantedRoles = [
  makeRole('Admin', '123456789012'),
  makeRole('Developer', '123456789012'),
  makeRole('ReadOnly', '123456789012'),
];

const requestableRoles = [
  makeRole('StagingAdmin', '123456789012', { requiresRequest: true }),
  makeRole('ProdAdmin', '123456789012', { requiresRequest: true }),
  makeRole('SecurityAudit', '123456789012', { requiresRequest: true }),
];

export default {
  title: 'Shared/AccessRequests/AppAWSRoleMenu',
  component: AppAWSRoleMenu,
  args: {
    clusterName,
    appName,
    isAppInCart: false,
    addedResourceConstraints: {},
    requestStarted: false,
    isNewRequestFlow: false,
    width: '123px',
    addOrRemoveApp: action('addOrRemoveApp'),
    setResourceConstraints: action('setResourceConstraints'),
  },
  render: (args => {
    const [{ isAppInCart, addedResourceConstraints }, updateArgs] =
      useArgs<Meta<typeof AppAWSRoleMenu>['args']>();

    const handleSetConstraints = (
      key: ResourceIDString,
      rc?: ResourceConstraints
    ) => {
      action('setResourceConstraints')(key, rc);
      const next = { ...addedResourceConstraints };
      if (rc) {
        next[key] = rc;
      } else {
        delete next[key];
      }
      updateArgs({ addedResourceConstraints: next });
    };

    const handleAddOrRemoveApp = () => {
      action('addOrRemoveApp')();
      updateArgs({ isAppInCart: !isAppInCart });
    };

    return (
      <Flex alignItems="center" minHeight="100px">
        <AppAWSRoleMenu
          {...args}
          isAppInCart={isAppInCart}
          addedResourceConstraints={addedResourceConstraints}
          setResourceConstraints={handleSetConstraints}
          addOrRemoveApp={handleAddOrRemoveApp}
        />
      </Flex>
    );
  }) satisfies StoryFn<typeof AppAWSRoleMenu>,
} satisfies Meta<typeof AppAWSRoleMenu>;

type Story = StoryObj<typeof AppAWSRoleMenu>;

/**
 * Both granted and requestable roles are available.
 * The dropdown shows a "Connect:" section with direct launch links
 * and a "Request Access:" section with checkboxes.
 */
export const GrantedAndRequestable: Story = {
  args: {
    awsRoles: [...grantedRoles, ...requestableRoles],
  },
};

/**
 * Only granted roles (no requestable). The dropdown shows
 * launch links for each role.
 */
export const OnlyGrantedRoles: Story = {
  args: {
    awsRoles: grantedRoles,
  },
};

/**
 * Only requestable roles (no granted). The button shows "Request Access"
 * and the dropdown shows only checkboxes.
 */
export const OnlyRequestableRoles: Story = {
  args: {
    awsRoles: requestableRoles,
  },
};

/**
 * A single granted role and no requestable roles.
 * Renders as a simple "Connect" link button (no dropdown).
 */
export const SingleGrantedRole: Story = {
  args: {
    awsRoles: [grantedRoles[0]],
  },
};

/**
 * No roles at all. Shows a disabled "Connect" button
 * with a "No available logins" tooltip.
 */
export const NoAvailableRoles: Story = {
  args: {
    awsRoles: [],
  },
};

/**
 * A request has been started (other resources already in the cart).
 * The "Connect:" section is hidden, showing only requestable checkboxes.
 * The button text says "Add to request".
 */
export const RequestStarted: Story = {
  args: {
    awsRoles: [...grantedRoles, ...requestableRoles],
    requestStarted: true,
  },
};

/**
 * The app is already in the access request cart with some roles selected.
 * The button appears in its "filled/primary" style.
 */
export const AppInCart: Story = {
  args: {
    awsRoles: [...grantedRoles, ...requestableRoles],
    requestStarted: true,
    isAppInCart: true,
    addedResourceConstraints: {
      [resourceKey]: {
        aws_console: {
          role_arns: [requestableRoles[0].arn],
        },
      },
    },
  },
};

/**
 * New request flow: all roles are treated as requestable
 * regardless of their `requiresRequest` flag.
 * The button text says "Add to request".
 */
export const NewRequestFlow: Story = {
  args: {
    awsRoles: grantedRoles,
    isNewRequestFlow: true,
  },
};
