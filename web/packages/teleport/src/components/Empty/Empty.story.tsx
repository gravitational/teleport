/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
import { MemoryRouter } from 'react-router';

import Empty from '.';
import { EmptyResourceKind } from './type';

type StoryProps = {
  resourceKind: EmptyResourceKind;
  canCreate: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/EmptyResource',
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
        'awsIcApp',
      ],
    },
    canCreate: {
      control: { type: 'boolean' },
    },
  },
  // default
  args: {
    resourceKind: 'app',
    canCreate: true,
  },
};
export default meta;

export function SingleResource(props: StoryProps) {
  return (
    <MemoryRouter>
      <Empty
        canCreate={props.canCreate}
        clusterId="some-cluster-id"
        kind={props.resourceKind}
      />
    </MemoryRouter>
  );
}

export const CustomCanCreate = () => {
  return (
    <MemoryRouter>
      <Empty
        canCreate={true}
        clusterId="some-cluster-id"
        emptyStateInfo={{
          byline: 'custom byline',
          title: 'custom title',
          readOnly: {
            title: 'custom read only title',
            resource: 'custom read only resource',
          },
        }}
      />
    </MemoryRouter>
  );
};

export const CustomCannotCreate = () => {
  return (
    <MemoryRouter>
      <Empty
        canCreate={false}
        clusterId="some-cluster-id"
        emptyStateInfo={{
          byline: 'custom byline',
          title: 'custom title',
          readOnly: {
            title: 'custom read only title',
            resource: 'custom read only resource',
          },
        }}
      />
    </MemoryRouter>
  );
};
