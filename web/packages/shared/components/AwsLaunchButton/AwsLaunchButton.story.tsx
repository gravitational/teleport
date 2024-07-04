/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { AwsLaunchButton } from './AwsLaunchButton';

export default {
  title: 'Shared/AwsLaunchButton',
};

export const SameDisplayName = () => {
  return (
    <AwsLaunchButton
      awsRoles={[
        {
          arn: 'foo',
          display: 'foo',
          name: 'foo',
          accountId: '123456789012',
        },
        {
          arn: 'bar',
          display: 'bar',
          name: 'bar',
          accountId: '123456789012',
        },
      ]}
      getLaunchUrl={() => null}
    />
  );
};

export const DifferentDisplayName = () => {
  return (
    <AwsLaunchButton
      awsRoles={[
        {
          arn: 'foo',
          display: 'my foo',
          name: 'foo',
          accountId: '123456789012',
        },
        {
          arn: 'bar',
          display: 'bar',
          name: 'bar',
          accountId: '123456789013',
        },
      ]}
      getLaunchUrl={() => null}
    />
  );
};
