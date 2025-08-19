/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

import { ServiceDeployMethod } from '../common';
import {
  AwsRdsAuthRequirementAlert,
  AwsRdsAuthRequirements,
} from './AwsRdsAuthRequirements';

type StoryProps = {
  wantAutoDiscover: boolean;
  serviceDeployMethod?: ServiceDeployMethod;
  variant: 'standalone' | 'alert';
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/Discover/Database/SetupAccess',
  component: AwsAuthRequirements,
  argTypes: {
    wantAutoDiscover: {
      control: { type: 'boolean' },
    },
    serviceDeployMethod: {
      control: { type: 'select' },
      options: ['auto', 'manual', 'skipped', undefined],
      if: { arg: 'variant', eq: 'standalone' },
    },
    variant: {
      options: ['standalone', 'alert'],
      control: { type: 'radio' },
    },
  },
  // default
  args: {
    wantAutoDiscover: false,
    variant: 'standalone',
  },
};
export default meta;

export function AwsAuthRequirements(props: StoryProps) {
  if (props.variant === 'alert') {
    return (
      <AwsRdsAuthRequirementAlert
        wantAutoDiscover={props.wantAutoDiscover}
        id={DiscoverGuideId.DatabaseAwsRdsPostgres}
        uri="some-db-uri:3000"
      />
    );
  }
  return (
    <AwsRdsAuthRequirements
      wantAutoDiscover={props.wantAutoDiscover}
      id={DiscoverGuideId.DatabaseAwsRdsPostgres}
      uri="some-db-uri:3000"
      serviceDeploy={
        props.serviceDeployMethod && {
          method: props.serviceDeployMethod,
          selectedSecurityGroups: ['sg-1', 'sg-2', 'sg-3'],
          selectedSubnetIds: ['subnet-1', 'subnet-2'],
        }
      }
    />
  );
}
