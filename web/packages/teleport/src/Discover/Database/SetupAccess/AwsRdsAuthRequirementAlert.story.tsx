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

import { Meta } from '@storybook/react';

import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

import { AwsRdsAuthRequirementAlert } from './AwsRdsAuthRequirements';

type StoryProps = {
  wantAutoDiscover: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/Discover/Database/SetupAccess',
  component: AwsAuthRequirementAlert,
  argTypes: {
    wantAutoDiscover: {
      control: { type: 'boolean' },
    },
  },
  // default
  args: {
    wantAutoDiscover: false,
  },
};
export default meta;

export function AwsAuthRequirementAlert(props: StoryProps) {
  return (
    <AwsRdsAuthRequirementAlert
      wantAutoDiscover={props.wantAutoDiscover}
      id={DiscoverGuideId.DatabaseAwsRdsPostgres}
      uri="some-db-uri:3000"
    />
  );
}
