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

import { Meta } from '@storybook/react-vite';

import Box from 'design/Box';

import {
  AppAccessSection,
  DatabaseAccessSection,
  GitHubOrganizationAccessSection,
  KubernetesAccessSection,
  ServerAccessSection,
  WindowsDesktopAccessSection,
} from './Resources';
import {
  AppAccessInputFields,
  DatabaseAccessInputFields,
  defaultRoleVersion,
  GitHubOrganizationAccessInputFields,
  KubernetesAccessInputFields,
  newResourceAccess,
  ResourceAccessKind,
  ServerAccessInputFields,
  WindowsDesktopAccessInputFields,
} from './standardmodel';
import { StatefulSection } from './StatefulSection';
import {
  ResourceAccessValidationResult,
  validateResourceAccess,
} from './validation';

type StoryProps = {
  resourceKind: ResourceAccessKind;
  readOnly: boolean;
  hideSomeInputFields: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/Roles',
  argTypes: {
    resourceKind: {
      control: { type: 'select' },
      options: [
        'node',
        'kube_cluster',
        'app',
        'db',
        'windows_desktop',
        'git_server',
      ],
    },
    readOnly: {
      control: { type: 'boolean' },
    },
    hideSomeInputFields: {
      control: { type: 'boolean' },
    },
  },
  // default
  args: {
    readOnly: false,
    hideSomeInputFields: false,
    resourceKind: 'node',
  },
};
export default meta;

export function ResourceSection(props: StoryProps) {
  const kind = props.resourceKind;
  let component;
  let defaultValue;
  let showInputFields;
  switch (kind) {
    case 'app':
      component = AppAccessSection;
      defaultValue = newResourceAccess('app', defaultRoleVersion);
      if (props.hideSomeInputFields) {
        showInputFields = {
          labels: true,
          azureIdentities: true,
        } as AppAccessInputFields;
      }
      break;
    case 'db':
      component = DatabaseAccessSection;
      defaultValue = newResourceAccess('db', defaultRoleVersion);
      if (props.hideSomeInputFields) {
        showInputFields = {
          labels: true,
          names: true,
        } as DatabaseAccessInputFields;
      }
      break;
    case 'git_server':
      component = GitHubOrganizationAccessSection;
      defaultValue = newResourceAccess('git_server', defaultRoleVersion);
      if (props.hideSomeInputFields) {
        showInputFields = {
          organizations: false,
        } as GitHubOrganizationAccessInputFields;
      }
      break;
    case 'kube_cluster':
      component = KubernetesAccessSection;
      defaultValue = {
        ...newResourceAccess('kube_cluster', defaultRoleVersion),
        resources: [
          {
            id: '12',
            kind: [{ value: 'hello', label: 'hello' }],
            name: 'kube',
            namespace: 'namespace',
            verbs: [{ value: 'read', label: 'read' }],
            roleVersion: defaultRoleVersion,
          },
        ],
      };
      if (props.hideSomeInputFields) {
        showInputFields = {
          labels: true,
          users: true,
        } as KubernetesAccessInputFields;
      }
      break;
    case 'node':
      component = ServerAccessSection;
      defaultValue = newResourceAccess('node', defaultRoleVersion);
      if (props.hideSomeInputFields) {
        showInputFields = {
          labels: true,
        } as ServerAccessInputFields;
      }
      break;
    case 'windows_desktop':
      component = WindowsDesktopAccessSection;
      defaultValue = newResourceAccess('windows_desktop', defaultRoleVersion);
      if (props.hideSomeInputFields) {
        showInputFields = {
          labels: true,
        } as WindowsDesktopAccessInputFields;
      }
      break;
    default:
      kind satisfies never;
  }

  return (
    <Box
      p={3}
      width="550px"
      borderRadius={3}
      border="1px solid"
      borderColor="text.disabled"
    >
      <StatefulSection<any, ResourceAccessValidationResult, any>
        component={component}
        defaultValue={defaultValue}
        onChange={() => null}
        validatorRef={() => null}
        validate={validateResourceAccess}
        showInputFields={showInputFields}
        readOnly={props.readOnly}
        key={crypto.randomUUID()}
      />
    </Box>
  );
}
